package main

import (
	"context"
	"net"
	"strings"
	"sync"
	"time"
)

// ReverseDNS resolves IP addresses to host names (PTR records), with a cache.
type ReverseDNS struct {
	enabled bool
	r       *net.Resolver
	posTTL  time.Duration // cache duration of a found name
	negTTL  time.Duration // cache duration of "no name"

	mu    sync.Mutex
	cache map[string]*rdnsEntry
}

type rdnsEntry struct {
	name    string
	expires time.Time
	done    chan struct{} // closed once the lookup has finished
}

const rdnsMaxEntries = 10000

// NewReverseDNS uses the system resolver, or `server` ("ip" or "ip:port") when set.
func NewReverseDNS(enabled bool, server string) *ReverseDNS {
	r := net.DefaultResolver
	if server != "" {
		if _, _, err := net.SplitHostPort(server); err != nil {
			server = net.JoinHostPort(server, "53")
		}
		dialer := net.Dialer{Timeout: 2 * time.Second}
		r = &net.Resolver{
			PreferGo: true,
			Dial: func(ctx context.Context, network, _ string) (net.Conn, error) {
				return dialer.DialContext(ctx, network, server)
			},
		}
	}
	return &ReverseDNS{
		enabled: enabled,
		r:       r,
		posTTL:  time.Hour,
		negTTL:  10 * time.Minute,
		cache:   make(map[string]*rdnsEntry),
	}
}

func isClosed(ch chan struct{}) bool {
	select {
	case <-ch:
		return true
	default:
		return false
	}
}

// Lookup returns the name of ip, or "" when ip is not an IP address, has no
// PTR record, or is not resolved within `wait`. A lookup that takes longer
// keeps running in the background and fills the cache for later calls.
func (d *ReverseDNS) Lookup(ip string, wait time.Duration) string {
	if !d.enabled || net.ParseIP(ip) == nil {
		return ""
	}
	d.mu.Lock()
	e := d.cache[ip]
	if e == nil || (isClosed(e.done) && time.Now().After(e.expires)) {
		if len(d.cache) >= rdnsMaxEntries {
			d.cache = make(map[string]*rdnsEntry)
		}
		e = &rdnsEntry{done: make(chan struct{})}
		d.cache[ip] = e
		go d.resolve(ip, e)
	}
	d.mu.Unlock()

	if !isClosed(e.done) {
		timer := time.NewTimer(wait)
		defer timer.Stop()
		select {
		case <-e.done:
		case <-timer.C:
			return ""
		}
	}
	return e.name
}

// LookupMany resolves several addresses in parallel; unresolved ones are absent.
func (d *ReverseDNS) LookupMany(ips []string, wait time.Duration) map[string]string {
	out := make(map[string]string)
	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, ip := range ips {
		wg.Add(1)
		go func(ip string) {
			defer wg.Done()
			if name := d.Lookup(ip, wait); name != "" {
				mu.Lock()
				out[ip] = name
				mu.Unlock()
			}
		}(ip)
	}
	wg.Wait()
	return out
}

func (d *ReverseDNS) resolve(ip string, e *rdnsEntry) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	ttl := d.negTTL
	if names, err := d.r.LookupAddr(ctx, ip); err == nil && len(names) > 0 {
		e.name = strings.TrimSuffix(names[0], ".")
		ttl = d.posTTL
	}
	e.expires = time.Now().Add(ttl)
	close(e.done)
}
