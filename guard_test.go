package main

import (
	"errors"
	"io/fs"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

var echo = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("app " + r.URL.Path)) })

func TestSecureHeadersAndCrossSite(t *testing.T) {
	h := secure(echo, "default-src 'self'", false)
	cases := []struct {
		method string
		hdr    map[string]string
		want   int
	}{
		{"GET", map[string]string{"Sec-Fetch-Site": "cross-site"}, 200}, // reading is fine
		{"POST", nil, 200}, // curl, scripts
		{"POST", map[string]string{"Sec-Fetch-Site": "same-origin"}, 200},
		{"POST", map[string]string{"Sec-Fetch-Site": "cross-site"}, 403},
		{"DELETE", map[string]string{"Sec-Fetch-Site": "same-site"}, 403},
		{"POST", map[string]string{"Origin": "http://logs.lan:8080"}, 200},
		{"POST", map[string]string{"Origin": "https://evil.example"}, 403},
		{"PUT", map[string]string{"Origin": "null"}, 403},
	}
	for _, c := range cases {
		r := httptest.NewRequest(c.method, "http://logs.lan:8080/api/purge", nil)
		for k, v := range c.hdr {
			r.Header.Set(k, v)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, r)
		if rec.Code != c.want {
			t.Errorf("%s %v: %d, want %d", c.method, c.hdr, rec.Code, c.want)
		}
		if rec.Header().Get("Content-Security-Policy") == "" || rec.Header().Get("X-Frame-Options") != "DENY" {
			t.Errorf("%s %v: security headers missing", c.method, c.hdr)
		}
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/auth/me", nil))
	if !strings.Contains(rec.Body.String(), `"mode":"none"`) || !strings.Contains(rec.Body.String(), `"role":"admin"`) {
		t.Errorf("/auth/me without auth: %s", rec.Body)
	}
	rec = httptest.NewRecorder()
	secure(echo, "", true).ServeHTTP(rec, httptest.NewRequest("GET", "/auth/me", nil))
	if rec.Body.String() != "app /auth/me" {
		t.Errorf("/auth/me with auth answered by the guard: %s", rec.Body)
	}
}

func TestContentSecurityPolicyHashesInlineScripts(t *testing.T) {
	static, _ := fs.Sub(webFS, "web")
	csp := contentSecurityPolicy(static)
	// index.html and login.html each have inline scripts.
	if n := strings.Count(csp, "'sha256-"); n < 3 {
		t.Errorf("%d script hashes in %q", n, csp)
	}
	for _, want := range []string{"frame-ancestors 'none'", "connect-src 'self'", "https://fonts.bunny.net"} {
		if !strings.Contains(csp, want) {
			t.Errorf("CSP lacks %q", want)
		}
	}
}

func TestLocalViewerIsReadOnly(t *testing.T) {
	loginFailDelay = 0
	h, err := newAuth(authConfig{mode: "local", user: "admin", pass: "pw", viewerUser: "guest", viewerPass: "ro", dataDir: t.TempDir()}, readOnly(echo))
	if err != nil {
		t.Fatal(err)
	}
	jar, _ := cookiejar.New(nil)
	c := &http.Client{Jar: jar, Transport: hosts{"app.test": h}, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	app := "http://app.test"

	login(t, c, app, "guest", "ro", "/")
	if code, body := get(t, c, app+"/auth/me"); code != 200 || !strings.Contains(body, `"role":"viewer"`) {
		t.Fatalf("me: %d %s", code, body)
	}
	if code, _ := get(t, c, app+"/api/logs"); code != 200 {
		t.Errorf("viewer reading logs: %d", code)
	}
	res, err := c.Post(app+"/api/purge", "application/json", strings.NewReader(`{"confirm":"PURGE"}`))
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusForbidden {
		t.Errorf("viewer purge: %d, want 403", res.StatusCode)
	}

	// The admin account still changes things, also through Basic auth.
	req, _ := http.NewRequest("PUT", app+"/api/syslog", strings.NewReader("{}"))
	req.SetBasicAuth("admin", "pw")
	if res, _ := (&http.Client{Transport: hosts{"app.test": h}}).Do(req); res.StatusCode != 200 {
		t.Errorf("admin change: %d", res.StatusCode)
	}
	req, _ = http.NewRequest("PUT", app+"/api/syslog", strings.NewReader("{}"))
	req.SetBasicAuth("guest", "ro")
	if res, _ := (&http.Client{Transport: hosts{"app.test": h}}).Do(req); res.StatusCode != http.StatusForbidden {
		t.Errorf("viewer change through Basic auth: %d", res.StatusCode)
	}
}

func TestOIDCAdminGroup(t *testing.T) {
	for _, tc := range []struct {
		groups any
		role   string
	}{
		{[]any{"staff", "/logstream-admins"}, "admin"},
		{[]any{"staff"}, "viewer"},
		{nil, "viewer"},
	} {
		idp := newFakeIdP(t)
		idp.claims = func(c map[string]any) {
			if tc.groups != nil {
				c["groups"] = tc.groups
			}
		}
		h, err := newAuth(authConfig{
			mode: "oidc", issuer: "http://idp.test/realm", clientID: "logstream", clientSecret: "s3cret",
			redirectURL: "http://app.test/auth/callback", dataDir: t.TempDir(), adminGroup: "logstream-admins",
		}, readOnly(echo))
		if err != nil {
			t.Fatal(err)
		}
		netw := hosts{"app.test": h, "idp.test": idp.mux}
		h.(*OIDC).client.Transport = netw
		jar, _ := cookiejar.New(nil)
		c := &http.Client{Jar: jar, Transport: netw}
		get(t, c, "http://app.test/")
		if _, body := get(t, c, "http://app.test/auth/me"); !strings.Contains(body, `"role":"`+tc.role+`"`) {
			t.Errorf("groups %v: %s, want role %s", tc.groups, body, tc.role)
		}
	}
}

func TestHasGroup(t *testing.T) {
	if !hasGroup("ops logstream-admins", "/logstream-admins") || !hasGroup([]any{"a", "b"}, "b") || hasGroup(42, "b") {
		t.Error("hasGroup")
	}
}

func TestTCPIdleTimeout(t *testing.T) {
	a, b := net.Pipe()
	defer b.Close()
	c := idleConn{a, 30 * time.Millisecond}
	go func() { _, _ = b.Write([]byte("x")) }()
	buf := make([]byte, 1)
	if _, err := c.Read(buf); err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	if _, err := c.Read(buf); !errors.Is(err, os.ErrDeadlineExceeded) {
		t.Fatalf("silent connection: %v, want a deadline error", err)
	}
	if time.Since(start) > time.Second {
		t.Error("deadline not applied")
	}
}
