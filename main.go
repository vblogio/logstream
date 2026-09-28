// Logstream : puits de logs syslog (UDP/TCP) stocké dans VictoriaLogs,
// avec une interface web de consultation.
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
	"syscall"
	"time"
	_ "time/tzdata" // fuseaux horaires embarqués : la variable TZ fonctionne sans paquet système
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

func main() {
	cfg := config{
		syslogAddr: getenv("SYSLOG_ADDR", ":5514"),
		httpAddr:   getenv("HTTP_ADDR", ":8080"),
		vlogsURL:   getenv("VLOGS_URL", "http://victorialogs:9428"),
		dataDir:    getenv("DATA_DIR", "/data"),
		authUser:   os.Getenv("AUTH_USER"),
		authPass:   os.Getenv("AUTH_PASS"),
		batchSize:  getenvInt("BATCH_SIZE", 1000),
		queueSize:  getenvInt("QUEUE_SIZE", 100000),
		flushEvery: time.Duration(getenvInt("FLUSH_MS", 1000)) * time.Millisecond,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	tags, err := LoadTagStore(filepath.Join(cfg.dataDir, "tags.json"))
	if err != nil {
		log.Fatalf("tags : %v", err)
	}

	store := NewStore(cfg.vlogsURL, cfg.batchSize, cfg.queueSize, cfg.flushEvery)
	storeDone := make(chan struct{})
	go func() {
		store.Run(ctx)
		close(storeDone)
	}()

	hub := NewHub()
	sink := func(e *Entry) {
		store.Enqueue(e)
		hub.Publish(e)
	}
	if err := StartSyslog(ctx, cfg.syslogAddr, sink); err != nil {
		log.Fatalf("syslog : %v", err)
	}

	static, err := fs.Sub(webFS, "web")
	if err != nil {
		log.Fatal(err)
	}
	mux := http.NewServeMux()
	api := &API{store: store, hub: hub, tags: tags}
	api.Routes(mux)
	mux.Handle("GET /", http.FileServer(http.FS(static)))

	srv := &http.Server{
		Addr:              cfg.httpAddr,
		Handler:           basicAuth(cfg.authUser, cfg.authPass, mux),
		ReadHeaderTimeout: 10 * time.Second,
		// Les requêtes héritent du contexte global : les flux SSE se ferment à l'arrêt.
		BaseContext: func(net.Listener) context.Context { return ctx },
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	log.Printf("interface web sur %s, syslog sur %s (udp+tcp), stockage %s", cfg.httpAddr, cfg.syslogAddr, cfg.vlogsURL)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("http : %v", err)
	}
	<-storeDone
	log.Println("arrêt terminé")
}

// basicAuth protège l'interface si AUTH_USER est défini (sauf /healthz).
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
			http.Error(w, "authentification requise", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}
