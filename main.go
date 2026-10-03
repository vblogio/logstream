// Logstream: a syslog (UDP/TCP), Docker and host system logs sink stored in VictoriaLogs,
// with a web interface to browse and search the logs.
package main

import (
	"context"
	"crypto/subtle"
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
	authUser   string
	authPass   string
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
		authUser:   os.Getenv("AUTH_USER"),
		authPass:   os.Getenv("AUTH_PASS"),
		rdns:       getenvBool("RDNS", true),
		dnsServer:  os.Getenv("DNS_SERVER"),
		allowPurge: getenvBool("ALLOW_PURGE", true),
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
	// Listening errors (port already used…) are shown in Settings > Sources.
	syslogSrv := NewSyslogServer(ctx, cfg.syslogAddr, getenv("SYSLOG_PUBLIC_PORT", ""), cfg.dataDir, sink)

	static, err := fs.Sub(webFS, "web")
	if err != nil {
		log.Fatal(err)
	}
	mux := http.NewServeMux()
	api := &API{store: store, hub: hub, tags: tags, rdns: rdns, allowPurge: cfg.allowPurge, exportMax: cfg.exportMax, syslog: syslogSrv}
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

	srv := &http.Server{
		Addr:              cfg.httpAddr,
		Handler:           basicAuth(cfg.authUser, cfg.authPass, mux),
		ReadHeaderTimeout: 10 * time.Second,
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

// basicAuth protects the UI when AUTH_USER is set (except /healthz).
func basicAuth(user, pass string, next http.Handler) http.Handler {
	if user == "" {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" {
			next.ServeHTTP(w, r)
			return
		}
		u, p, ok := r.BasicAuth()
		if !ok ||
			subtle.ConstantTimeCompare([]byte(u), []byte(user)) != 1 ||
			subtle.ConstantTimeCompare([]byte(p), []byte(pass)) != 1 {
			w.Header().Set("WWW-Authenticate", `Basic realm="logstream"`)
			http.Error(w, "authentication required", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}
