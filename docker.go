package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// DockerContainer is a container as listed in Settings > Sources.
type DockerContainer struct {
	ID        string `json:"id"`
	Key       string `json:"key"` // stable across re-creations: "project/service", or the name
	Name      string `json:"name"`
	Image     string `json:"image"`
	Project   string `json:"project,omitempty"`
	Service   string `json:"service,omitempty"`
	State     string `json:"state"` // running, exited, …
	Enabled   bool   `json:"enabled"`
	Locked    bool   `json:"locked"` // Logstream itself, or label logstream.exclude=true
	Following bool   `json:"following"`
}

// dockerConfig is saved in /data/docker.json and edited from the UI.
type dockerConfig struct {
	DefaultEnabled bool            `json:"defaultEnabled"`
	Containers     map[string]bool `json:"containers"` // key -> enabled, overrides the default
}

type follower struct{ cancel context.CancelFunc }

// DockerManager follows the logs of the containers of the local Docker engine
// (through the Docker API) and sends each line to the sink as an Entry.
type DockerManager struct {
	client    *http.Client
	base      string
	sink      func(*Entry)
	backfill  time.Duration
	selfID    string
	cfgPath   string
	statePath string
	syncCh    chan struct{}

	mu         sync.Mutex
	cfg        dockerConfig
	hostName   string
	containers []DockerContainer
	followers  map[string]*follower // container ID -> running follower
	checkpoint map[string]time.Time // container ID -> timestamp of the last line stored
	dirty      bool
	connected  bool
	lastErr    string
}

func NewDockerManager(host, dataDir string, backfill time.Duration, sink func(*Entry)) (*DockerManager, error) {
	client, base, err := dockerHTTP(host)
	if err != nil {
		return nil, err
	}
	self, _ := os.Hostname() // in a container: the short container ID
	m := &DockerManager{
		client:     client,
		base:       base,
		sink:       sink,
		backfill:   backfill,
		selfID:     self,
		cfgPath:    filepath.Join(dataDir, "docker.json"),
		statePath:  filepath.Join(dataDir, "docker-state.json"),
		syncCh:     make(chan struct{}, 1),
		cfg:        dockerConfig{DefaultEnabled: true, Containers: map[string]bool{}},
		followers:  map[string]*follower{},
		checkpoint: map[string]time.Time{},
	}
	m.load()
	return m, nil
}

// dockerHTTP returns an HTTP client for DOCKER_HOST: unix:///path, tcp://host:port or http(s)://…
func dockerHTTP(host string) (*http.Client, string, error) {
	switch {
	case strings.HasPrefix(host, "unix://"):
		path := strings.TrimPrefix(host, "unix://")
		tr := &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, "unix", path)
		}}
		return &http.Client{Transport: tr}, "http://docker", nil
	case strings.HasPrefix(host, "tcp://"):
		return &http.Client{}, "http://" + strings.TrimPrefix(host, "tcp://"), nil
	case strings.HasPrefix(host, "http://"), strings.HasPrefix(host, "https://"):
		return &http.Client{}, strings.TrimRight(host, "/"), nil
	}
	return nil, "", fmt.Errorf("unsupported DOCKER_HOST %q", host)
}

func (m *DockerManager) load() {
	if b, err := os.ReadFile(m.cfgPath); err == nil {
		var cfg dockerConfig
		if err := json.Unmarshal(b, &cfg); err == nil {
			if cfg.Containers == nil {
				cfg.Containers = map[string]bool{}
			}
			m.cfg = cfg
		}
	}
	if b, err := os.ReadFile(m.statePath); err == nil {
		var st map[string]string
		if err := json.Unmarshal(b, &st); err == nil {
			for id, s := range st {
				if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
					m.checkpoint[id] = t
				}
			}
		}
	}
}

func writeJSONFile(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// Run keeps the followed containers in line with the Docker engine and the
// configuration until ctx is cancelled.
func (m *DockerManager) Run(ctx context.Context) {
	go m.watchEvents(ctx)
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	save := time.NewTicker(5 * time.Second)
	defer save.Stop()
	m.sync(ctx)
	for {
		select {
		case <-ctx.Done():
			m.saveState()
			return
		case <-ticker.C:
			m.sync(ctx)
		case <-m.syncCh:
			m.sync(ctx)
		case <-save.C:
			m.saveState()
		}
	}
}

// watchEvents listens to container events (start, stop, removal…) so that a
// new container is followed right away instead of at the next periodic sync.
func (m *DockerManager) watchEvents(ctx context.Context) {
	filters := url.QueryEscape(`{"type":["container"]}`)
	for {
		resp, err := m.get(ctx, "/events?filters="+filters)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			m.setError(err)
			sleepCtx(ctx, 5*time.Second)
			continue
		}
		dec := json.NewDecoder(resp.Body)
		for {
			var ev struct{ Action string }
			if err := dec.Decode(&ev); err != nil {
				break
			}
			switch ev.Action {
			case "create", "start", "die", "destroy", "rename":
				m.requestSync()
			}
		}
		resp.Body.Close()
		if ctx.Err() != nil {
			return
		}
		sleepCtx(ctx, time.Second) // stream cut (proxy timeout, Docker restart): reconnect
	}
}

func (m *DockerManager) requestSync() {
	select {
	case m.syncCh <- struct{}{}:
	default:
	}
}

func (m *DockerManager) get(ctx context.Context, path string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, m.base+path, nil)
	if err != nil {
		return nil, err
	}
	resp, err := m.client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		resp.Body.Close()
		return nil, fmt.Errorf("docker %s: HTTP %d %s", strings.SplitN(path, "?", 2)[0], resp.StatusCode, strings.TrimSpace(string(msg)))
	}
	return resp, nil
}

func (m *DockerManager) getJSON(ctx context.Context, path string, v any) error {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	resp, err := m.get(ctx, path)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return json.NewDecoder(resp.Body).Decode(v)
}

func (m *DockerManager) setError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.lastErr != err.Error() {
		log.Printf("docker: %v", err)
	}
	m.connected = false
	m.lastErr = err.Error()
}

// enabled tells whether a container's logs must be collected; the caller holds the lock.
func (m *DockerManager) enabled(c DockerContainer) bool {
	if c.Locked {
		return false
	}
	if v, ok := m.cfg.Containers[c.Key]; ok {
		return v
	}
	return m.cfg.DefaultEnabled
}

// sync lists the containers and starts or stops their followers.
func (m *DockerManager) sync(ctx context.Context) {
	m.mu.Lock()
	needInfo := m.hostName == ""
	m.mu.Unlock()
	if needInfo {
		var info struct{ Name string }
		if err := m.getJSON(ctx, "/info", &info); err == nil && info.Name != "" {
			m.mu.Lock()
			m.hostName = info.Name
			m.mu.Unlock()
		}
	}

	var list []struct {
		ID     string `json:"Id"`
		Names  []string
		Image  string
		State  string
		Labels map[string]string
	}
	if err := m.getJSON(ctx, "/containers/json?all=1", &list); err != nil {
		if ctx.Err() == nil {
			m.setError(err)
		}
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.connected {
		log.Printf("docker: connected, %d containers", len(list))
	}
	m.connected, m.lastErr = true, ""
	seen := map[string]bool{}
	cs := make([]DockerContainer, 0, len(list))
	for _, c := range list {
		dc := DockerContainer{ID: c.ID, Image: c.Image, State: c.State,
			Project: c.Labels["com.docker.compose.project"], Service: c.Labels["com.docker.compose.service"]}
		if len(c.Names) > 0 {
			dc.Name = strings.TrimPrefix(c.Names[0], "/")
		}
		dc.Key = dc.Name
		if dc.Project != "" && dc.Service != "" {
			dc.Key = dc.Project + "/" + dc.Service
		}
		dc.Locked = c.Labels["logstream.exclude"] == "true" || (m.selfID != "" && strings.HasPrefix(c.ID, m.selfID))
		dc.Enabled = m.enabled(dc)
		seen[c.ID] = true

		want := dc.Enabled && c.State == "running"
		f, following := m.followers[c.ID]
		switch {
		case want && !following:
			m.startFollower(ctx, dc)
		case !want && following:
			f.cancel()
			delete(m.followers, c.ID)
		}
		cs = append(cs, dc)
	}
	for id, f := range m.followers {
		if !seen[id] {
			f.cancel()
			delete(m.followers, id)
		}
	}
	for id := range m.checkpoint {
		if !seen[id] {
			delete(m.checkpoint, id)
			m.dirty = true
		}
	}
	sort.Slice(cs, func(i, j int) bool {
		if cs[i].Project != cs[j].Project {
			return cs[i].Project < cs[j].Project
		}
		return cs[i].Name < cs[j].Name
	})
	m.containers = cs
}

// startFollower launches the log reader of a container; the caller holds the lock.
func (m *DockerManager) startFollower(parent context.Context, c DockerContainer) {
	ctx, cancel := context.WithCancel(parent)
	f := &follower{cancel: cancel}
	m.followers[c.ID] = f
	go func() {
		defer func() {
			cancel()
			m.mu.Lock()
			if m.followers[c.ID] == f {
				delete(m.followers, c.ID)
			}
			m.mu.Unlock()
		}()
		m.follow(ctx, c)
	}()
}

// follow reads the logs of a container while it runs, reconnecting when the
// stream is cut (proxy timeout, Docker restart) without losing or repeating lines.
func (m *DockerManager) follow(ctx context.Context, c DockerContainer) {
	for {
		var inspect struct {
			Config struct{ Tty bool }
			State  struct{ Running bool }
		}
		if err := m.getJSON(ctx, "/containers/"+c.ID+"/json", &inspect); err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Printf("docker %s: %v", c.Name, err)
			sleepCtx(ctx, 5*time.Second)
			continue
		}
		if !inspect.State.Running {
			return
		}
		err := m.stream(ctx, c, inspect.Config.Tty, m.since(c.ID))
		if ctx.Err() != nil {
			return
		}
		if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
			log.Printf("docker %s: %v", c.Name, err)
		}
		sleepCtx(ctx, time.Second)
	}
}

// sleepCtx waits for d, or returns false if the context ends first.
func sleepCtx(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}

// since returns where to resume reading: just after the last line stored, or
// DOCKER_BACKFILL ago for a container seen for the first time.
func (m *DockerManager) since(id string) time.Time {
	m.mu.Lock()
	defer m.mu.Unlock()
	if t, ok := m.checkpoint[id]; ok {
		return t.Add(time.Nanosecond)
	}
	return time.Now().Add(-m.backfill)
}

func (m *DockerManager) setCheckpoint(id string, t time.Time) {
	m.mu.Lock()
	if t.After(m.checkpoint[id]) {
		m.checkpoint[id] = t
		m.dirty = true
	}
	m.mu.Unlock()
}

func (m *DockerManager) saveState() {
	m.mu.Lock()
	if !m.dirty {
		m.mu.Unlock()
		return
	}
	st := make(map[string]string, len(m.checkpoint))
	for id, t := range m.checkpoint {
		st[id] = t.UTC().Format(time.RFC3339Nano)
	}
	m.dirty = false
	m.mu.Unlock()
	if err := writeJSONFile(m.statePath, st); err != nil {
		log.Printf("docker: saving positions: %v", err)
	}
}

func (m *DockerManager) stream(ctx context.Context, c DockerContainer, tty bool, since time.Time) error {
	q := url.Values{
		"follow":     {"1"},
		"stdout":     {"1"},
		"stderr":     {"1"},
		"timestamps": {"1"},
		"since":      {fmt.Sprintf("%d.%09d", since.Unix(), since.Nanosecond())},
	}
	resp, err := m.get(ctx, "/containers/"+c.ID+"/logs?"+q.Encode())
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	emit := func(stream string, line []byte) { m.emit(c, stream, line) }
	if tty {
		return readLines(resp.Body, "stdout", emit)
	}
	return readMultiplexed(resp.Body, emit)
}

// readLines reads a TTY container stream: plain lines.
func readLines(r io.Reader, stream string, emit func(string, []byte)) error {
	br := bufio.NewReaderSize(r, 64*1024)
	for {
		line, err := br.ReadSlice('\n')
		if len(line) > 0 {
			emit(stream, line)
		}
		if err == bufio.ErrBufferFull {
			continue
		}
		if err != nil {
			return err
		}
	}
}

// readMultiplexed reads a non-TTY container stream: frames with an 8-byte
// header (stream type, 3 zero bytes, big-endian payload size).
func readMultiplexed(r io.Reader, emit func(string, []byte)) error {
	br := bufio.NewReaderSize(r, 64*1024)
	hdr := make([]byte, 8)
	names := [2]string{"stdout", "stderr"}
	var partial [2][]byte
	for {
		if _, err := io.ReadFull(br, hdr); err != nil {
			return err
		}
		size := int(binary.BigEndian.Uint32(hdr[4:8]))
		idx := 0
		if hdr[0] == 2 {
			idx = 1
		}
		if size > maxFrame {
			if _, err := io.CopyN(io.Discard, br, int64(size)); err != nil {
				return err
			}
			continue
		}
		buf := make([]byte, size)
		if _, err := io.ReadFull(br, buf); err != nil {
			return err
		}
		data := append(partial[idx], buf...)
		for {
			i := bytes.IndexByte(data, '\n')
			if i < 0 {
				break
			}
			emit(names[idx], data[:i])
			data = data[i+1:]
		}
		if len(data) > maxFrame {
			emit(names[idx], data)
			data = nil
		}
		partial[idx] = append([]byte(nil), data...)
	}
}

// Terminal color and cursor codes, frequent in container output.
var ansiRe = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]`)

func (m *DockerManager) emit(c DockerContainer, stream string, line []byte) {
	now := time.Now()
	ts := now
	s := strings.TrimRight(string(line), "\r\n")
	// With timestamps=1, Docker prefixes each line with its RFC 3339 timestamp.
	if sp := strings.IndexByte(s, ' '); sp > 0 {
		if t, err := time.Parse(time.RFC3339Nano, s[:sp]); err == nil {
			ts, s = t, s[sp+1:]
		}
	} else if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		ts, s = t, ""
	}
	s = ansiRe.ReplaceAllString(s, "")
	if !utf8.ValidString(s) {
		s = strings.ToValidUTF8(s, string(utf8.RuneError))
	}
	if strings.TrimSpace(s) == "" {
		return
	}
	sev := 6 // info
	if n, ok := detectLevel(s); ok {
		sev = n
	}
	app := c.Service
	if app == "" {
		app = c.Name
	}
	m.mu.Lock()
	host := m.hostName
	m.mu.Unlock()
	if host == "" {
		host = "docker"
	}
	short := c.ID
	if len(short) > 12 {
		short = short[:12]
	}
	m.sink(&Entry{
		Time:       ts,
		Received:   now,
		Host:       host,
		App:        app,
		SevNum:     sev,
		Severity:   severityNames[sev],
		Message:    s,
		Proto:      "docker",
		SourceType: "docker",
		Extra: map[string]string{
			"container":       c.Name,
			"container_id":    short,
			"image":           c.Image,
			"compose_project": c.Project,
			"compose_service": c.Service,
			"stream":          stream,
		},
		// Docker keeps the logs: wait for room in the queue rather than drop the
		// line, and resume after it only once it is stored.
		Wait: true,
		Done: func() { m.setCheckpoint(c.ID, ts) },
	})
}

// Log level written by the application: JSON ("level":"error"), logfmt
// (level=warn), [ERROR], a lower-case level between tabs (VictoriaMetrics),
// or an upper-case level word near the start (ERROR, zerolog's INF/WRN…).
var (
	levelJSON    = regexp.MustCompile(`(?i)"(?:level|lvl|severity|log\.level)"\s*:\s*"([a-z]+)"`)
	levelLogfmt  = regexp.MustCompile(`(?i)\b(?:level|lvl|severity)="?([a-z]+)`)
	levelBracket = regexp.MustCompile(`(?i)\[\s*(emerg|alert|crit|critical|fatal|panic|err|error|warn|warning|notice|info|debug|trace)\s*\]`)
	levelTab     = regexp.MustCompile(`(?:^|\t)(debug|info|warn|warning|error|fatal|panic)\t`)
	levelWord    = regexp.MustCompile(`\b(EMERG|ALERT|CRIT|CRITICAL|FATAL|FTL|PANIC|PNC|ERR|ERROR|WARN|WARNING|WRN|NOTICE|INFO|INF|DEBUG|DBG|TRACE|TRC)\b`)
)

func levelNum(word string) (int, bool) {
	switch strings.ToLower(word) {
	case "emerg", "emergency":
		return 0, true
	case "alert":
		return 1, true
	case "crit", "critical", "fatal", "ftl", "panic", "pnc":
		return 2, true
	case "err", "error":
		return 3, true
	case "warn", "warning", "wrn":
		return 4, true
	case "notice":
		return 5, true
	case "info", "information", "inf":
		return 6, true
	case "debug", "dbg", "trace", "trc":
		return 7, true
	}
	return 0, false
}

func detectLevel(s string) (int, bool) {
	for _, re := range []*regexp.Regexp{levelJSON, levelLogfmt, levelBracket} {
		if m := re.FindStringSubmatch(s); m != nil {
			if n, ok := levelNum(m[1]); ok {
				return n, true
			}
		}
	}
	head := s
	if len(head) > 60 {
		head = head[:60]
	}
	if m := levelTab.FindStringSubmatch(head); m != nil {
		return levelNum(m[1])
	}
	if m := levelWord.FindStringSubmatch(head); m != nil {
		return levelNum(m[1])
	}
	return 0, false
}

// Status is the state shown in Settings > Sources.
func (m *DockerManager) Status() map[string]any {
	m.mu.Lock()
	defer m.mu.Unlock()
	cs := make([]DockerContainer, len(m.containers))
	copy(cs, m.containers)
	for i := range cs {
		cs[i].Enabled = m.enabled(cs[i])
		_, cs[i].Following = m.followers[cs[i].ID]
	}
	return map[string]any{
		"enabled":        true,
		"connected":      m.connected,
		"error":          m.lastErr,
		"host":           m.hostName,
		"defaultEnabled": m.cfg.DefaultEnabled,
		"containers":     cs,
	}
}

// Configure changes the default and per-container choices, then applies them.
func (m *DockerManager) Configure(defaultEnabled *bool, containers map[string]bool) error {
	m.mu.Lock()
	if defaultEnabled != nil {
		m.cfg.DefaultEnabled = *defaultEnabled
	}
	for k, v := range containers {
		m.cfg.Containers[k] = v
	}
	cfg := dockerConfig{DefaultEnabled: m.cfg.DefaultEnabled, Containers: map[string]bool{}}
	for k, v := range m.cfg.Containers {
		cfg.Containers[k] = v
	}
	m.mu.Unlock()
	m.requestSync()
	return writeJSONFile(m.cfgPath, cfg)
}
