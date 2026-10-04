package main

import (
	"context"
	"encoding/json"
	"log"
	"net"
	"os"
	"path/filepath"
	"sync"
)

// syslogConfig is saved in /data/syslog.json and edited in Settings > Sources.
type syslogConfig struct {
	Enabled bool `json:"enabled"`
	UDP     bool `json:"udp"`
	TCP     bool `json:"tcp"`
}

// SyslogServer owns the UDP and TCP listeners and opens or closes them when
// the configuration changes, without restarting Logstream.
type SyslogServer struct {
	parent     context.Context
	addr       string // listening address inside the container, e.g. ":5514"
	publicPort string // port published on the host by docker-compose (display only)
	sink       func(*Entry)
	cfgPath    string

	mu     sync.Mutex
	cfg    syslogConfig
	cancel context.CancelFunc
	pc     net.PacketConn
	ln     net.Listener
	errUDP string
	errTCP string
}

func NewSyslogServer(ctx context.Context, addr, publicPort, dataDir string, sink func(*Entry)) *SyslogServer {
	s := &SyslogServer{
		parent:     ctx,
		addr:       addr,
		publicPort: publicPort,
		sink:       sink,
		cfgPath:    filepath.Join(dataDir, "syslog.json"),
		cfg:        syslogConfig{Enabled: true, UDP: true, TCP: true},
	}
	if b, err := os.ReadFile(s.cfgPath); err == nil {
		var cfg syslogConfig
		if err := json.Unmarshal(b, &cfg); err == nil {
			s.cfg = cfg
		}
	}
	s.apply()
	go func() {
		<-ctx.Done()
		s.mu.Lock()
		s.closeLocked()
		s.mu.Unlock()
	}()
	return s
}

// closeLocked stops the listeners synchronously, so that the port is free
// before it is opened again; the caller holds the lock.
func (s *SyslogServer) closeLocked() {
	if s.cancel != nil {
		s.cancel() // also closes the open TCP connections
		s.cancel = nil
	}
	if s.pc != nil {
		s.pc.Close()
		s.pc = nil
	}
	if s.ln != nil {
		s.ln.Close()
		s.ln = nil
	}
}

// apply opens the listeners required by the configuration.
func (s *SyslogServer) apply() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closeLocked()
	s.errUDP, s.errTCP = "", ""
	if !s.cfg.Enabled || s.parent.Err() != nil {
		log.Printf("syslog: reception disabled")
		return
	}
	ctx, cancel := context.WithCancel(s.parent)
	s.cancel = cancel
	var open []string
	if s.cfg.UDP {
		pc, err := net.ListenPacket("udp", s.addr)
		if err != nil {
			s.errUDP = err.Error()
			logEvent("syslog", 3, "UDP listener failed: %v", err)
		} else {
			if uc, ok := pc.(*net.UDPConn); ok {
				_ = uc.SetReadBuffer(8 << 20)
			}
			s.pc = pc
			go serveUDP(ctx, pc, s.sink)
			open = append(open, "udp")
		}
	}
	if s.cfg.TCP {
		ln, err := net.Listen("tcp", s.addr)
		if err != nil {
			s.errTCP = err.Error()
			logEvent("syslog", 3, "TCP listener failed: %v", err)
		} else {
			s.ln = ln
			go serveTCP(ctx, ln, s.sink)
			open = append(open, "tcp")
		}
	}
	log.Printf("syslog: listening on %s %v", s.addr, open)
}

// Configure saves and applies a new configuration.
func (s *SyslogServer) Configure(cfg syslogConfig) error {
	s.mu.Lock()
	s.cfg = cfg
	s.mu.Unlock()
	s.apply()
	return writeJSONFile(s.cfgPath, cfg)
}

// Status is the state shown in Settings > Sources.
func (s *SyslogServer) Status() map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()
	return map[string]any{
		"enabled":      s.cfg.Enabled,
		"udp":          s.cfg.UDP,
		"tcp":          s.cfg.TCP,
		"udpListening": s.pc != nil,
		"tcpListening": s.ln != nil,
		"udpError":     s.errUDP,
		"tcpError":     s.errTCP,
		"port":         s.publicPort,
		"internalAddr": s.addr,
	}
}
