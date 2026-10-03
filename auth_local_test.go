package main

import (
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// newLocalApp puts the local login in front of a handler that echoes "app" and returns a
// browser (client with cookies) that does not follow redirects.
func newLocalApp(t *testing.T, c authConfig) (string, *http.Client) {
	t.Helper()
	loginFailDelay = 0
	c.mode, c.dataDir = "local", t.TempDir()
	h, err := newAuth(c, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("app " + r.URL.Path)) }))
	if err != nil {
		t.Fatal(err)
	}
	jar, _ := cookiejar.New(nil)
	return "http://app.test", &http.Client{
		Jar:           jar,
		Transport:     hosts{"app.test": h},
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
}

func login(t *testing.T, c *http.Client, app, user, pass, ret string) *http.Response {
	t.Helper()
	res, err := c.PostForm(app+"/auth/login", url.Values{"user": {user}, "pass": {pass}, "r": {ret}})
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	return res
}

func TestLocalLoginFlow(t *testing.T) {
	app, c := newLocalApp(t, authConfig{user: "admin", pass: "pw"})

	res, _ := c.Get(app + "/api/logs?q=x")
	if res.StatusCode != http.StatusUnauthorized || res.Header.Get("WWW-Authenticate") != "" {
		t.Fatalf("API without session: %d %q", res.StatusCode, res.Header.Get("WWW-Authenticate"))
	}
	res, _ = c.Get(app + "/?q=disk")
	if loc := res.Header.Get("Location"); res.StatusCode != http.StatusFound || loc != "/login.html?r=%2F%3Fq%3Ddisk" {
		t.Fatalf("page without session: %d %q", res.StatusCode, loc)
	}
	for _, p := range []string{"/login.html", "/style.css", "/healthz"} {
		if code, body := get(t, c, app+p); code != http.StatusOK || body != "app "+p {
			t.Errorf("%s without session: %d %q", p, code, body)
		}
	}

	res = login(t, c, app, "admin", "wrong", "/?q=disk")
	if loc := res.Header.Get("Location"); res.StatusCode != http.StatusSeeOther || !strings.Contains(loc, "e=1") {
		t.Fatalf("wrong password: %d %q", res.StatusCode, loc)
	}
	if code, _ := get(t, c, app+"/api/logs"); code != http.StatusUnauthorized {
		t.Fatal("session created by a wrong password")
	}

	res = login(t, c, app, "admin", "pw", "//evil.example/")
	if loc := res.Header.Get("Location"); loc != "/" {
		t.Fatalf("open redirect: %q", loc)
	}
	res = login(t, c, app, "admin", "pw", "/?q=disk")
	if loc := res.Header.Get("Location"); res.StatusCode != http.StatusSeeOther || loc != "/?q=disk" {
		t.Fatalf("login: %d %q", res.StatusCode, loc)
	}
	if code, body := get(t, c, app+"/api/logs"); code != http.StatusOK || body != "app /api/logs" {
		t.Fatalf("API with session: %d %q", code, body)
	}
	if code, body := get(t, c, app+"/auth/me"); code != http.StatusOK || !strings.Contains(body, `"user":"admin"`) || !strings.Contains(body, `"mode":"local"`) {
		t.Fatalf("/auth/me: %d %s", code, body)
	}
	if res, _ := c.Get(app + "/login.html"); res.StatusCode != http.StatusFound {
		t.Errorf("login page while logged in: %d", res.StatusCode)
	}

	if res, _ := c.Get(app + "/auth/logout"); res.Header.Get("Location") != "/login.html" {
		t.Fatalf("logout: %q", res.Header.Get("Location"))
	}
	if code, _ := get(t, c, app+"/api/logs"); code != http.StatusUnauthorized {
		t.Fatal("session still valid after logout")
	}
}

func TestLocalBasicAuthForScripts(t *testing.T) {
	app, c := newLocalApp(t, authConfig{user: "admin", pass: "pw"})
	req, _ := http.NewRequest(http.MethodGet, app+"/api/logs", nil)
	req.SetBasicAuth("admin", "pw")
	if res, _ := c.Do(req); res.StatusCode != http.StatusOK {
		t.Fatalf("basic auth: %d", res.StatusCode)
	}
	req.SetBasicAuth("admin", "nope")
	if res, _ := c.Do(req); res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("wrong basic auth: %d", res.StatusCode)
	}
}

func TestLocalPasswordChangeEndsSessions(t *testing.T) {
	dir := t.TempDir()
	loginFailDelay = 0
	echo := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	h1, _ := newAuth(authConfig{mode: "local", user: "admin", pass: "old", dataDir: dir}, echo)
	h2, _ := newAuth(authConfig{mode: "local", user: "admin", pass: "new", dataDir: dir}, echo)
	cookie := signCookie(h1.(*Local).key, session{User: "admin", Exp: 9999999999})
	r, _ := http.NewRequest(http.MethodGet, "/", nil)
	r.AddCookie(&http.Cookie{Name: sessionCookie, Value: cookie})
	if _, ok := h1.(*Local).sessionUser(r); !ok {
		t.Fatal("session refused with the same password")
	}
	if _, ok := h2.(*Local).sessionUser(r); ok {
		t.Fatal("session kept after a password change")
	}
}

func TestLocalLogo(t *testing.T) {
	app, c := newLocalApp(t, authConfig{user: "admin", pass: "pw"})
	if code, _ := get(t, c, app+"/auth/logo"); code != http.StatusNotFound {
		t.Errorf("no LOGIN_LOGO: %d", code)
	}
	png := filepath.Join(t.TempDir(), "logo.png")
	_ = os.WriteFile(png, []byte("\x89PNG\r\n\x1a\nfake"), 0o644)
	app, c = newLocalApp(t, authConfig{user: "admin", pass: "pw", loginLogo: png})
	res, _ := c.Get(app + "/auth/logo")
	if res.StatusCode != http.StatusOK || res.Header.Get("Content-Type") != "image/png" {
		t.Errorf("LOGIN_LOGO: %d %q", res.StatusCode, res.Header.Get("Content-Type"))
	}
}

func TestLocalWithoutUserIsOpen(t *testing.T) {
	if h, _ := newAuth(authConfig{mode: "local"}, http.NotFoundHandler()); h == nil || isLocal(h) {
		t.Error("local mode without AUTH_USER should not protect anything")
	}
}

func isLocal(h http.Handler) bool { _, ok := h.(*Local); return ok }
