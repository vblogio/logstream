package main

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"io/fs"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

// Request guards shared by every auth mode: security headers, a cross-site
// request check, and the read-only role.

type viewerKey struct{}

// asViewer marks the request as made by a read-only user.
func asViewer(r *http.Request) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), viewerKey{}, true))
}

func isViewer(r *http.Request) bool {
	v, _ := r.Context().Value(viewerKey{}).(bool)
	return v
}

func roleName(viewer bool) string {
	if viewer {
		return "viewer"
	}
	return "admin"
}

func isSafeMethod(m string) bool {
	return m == http.MethodGet || m == http.MethodHead || m == http.MethodOptions
}

// readOnly refuses the API calls that change something (tags, sources, purge)
// to read-only users. Without authentication everyone is admin.
func readOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isViewer(r) && !isSafeMethod(r.Method) && strings.HasPrefix(r.URL.Path, "/api/") {
			writeErr(w, http.StatusForbidden, &codedError{code: "read_only", msg: "read-only account: changes are reserved to administrators"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

// crossSite tells whether a request that changes something comes from another
// site (a form or script on a third-party page), using the headers browsers
// add; tools such as curl send neither and are let through.
func crossSite(r *http.Request) bool {
	switch r.Header.Get("Sec-Fetch-Site") {
	case "same-origin", "none":
		return false
	case "":
	default: // same-site, cross-site
		return true
	}
	o := r.Header.Get("Origin")
	if o == "" {
		return false
	}
	u, err := url.Parse(o)
	return err != nil || !strings.EqualFold(u.Host, r.Host)
}

// secure adds the security headers to every answer and refuses cross-site
// changes. Without authentication it also answers /auth/me, so the UI can
// warn that anyone on the network has full access.
func secure(next http.Handler, csp string, authOn bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "same-origin")
		h.Set("Content-Security-Policy", csp)
		if !isSafeMethod(r.Method) && crossSite(r) {
			writeErr(w, http.StatusForbidden, &codedError{code: "cross_site", msg: "cross-site request refused"})
			return
		}
		if !authOn && r.URL.Path == "/auth/me" {
			writeJSON(w, http.StatusOK, map[string]string{"mode": "none", "role": "admin"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

var inlineScript = regexp.MustCompile(`(?s)<script>(.*?)</script>`)

// contentSecurityPolicy allows the UI's own files, the inline scripts of the
// embedded pages (by hash) and the optional Bunny Fonts.
func contentSecurityPolicy(static fs.FS) string {
	scripts := []string{"'self'"}
	for _, page := range []string{"index.html", "login.html"} {
		b, err := fs.ReadFile(static, page)
		if err != nil {
			continue
		}
		for _, m := range inlineScript.FindAllSubmatch(b, -1) {
			sum := sha256.Sum256(m[1])
			scripts = append(scripts, "'sha256-"+base64.StdEncoding.EncodeToString(sum[:])+"'")
		}
	}
	return strings.Join([]string{
		"default-src 'self'",
		"script-src " + strings.Join(scripts, " "),
		// Inline style attributes carry the tag and project colors.
		"style-src 'self' 'unsafe-inline' https://fonts.bunny.net",
		"font-src 'self' https://fonts.bunny.net",
		"img-src 'self' data:",
		"connect-src 'self'",
		"object-src 'none'",
		"base-uri 'none'",
		"form-action 'self'",
		"frame-ancestors 'none'",
	}, "; ")
}
