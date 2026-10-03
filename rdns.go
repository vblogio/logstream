package main

import (
	"container/list"
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

	lookups chan struct{} // limits the lookups running at once
	waiters chan struct{} // limits the messages waiting for a lookup (Resolve)

	mu    sync.Mutex
	cache map[string]*list.Element // ip -> element of lru
	lru   *list.List               // *rdnsEntry, most recently used first
}

type rdnsEntry struct {
	ip      string
	name    string
	expires time.Time
	done    chan struct{} // closed once the lookup has finished
}

const (
	rdnsMaxEntries = 10000
	rdnsMaxLookups = 64
	rdnsMaxWaiters = 1024
)

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
		lookups: make(chan struct{}, rdnsMaxLookups),
		waiters: make(chan struct{}, rdnsMaxWaiters),
		cache:   make(map[string]*list.Element),
		lru:     list.New(),
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

// entry returns the cache entry of ip, starting its lookup when it is missing
// or expired, or nil when too many lookups are already running (a flood of
// unknown addresses). The least recently used entry makes room for a new one.
func (d *ReverseDNS) entry(ip string) *rdnsEntry {
	d.mu.Lock()
	defer d.mu.Unlock()
	if el := d.cache[ip]; el != nil {
		e := el.Value.(*rdnsEntry)
		if !isClosed(e.done) || time.Now().Before(e.expires) {
			d.lru.MoveToFront(el)
			return e
		}
	}
	select {
	case d.lookups <- struct{}{}:
	default:
		return nil
	}
	if el := d.cache[ip]; el != nil {
		d.lru.Remove(el)
	}
	for d.lru.Len() >= rdnsMaxEntries {
		old := d.lru.Back()
		d.lru.Remove(old)
		delete(d.cache, old.Value.(*rdnsEntry).ip)
	}
	e := &rdnsEntry{ip: ip, done: make(chan struct{})}
	d.cache[ip] = d.lru.PushFront(e)
	go d.resolve(ip, e)
	return e
}

// Lookup returns the name of ip, or "" when ip is not an IP address, has no
// PTR record, or is not resolved within `wait`. A lookup that takes longer
// keeps running in the background and fills the cache for later calls.
func (d *ReverseDNS) Lookup(ip string, wait time.Duration) string {
	if !d.enabled || net.ParseIP(ip) == nil {
		return ""
	}
	e := d.entry(ip)
	if e == nil {
		return ""
	}
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

// Resolve is Lookup without blocking the caller: fn gets the name (or "") at
// once when it is known, otherwise from a goroutine after at most `wait`. The
// syslog listeners use it so that a slow DNS server never delays the reading
// of the next messages.
func (d *ReverseDNS) Resolve(ip string, wait time.Duration, fn func(name string)) {
	if !d.enabled || net.ParseIP(ip) == nil {
		fn("")
		return
	}
	e := d.entry(ip)
	switch {
	case e == nil:
		fn("")
		return
	case isClosed(e.done):
		fn(e.name)
		return
	}
	select {
	case d.waiters <- struct{}{}:
	default:
		fn("") // too many messages waiting already
		return
	}
	go func() {
		defer func() { <-d.waiters }()
		timer := time.NewTimer(wait)
		defer timer.Stop()
		select {
		case <-e.done:
			fn(e.name)
		case <-timer.C:
			fn("")
		}
	}()
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
	defer func() { <-d.lookups }()
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
