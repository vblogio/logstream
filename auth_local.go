package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// Local mode with AUTH_USER set: a login page (web/login.html) in the colors of the UI and a
// session kept in a signed cookie, like the OIDC mode. HTTP Basic credentials are still
// accepted so scripts calling the API keep working, but the browser popup is gone.

const loginPage = "/login.html"

var loginFailDelay = time.Second // slows down password guessing

type Local struct {
	user, pass string
	ttl        time.Duration
	logo       string // LOGIN_LOGO, served at /auth/logo
	key        []byte
	next       http.Handler
}

func newLocal(c authConfig) *Local {
	// The key depends on the credentials: changing AUTH_USER or AUTH_PASS ends every session.
	m := hmac.New(sha256.New, sessionKey(c.dataDir))
	m.Write([]byte("local\x00" + c.user + "\x00" + c.pass))
	if c.loginLogo != "" {
		if _, err := os.Stat(c.loginLogo); err != nil {
			log.Printf("auth: LOGIN_LOGO: %v", err)
		}
	}
	log.Printf("local authentication enabled (user %s)", c.user)
	return &Local{user: c.user, pass: c.pass, ttl: c.sessionTTL, logo: c.loginLogo, key: m.Sum(nil)}
}

func (l *Local) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/healthz", "/style.css":
		l.next.ServeHTTP(w, r)
		return
	case "/auth/logo":
		l.serveLogo(w, r)
		return
	case "/auth/login":
		l.handleLogin(w, r)
		return
	case "/auth/logout":
		http.SetCookie(w, &http.Cookie{Name: sessionCookie, Path: "/", MaxAge: -1, HttpOnly: true, Secure: isHTTPS(r)})
		http.Redirect(w, r, loginPage, http.StatusFound)
		return
	}
	user, ok := l.sessionUser(r)
	if !ok {
		if u, p, basic := r.BasicAuth(); basic && l.check(u, p) {
			user, ok = u, true
		}
	}
	switch {
	case r.URL.Path == loginPage:
		if ok {
			http.Redirect(w, r, safeReturn(r.URL.Query().Get("r")), http.StatusFound)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		l.next.ServeHTTP(w, r)
	case ok && r.URL.Path == "/auth/me":
		writeJSON(w, http.StatusOK, map[string]string{"mode": "local", "user": user})
	case ok:
		l.next.ServeHTTP(w, r)
	case r.Method == http.MethodGet && !strings.HasPrefix(r.URL.Path, "/api/") && r.URL.Path != "/auth/me":
		target := loginPage
		if ret := r.URL.RequestURI(); ret != "/" {
			target += "?" + url.Values{"r": {ret}}.Encode()
		}
		http.Redirect(w, r, target, http.StatusFound)
	default:
		writeAuthRequired(w)
	}
}

func (l *Local) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, loginPage, http.StatusFound)
		return
	}
	user, pass := r.PostFormValue("user"), r.PostFormValue("pass")
	ret := safeReturn(r.PostFormValue("r"))
	if !l.check(user, pass) {
		log.Printf("auth: failed login for %q from %s", user, clientIP(r))
		time.Sleep(loginFailDelay)
		q := url.Values{"e": {"1"}}
		if ret != "/" {
			q.Set("r", ret)
		}
		http.Redirect(w, r, loginPage+"?"+q.Encode(), http.StatusSeeOther)
		return
	}
	log.Printf("auth: %s logged in from %s", user, clientIP(r))
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    signCookie(l.key, session{User: user, Exp: time.Now().Add(l.ttl).Unix()}),
		Path:     "/",
		MaxAge:   int(l.ttl.Seconds()),
		HttpOnly: true,
		Secure:   isHTTPS(r),
		SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(w, r, ret, http.StatusSeeOther)
}

func (l *Local) sessionUser(r *http.Request) (string, bool) {
	var s session
	c, err := r.Cookie(sessionCookie)
	if err != nil || !verifyCookie(l.key, c.Value, &s) || time.Now().Unix() >= s.Exp {
		return "", false
	}
	return s.User, true
}

func (l *Local) check(user, pass string) bool {
	u := subtle.ConstantTimeCompare([]byte(user), []byte(l.user))
	p := subtle.ConstantTimeCompare([]byte(pass), []byte(l.pass))
	return u&p == 1
}

// serveLogo sends LOGIN_LOGO; without it the login page hides the image.
func (l *Local) serveLogo(w http.ResponseWriter, r *http.Request) {
	if l.logo == "" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", "no-cache")
	http.ServeFile(w, r, l.logo)
}

// safeReturn keeps the page to open after login inside logstream.
func safeReturn(ret string) string {
	if !strings.HasPrefix(ret, "/") || strings.HasPrefix(ret, "//") || strings.HasPrefix(ret, "/\\") || ret == loginPage {
		return "/"
	}
	return ret
}

// isHTTPS is true when the browser talks HTTPS, directly or through a reverse proxy.
func isHTTPS(r *http.Request) bool {
	return r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}

func clientIP(r *http.Request) string {
	if f := r.Header.Get("X-Forwarded-For"); f != "" {
		return strings.TrimSpace(strings.Split(f, ",")[0])
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
