package main

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

// hosts routes requests to in-memory handlers (no listening socket needed).
type hosts map[string]http.Handler

func (h hosts) RoundTrip(r *http.Request) (*http.Response, error) {
	rec := httptest.NewRecorder()
	h[r.URL.Host].ServeHTTP(rec, r)
	res := rec.Result()
	res.Request = r
	return res, nil
}

// fakeIdP is a minimal OpenID provider: it logs in "alice" without asking.
type fakeIdP struct {
	mux      *http.ServeMux
	rsaKey   *rsa.PrivateKey
	ecKey    *ecdsa.PrivateKey
	useEC    bool
	codes    map[string]url.Values // code -> authorize request
	claims   func(map[string]any)  // last-minute changes to the ID token
	tokenErr bool
}

func newFakeIdP(t *testing.T) *fakeIdP {
	rk, _ := rsa.GenerateKey(rand.Reader, 2048)
	ek, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	p := &fakeIdP{rsaKey: rk, ecKey: ek, codes: map[string]url.Values{}}
	mux := http.NewServeMux()
	p.mux = mux
	iss := "http://idp.test/realm"
	mux.HandleFunc("/realm/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"issuer": iss, "authorization_endpoint": iss + "/auth", "token_endpoint": iss + "/token",
			"jwks_uri": iss + "/jwks", "end_session_endpoint": iss + "/logout",
		})
	})
	mux.HandleFunc("/realm/jwks", func(w http.ResponseWriter, r *http.Request) {
		b := func(i *big.Int) string { return base64.RawURLEncoding.EncodeToString(i.Bytes()) }
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []any{
			map[string]string{"kty": "RSA", "kid": "r1", "use": "sig", "n": b(rk.N), "e": "AQAB"},
			map[string]string{"kty": "EC", "kid": "e1", "crv": "P-256", "x": b(ek.X), "y": b(ek.Y)},
		}})
	})
	mux.HandleFunc("/realm/auth", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		code := randomString()
		p.codes[code] = q
		http.Redirect(w, r, q.Get("redirect_uri")+"?code="+code+"&state="+q.Get("state"), http.StatusFound)
	})
	mux.HandleFunc("/realm/token", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		id, secret, _ := r.BasicAuth()
		authz, ok := p.codes[r.Form.Get("code")]
		sum := sha256.Sum256([]byte(r.Form.Get("code_verifier")))
		if p.tokenErr || !ok || id != "logstream" || secret != "s3cret" ||
			base64.RawURLEncoding.EncodeToString(sum[:]) != authz.Get("code_challenge") ||
			r.Form.Get("redirect_uri") != authz.Get("redirect_uri") {
			http.Error(w, `{"error":"invalid_grant"}`, http.StatusBadRequest)
			return
		}
		delete(p.codes, r.Form.Get("code"))
		c := map[string]any{
			"iss": iss, "aud": "logstream", "sub": "123", "preferred_username": "alice",
			"exp": time.Now().Add(5 * time.Minute).Unix(), "iat": time.Now().Unix(), "nonce": authz.Get("nonce"),
		}
		if p.claims != nil {
			p.claims(c)
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"access_token": "x", "id_token": p.sign(c)})
	})
	return p
}

func (p *fakeIdP) sign(claims map[string]any) string {
	alg, kid := "RS256", "r1"
	if p.useEC {
		alg, kid = "ES256", "e1"
	}
	h, _ := json.Marshal(map[string]string{"alg": alg, "kid": kid, "typ": "JWT"})
	c, _ := json.Marshal(claims)
	in := base64.RawURLEncoding.EncodeToString(h) + "." + base64.RawURLEncoding.EncodeToString(c)
	d := sha256.Sum256([]byte(in))
	var sig []byte
	if p.useEC {
		r, s, _ := ecdsa.Sign(rand.Reader, p.ecKey, d[:])
		sig = make([]byte, 64)
		r.FillBytes(sig[:32])
		s.FillBytes(sig[32:])
	} else {
		sig, _ = rsa.SignPKCS1v15(rand.Reader, p.rsaKey, crypto.SHA256, d[:])
	}
	return in + "." + base64.RawURLEncoding.EncodeToString(sig)
}

// newOIDCApp puts logstream's auth in front of a handler that echoes "app" and returns
// a browser (client with cookies) that reaches both the app and the provider.
func newOIDCApp(t *testing.T, idp *fakeIdP) (string, *http.Client) {
	h, err := newAuth(authConfig{
		mode: "oidc", issuer: "http://idp.test/realm", clientID: "logstream", clientSecret: "s3cret",
		redirectURL: "http://app.test/auth/callback", dataDir: t.TempDir(),
	}, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("app " + r.URL.Path)) }))
	if err != nil {
		t.Fatal(err)
	}
	net := hosts{"app.test": h, "idp.test": idp.mux}
	h.(*OIDC).client.Transport = net
	jar, _ := cookiejar.New(nil)
	return "http://app.test", &http.Client{Jar: jar, Transport: net}
}

func get(t *testing.T, c *http.Client, u string) (int, string) {
	t.Helper()
	res, err := c.Get(u)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var b strings.Builder
	buf := make([]byte, 4096)
	for {
		n, err := res.Body.Read(buf)
		b.Write(buf[:n])
		if err != nil {
			break
		}
	}
	return res.StatusCode, b.String()
}

func TestOIDCLoginFlow(t *testing.T) {
	for _, ec := range []bool{false, true} {
		idp := newFakeIdP(t)
		idp.useEC = ec
		app, c := newOIDCApp(t, idp)

		if code, _ := get(t, c, app+"/api/logs"); code != http.StatusUnauthorized {
			t.Fatalf("api without session: %d", code)
		}
		if code, body := get(t, c, app+"/healthz"); code != 200 || body != "app /healthz" {
			t.Fatalf("healthz: %d %q", code, body)
		}
		// A page goes through the provider and comes back to the page asked for.
		if code, body := get(t, c, app+"/index.html?x=1"); code != 200 || body != "app /index.html" {
			t.Fatalf("login (ec=%v): %d %q", ec, code, body)
		}
		if code, body := get(t, c, app+"/api/logs"); code != 200 || body != "app /api/logs" {
			t.Fatalf("api with session: %d %q", code, body)
		}
		if code, body := get(t, c, app+"/auth/me"); code != 200 || !strings.Contains(body, `"user":"alice"`) {
			t.Fatalf("me: %d %q", code, body)
		}
		// Logout drops the session (the fake provider has no logout page: 404).
		get(t, c, app+"/auth/logout")
		if code, _ := get(t, c, app+"/api/logs"); code != http.StatusUnauthorized {
			t.Fatalf("api after logout: %d", code)
		}
	}
}

func TestOIDCRejectsBadTokens(t *testing.T) {
	cases := map[string]func(map[string]any){
		"wrong nonce":    func(c map[string]any) { c["nonce"] = "x" },
		"wrong audience": func(c map[string]any) { c["aud"] = "other" },
		"wrong issuer":   func(c map[string]any) { c["iss"] = "https://evil" },
		"expired":        func(c map[string]any) { c["exp"] = time.Now().Add(-time.Hour).Unix() },
	}
	for name, change := range cases {
		idp := newFakeIdP(t)
		idp.claims = change
		app, c := newOIDCApp(t, idp)
		if code, _ := get(t, c, app+"/"); code != http.StatusForbidden {
			t.Errorf("%s: login gave %d, expected 403", name, code)
		}
		if code, _ := get(t, c, app+"/api/logs"); code != http.StatusUnauthorized {
			t.Errorf("%s: session created", name)
		}
	}
}

func TestOIDCCallbackNeedsLoginCookie(t *testing.T) {
	idp := newFakeIdP(t)
	app, c := newOIDCApp(t, idp)
	if code, _ := get(t, c, app+"/auth/callback?code=abc&state=forged"); code != http.StatusBadRequest {
		t.Fatalf("forged callback: %d", code)
	}
}

func TestOIDCForgedSessionCookie(t *testing.T) {
	idp := newFakeIdP(t)
	app, c := newOIDCApp(t, idp)
	u, _ := url.Parse(app)
	payload := base64.RawURLEncoding.EncodeToString([]byte(`{"u":"mallory","e":9999999999}`))
	c.Jar.SetCookies(u, []*http.Cookie{{Name: sessionCookie, Value: payload + ".AAAA"}})
	if code, _ := get(t, c, app+"/api/logs"); code != http.StatusUnauthorized {
		t.Fatalf("forged session accepted: %d", code)
	}
}

func TestAuthModeConfig(t *testing.T) {
	next := http.NotFoundHandler()
	if _, err := newAuth(authConfig{mode: "oidc"}, next); err == nil || !strings.Contains(err.Error(), "OIDC_CLIENT_ID") {
		t.Errorf("missing variables not reported: %v", err)
	}
	if _, err := newAuth(authConfig{mode: "ldap"}, next); err == nil {
		t.Error("unknown mode accepted")
	}
	if h, err := newAuth(authConfig{mode: "local"}, next); err != nil || h == nil {
		t.Errorf("local mode: %v", err)
	}
}
