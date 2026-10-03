// Logstream: a syslog (UDP/TCP), Docker and host system logs sink stored in VictoriaLogs,
// with a web interface to browse and search the logs.
package main

import (
	"context"
	"embed"
	"errors"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
	_ "time/tzdata" // embedded time zones: TZ works without a system package
)

//go:embed web
var webFS embed.FS

type config struct {
	syslogAddr string
	httpAddr   string
	vlogsURL   string
	dataDir    string
	auth       authConfig
	rdns       bool
	dnsServer  string
	allowPurge bool
	exportMax  int
	dockerLogs bool
	dockerHost string
	backfill   time.Duration
	hostRoot   string
	hostFill   time.Duration
	batchSize  int
	queueSize  int
	flushEvery time.Duration
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getenvInt(key string, def int) int {
	if v, err := strconv.Atoi(os.Getenv(key)); err == nil && v > 0 {
		return v
	}
	return def
}

func getenvBool(key string, def bool) bool {
	switch strings.ToLower(os.Getenv(key)) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	}
	return def
}

func getenvDuration(key string, def time.Duration) time.Duration {
	if d, err := time.ParseDuration(os.Getenv(key)); err == nil && d >= 0 {
		return d
	}
	return def
}

func main() {
	cfg := config{
		syslogAddr: getenv("SYSLOG_ADDR", ":5514"),
		httpAddr:   getenv("HTTP_ADDR", ":8080"),
		vlogsURL:   getenv("VLOGS_URL", "http://victorialogs:9428"),
		dataDir:    getenv("DATA_DIR", "/data"),
		auth: authConfig{
			mode:         getenv("AUTH_MODE", "local"),
			user:         os.Getenv("AUTH_USER"),
			pass:         os.Getenv("AUTH_PASS"),
			viewerUser:   os.Getenv("AUTH_VIEWER_USER"),
			viewerPass:   os.Getenv("AUTH_VIEWER_PASS"),
			adminGroup:   os.Getenv("OIDC_ADMIN_GROUP"),
			groupsClaim:  os.Getenv("OIDC_GROUPS_CLAIM"),
			issuer:       os.Getenv("OIDC_ISSUER"),
			clientID:     os.Getenv("OIDC_CLIENT_ID"),
			clientSecret: os.Getenv("OIDC_CLIENT_SECRET"),
			redirectURL:  os.Getenv("OIDC_REDIRECT_URL"),
			scopes:       os.Getenv("OIDC_SCOPES"),
			// SESSION_TTL applies to both modes; OIDC_SESSION_TTL is its former name.
			sessionTTL: getenvDuration("SESSION_TTL", getenvDuration("OIDC_SESSION_TTL", 12*time.Hour)),
			loginLogo:  os.Getenv("LOGIN_LOGO"),
		},
		rdns:       getenvBool("RDNS", true),
		dnsServer:  os.Getenv("DNS_SERVER"),
		allowPurge: getenvBool("ALLOW_PURGE", false),
		exportMax:  getenvInt("EXPORT_MAX", 100000),
		dockerLogs: getenvBool("DOCKER_LOGS", false),
		dockerHost: getenv("DOCKER_HOST", "unix:///var/run/docker.sock"),
		backfill:   getenvDuration("DOCKER_BACKFILL", time.Hour),
		hostRoot:   getenv("HOST_LOGS_ROOT", "/host"),
		hostFill:   getenvDuration("HOST_LOGS_BACKFILL", time.Hour),
		batchSize:  getenvInt("BATCH_SIZE", 1000),
		queueSize:  getenvInt("QUEUE_SIZE", 100000),
		flushEvery: time.Duration(getenvInt("FLUSH_MS", 1000)) * time.Millisecond,
	}

	cfg.auth.dataDir = cfg.dataDir

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	tags, err := LoadTagStore(filepath.Join(cfg.dataDir, "tags.json"))
	if err != nil {
		log.Fatalf("tags: %v", err)
	}

	store := NewStore(cfg.vlogsURL, cfg.batchSize, cfg.queueSize, cfg.flushEvery)
	storeDone := make(chan struct{})
	go func() {
		store.Run(ctx)
		close(storeDone)
	}()

	hub := NewHub()
	rdns := NewReverseDNS(cfg.rdns, cfg.dnsServer)
	sink := func(e *Entry) {
		// Host sent as an IP (or no host in the header): replace it with its DNS name.
		// A new IP waits at most 300 ms; slower lookups finish in the background.
		if name := rdns.Lookup(e.Host, 300*time.Millisecond); name != "" {
			e.HostIP, e.Host = e.Host, name
		}
		store.Enqueue(e)
		hub.Publish(e)
	}
	tcpMaxConns = getenvInt("SYSLOG_TCP_MAX_CONNS", tcpMaxConns)
	if d := getenvDuration("SYSLOG_TCP_IDLE", tcpIdle); d > 0 {
		tcpIdle = d
	}
	// Listening errors (port already used…) are shown in Settings > Sources.
	syslogSrv := NewSyslogServer(ctx, cfg.syslogAddr, getenv("SYSLOG_PUBLIC_PORT", ""), cfg.dataDir, sink)

	static, err := fs.Sub(webFS, "web")
	if err != nil {
		log.Fatal(err)
	}
	mux := http.NewServeMux()
	presets := getenv("PRESETS_FILE", filepath.Join(cfg.dataDir, "presets.json"))
	api := &API{store: store, hub: hub, tags: tags, presets: presets, rdns: rdns, allowPurge: cfg.allowPurge, exportMax: cfg.exportMax, syslog: syslogSrv}
	if cfg.dockerLogs {
		dm, err := NewDockerManager(cfg.dockerHost, cfg.dataDir, cfg.backfill, sink)
		if err != nil {
			log.Printf("docker logs disabled: %v", err)
		} else {
			go dm.Run(ctx)
			api.docker = dm
			log.Printf("docker logs enabled through %s", cfg.dockerHost)
		}
	}
	// System logs of the host (journal or /var/log), off until enabled in Settings > Sources.
	api.host = NewHostLogs(cfg.hostRoot, cfg.dataDir, cfg.hostFill, sink)
	go api.host.Run(ctx)
	api.Routes(mux)
	mux.Handle("GET /", http.FileServer(http.FS(static)))

	handler, err := newAuth(cfg.auth, readOnly(mux))
	if err != nil {
		log.Fatalf("auth: %v", err)
	}
	if o, ok := handler.(*OIDC); ok {
		go o.checkProvider()
	}
	_, local := handler.(*Local)
	_, oidc := handler.(*OIDC)
	authOn := local || oidc
	if !authOn {
		log.Printf("warning: no authentication (AUTH_USER is empty): anyone who can reach %s can read the logs and change the settings", cfg.httpAddr)
	}
	handler = secure(handler, contentSecurityPolicy(static), authOn)
	srv := &http.Server{
		Addr:              cfg.httpAddr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       2 * time.Minute,
		// Requests inherit the global context so SSE streams end on shutdown.
		BaseContext: func(net.Listener) context.Context { return ctx },
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	log.Printf("web UI on %s, syslog on %s (udp+tcp), storage %s", cfg.httpAddr, cfg.syslogAddr, cfg.vlogsURL)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("http: %v", err)
	}
	<-storeDone
	log.Println("shutdown complete")
}
