package main

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"
)

// slowDNS never answers before the context ends.
func slowDNS() *ReverseDNS {
	d := NewReverseDNS(true, "")
	d.r = &net.Resolver{PreferGo: true, Dial: func(ctx context.Context, _, _ string) (net.Conn, error) {
		<-ctx.Done()
		return nil, errors.New("timeout")
	}}
	return d
}

func TestResolveDoesNotBlock(t *testing.T) {
	d := slowDNS()
	got := make(chan string, 1)
	start := time.Now()
	d.Resolve("192.0.2.1", 50*time.Millisecond, func(name string) { got <- name })
	if time.Since(start) > 20*time.Millisecond {
		t.Fatal("Resolve waited for the DNS server")
	}
	if name := <-got; name != "" {
		t.Errorf("name %q, want none", name)
	}

	// Not an IP address: the callback runs at once.
	called := false
	d.Resolve("router", time.Second, func(name string) { called = name == "" })
	if !called {
		t.Error("callback not called synchronously for a host name")
	}
}

func TestReverseDNSLimits(t *testing.T) {
	d := slowDNS()
	// Lookups beyond the limit are not started (and not cached).
	for i := 0; i < rdnsMaxLookups+10; i++ {
		d.Lookup(net.IPv4(10, 0, byte(i>>8), byte(i)).String(), 0)
	}
	if n := d.lru.Len(); n != rdnsMaxLookups {
		t.Errorf("%d entries, want %d", n, rdnsMaxLookups)
	}
}

func TestReverseDNSEvictsLeastRecentlyUsed(t *testing.T) {
	d := slowDNS()
	done := make(chan struct{})
	close(done)
	add := func(ip string) {
		e := &rdnsEntry{ip: ip, name: ip + ".lan", expires: time.Now().Add(time.Hour), done: done}
		d.cache[ip] = d.lru.PushFront(e)
	}
	for i := 0; i < rdnsMaxEntries; i++ {
		add(net.IPv4(10, 1, byte(i>>8), byte(i)).String())
	}
	first := net.IPv4(10, 1, 0, 0).String()
	if name := d.Lookup(first, 0); name != first+".lan" { // now the most recent
		t.Fatalf("cached name %q", name)
	}
	d.entry("192.0.2.9")
	if d.lru.Len() != rdnsMaxEntries {
		t.Errorf("%d entries, want %d", d.lru.Len(), rdnsMaxEntries)
	}
	if d.cache[first] == nil {
		t.Error("recently used entry evicted")
	}
	if d.cache[net.IPv4(10, 1, 0, 1).String()] != nil {
		t.Error("least recently used entry kept")
	}
}
