package main

import (
	"bytes"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/hmac"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	_ "crypto/sha512" // SHA-384/512 for RS384, ES384, RS512…
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Web UI authentication. AUTH_MODE=local (default) shows a login page when AUTH_USER /
// AUTH_PASS are set (auth_local.go); AUTH_MODE=oidc delegates the login to an
// OpenID Connect provider (Keycloak, Authentik, Authelia…) with the authorization code
// flow and PKCE. Only the standard library is used.

const (
	sessionCookie = "logstream_session"
	loginCookie   = "logstream_login_" // + state: one cookie per login in progress
	loginTTL      = 10 * time.Minute
	clockSkew     = time.Minute
)

type authConfig struct {
	mode         string
	user, pass   string // local mode
	issuer       string
	clientID     string
	clientSecret string
	redirectURL  string
	scopes       string
	sessionTTL   time.Duration
	dataDir      string
	loginLogo    string // local mode: PNG shown on the login page

}

// newAuth returns the middleware that protects the UI and the API (except /healthz).
func newAuth(c authConfig, next http.Handler) (http.Handler, error) {
	if c.sessionTTL <= 0 {
		c.sessionTTL = 12 * time.Hour
	}
	switch strings.ToLower(c.mode) {
	case "", "local":
		if c.user == "" {
			return next, nil
		}
		l := newLocal(c)
		l.next = next
		return l, nil
	case "oidc":
		o, err := newOIDC(c)
		if err != nil {
			return nil, err
		}
		o.next = next
		log.Printf("oidc authentication enabled (issuer %s)", c.issuer)
		return o, nil
	}
	return nil, fmt.Errorf("AUTH_MODE=%q: expected local or oidc", c.mode)
}

type oidcMeta struct {
	Issuer           string   `json:"issuer"`
	AuthEndpoint     string   `json:"authorization_endpoint"`
	TokenEndpoint    string   `json:"token_endpoint"`
	JWKSURI          string   `json:"jwks_uri"`
	EndSession       string   `json:"end_session_endpoint"`
	TokenAuthMethods []string `json:"token_endpoint_auth_methods_supported"`
}

type OIDC struct {
	cfg      authConfig
	callback string // path of OIDC_REDIRECT_URL
	secure   bool   // cookies only sent over HTTPS
	key      []byte // signs the session and login cookies
	client   *http.Client
	next     http.Handler

	mu     sync.Mutex
	meta   *oidcMeta
	keys   map[string]crypto.PublicKey
	keysAt time.Time
}

func newOIDC(c authConfig) (*OIDC, error) {
	var missing []string
	for _, v := range [][2]string{
		{"OIDC_ISSUER", c.issuer}, {"OIDC_CLIENT_ID", c.clientID},
		{"OIDC_CLIENT_SECRET", c.clientSecret}, {"OIDC_REDIRECT_URL", c.redirectURL},
	} {
		if v[1] == "" {
			missing = append(missing, v[0])
		}
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("AUTH_MODE=oidc: missing %s", strings.Join(missing, ", "))
	}
	ru, err := url.Parse(c.redirectURL)
	if err != nil || ru.Host == "" || ru.Path == "" || ru.Path == "/" {
		return nil, fmt.Errorf("OIDC_REDIRECT_URL=%q: expected a full URL such as https://logs.example.org/auth/callback", c.redirectURL)
	}
	if c.scopes == "" {
		c.scopes = "openid profile email"
	}
	if !strings.Contains(" "+c.scopes+" ", " openid ") {
		c.scopes = "openid " + c.scopes
	}
	return &OIDC{
		cfg:      c,
		callback: ru.Path,
		secure:   ru.Scheme == "https",
		key:      sessionKey(c.dataDir),
		client:   &http.Client{Timeout: 10 * time.Second},
	}, nil
}

// checkProvider reads the provider configuration at startup so a mistake shows in the logs.
func (o *OIDC) checkProvider() {
	if _, err := o.discover(); err != nil {
		log.Printf("oidc: %v", err)
	}
}

// sessionKey is kept in DATA_DIR so sessions survive a restart.
func sessionKey(dir string) []byte {
	path := filepath.Join(dir, "session.key")
	if k, err := os.ReadFile(path); err == nil && len(k) >= 32 {
		return k
	}
	k := make([]byte, 32)
	if _, err := rand.Read(k); err != nil {
		log.Fatalf("session key: %v", err)
	}
	if err := os.WriteFile(path, k, 0o600); err != nil {
		log.Printf("auth: cannot save %s (%v): sessions end when logstream restarts", path, err)
	}
	return k
}

type session struct {
	User string `json:"u"`
	Exp  int64  `json:"e"`
}

// writeAuthRequired answers API calls without a session; the UI turns it into a reload
// (and so into a new login).
func writeAuthRequired(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_, _ = w.Write([]byte(`{"error":"authentication required","code":"auth"}` + "\n"))
}

type loginState struct {
	Nonce    string `json:"n"`
	Verifier string `json:"v"`
	Return   string `json:"r"`
	Exp      int64  `json:"e"`
}

func (o *OIDC) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/healthz":
		o.next.ServeHTTP(w, r)
		return
	case o.callback:
		o.handleCallback(w, r)
		return
	case "/auth/logout":
		o.handleLogout(w, r)
		return
	}
	var s session
	if c, err := r.Cookie(sessionCookie); err == nil && verifyCookie(o.key, c.Value, &s) && time.Now().Unix() < s.Exp {
		if r.URL.Path == "/auth/me" {
			writeJSON(w, http.StatusOK, map[string]string{"mode": "oidc", "user": s.User})
			return
		}
		o.next.ServeHTTP(w, r)
		return
	}
	// Not logged in: pages go to the provider, API calls get a 401 that the UI turns
	// into a reload (and so into a new login).
	if r.Method == http.MethodGet && !strings.HasPrefix(r.URL.Path, "/api/") && r.URL.Path != "/auth/me" {
		o.startLogin(w, r)
		return
	}
	writeAuthRequired(w)
}

func (o *OIDC) startLogin(w http.ResponseWriter, r *http.Request) {
	meta, err := o.discover()
	if err != nil {
		log.Printf("oidc: %v", err)
		http.Error(w, "identity provider unreachable, try again later", http.StatusBadGateway)
		return
	}
	state, nonce, verifier := randomString(), randomString(), randomString()+randomString()
	ret := r.URL.RequestURI()
	if !strings.HasPrefix(ret, "/") || strings.HasPrefix(ret, "//") {
		ret = "/"
	}
	http.SetCookie(w, &http.Cookie{
		Name:     loginCookie + state,
		Value:    signCookie(o.key, loginState{Nonce: nonce, Verifier: verifier, Return: ret, Exp: time.Now().Add(loginTTL).Unix()}),
		Path:     "/",
		MaxAge:   int(loginTTL.Seconds()),
		HttpOnly: true,
		Secure:   o.secure,
		SameSite: http.SameSiteLaxMode, // sent back on the redirect from the provider
	})
	challenge := sha256.Sum256([]byte(verifier))
	q := url.Values{
		"response_type":         {"code"},
		"client_id":             {o.cfg.clientID},
		"redirect_uri":          {o.cfg.redirectURL},
		"scope":                 {o.cfg.scopes},
		"state":                 {state},
		"nonce":                 {nonce},
		"code_challenge":        {base64.RawURLEncoding.EncodeToString(challenge[:])},
		"code_challenge_method": {"S256"},
	}
	http.Redirect(w, r, addQuery(meta.AuthEndpoint, q), http.StatusFound)
}

func (o *OIDC) handleCallback(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	if e := q.Get("error"); e != "" {
		log.Printf("oidc: login refused by the provider: %s %s", e, q.Get("error_description"))
		http.Error(w, "login refused by the identity provider: "+e, http.StatusForbidden)
		return
	}
	state := q.Get("state")
	var ls loginState
	c, err := r.Cookie(loginCookie + state)
	if state == "" || err != nil || !verifyCookie(o.key, c.Value, &ls) || time.Now().Unix() > ls.Exp {
		http.Error(w, "login expired or started in another browser: open LogStream again", http.StatusBadRequest)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: loginCookie + state, Path: "/", MaxAge: -1, HttpOnly: true, Secure: o.secure})

	user, err := o.exchange(r, q.Get("code"), ls)
	if err != nil {
		log.Printf("oidc: login failed: %v", err)
		http.Error(w, "login failed, see the LogStream logs", http.StatusForbidden)
		return
	}
	log.Printf("oidc: %s logged in", user)
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    signCookie(o.key, session{User: user, Exp: time.Now().Add(o.cfg.sessionTTL).Unix()}),
		Path:     "/",
		MaxAge:   int(o.cfg.sessionTTL.Seconds()),
		HttpOnly: true,
		Secure:   o.secure,
		SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(w, r, ls.Return, http.StatusFound)
}

// The session ends here; the provider's own session ends on its logout page if it has one.
func (o *OIDC) handleLogout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Path: "/", MaxAge: -1, HttpOnly: true, Secure: o.secure})
	if meta, err := o.discover(); err == nil && meta.EndSession != "" {
		http.Redirect(w, r, addQuery(meta.EndSession, url.Values{"client_id": {o.cfg.clientID}}), http.StatusFound)
		return
	}
	http.Redirect(w, r, "/", http.StatusFound)
}

// exchange trades the code for tokens and returns the user name from the verified ID token.
func (o *OIDC) exchange(r *http.Request, code string, ls loginState) (string, error) {
	if code == "" {
		return "", errors.New("no code in the callback")
	}
	meta, err := o.discover()
	if err != nil {
		return "", err
	}
	form := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {o.cfg.redirectURL},
		"code_verifier": {ls.Verifier},
	}
	// client_secret_basic is the default; some providers only accept client_secret_post.
	post := len(meta.TokenAuthMethods) > 0 && !contains(meta.TokenAuthMethods, "client_secret_basic") && contains(meta.TokenAuthMethods, "client_secret_post")
	if post {
		form.Set("client_id", o.cfg.clientID)
		form.Set("client_secret", o.cfg.clientSecret)
	}
	req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, meta.TokenEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	if !post {
		req.SetBasicAuth(url.QueryEscape(o.cfg.clientID), url.QueryEscape(o.cfg.clientSecret))
	}
	res, err := o.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("token endpoint: %w", err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("token endpoint: %s: %s", res.Status, bytes.TrimSpace(body))
	}
	var tok struct {
		IDToken string `json:"id_token"`
	}
	if err := json.Unmarshal(body, &tok); err != nil || tok.IDToken == "" {
		return "", errors.New("token endpoint: no id_token in the response")
	}
	claims, err := o.verifyIDToken(tok.IDToken, ls.Nonce)
	if err != nil {
		return "", err
	}
	for _, k := range []string{"preferred_username", "email", "name", "sub"} {
		if v, _ := claims[k].(string); v != "" {
			return v, nil
		}
	}
	return "", errors.New("id_token: no sub")
}

// verifyIDToken checks the signature (keys from jwks_uri) and the claims of an ID token.
func (o *OIDC) verifyIDToken(raw, nonce string) (map[string]any, error) {
	parts := strings.Split(raw, ".")
	if len(parts) != 3 {
		return nil, errors.New("id_token: not a JWT")
	}
	var hdr struct {
		Alg string `json:"alg"`
		Kid string `json:"kid"`
	}
	if err := decodeSegment(parts[0], &hdr); err != nil {
		return nil, fmt.Errorf("id_token header: %w", err)
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, errors.New("id_token: bad signature encoding")
	}
	key, err := o.keyFor(hdr.Kid)
	if err != nil {
		return nil, err
	}
	if err := verifySignature(hdr.Alg, key, []byte(parts[0]+"."+parts[1]), sig); err != nil {
		return nil, fmt.Errorf("id_token: %w", err)
	}
	var claims map[string]any
	if err := decodeSegment(parts[1], &claims); err != nil {
		return nil, fmt.Errorf("id_token claims: %w", err)
	}
	if iss, _ := claims["iss"].(string); iss != o.cfg.issuer {
		return nil, fmt.Errorf("id_token: issuer %q, expected %q", iss, o.cfg.issuer)
	}
	var aud []string
	switch v := claims["aud"].(type) {
	case string:
		aud = []string{v}
	case []any:
		for _, a := range v {
			if s, ok := a.(string); ok {
				aud = append(aud, s)
			}
		}
	}
	if !contains(aud, o.cfg.clientID) {
		return nil, fmt.Errorf("id_token: audience %v does not include %q", aud, o.cfg.clientID)
	}
	if azp, ok := claims["azp"].(string); ok && len(aud) > 1 && azp != o.cfg.clientID {
		return nil, fmt.Errorf("id_token: azp %q", azp)
	}
	now := time.Now()
	exp, _ := claims["exp"].(float64)
	if exp == 0 || now.After(time.Unix(int64(exp), 0).Add(clockSkew)) {
		return nil, errors.New("id_token: expired (check the clocks)")
	}
	if iat, ok := claims["iat"].(float64); ok && time.Unix(int64(iat), 0).After(now.Add(clockSkew)) {
		return nil, errors.New("id_token: issued in the future (check the clocks)")
	}
	if n, _ := claims["nonce"].(string); subtle.ConstantTimeCompare([]byte(n), []byte(nonce)) != 1 {
		return nil, errors.New("id_token: wrong nonce")
	}
	return claims, nil
}

func verifySignature(alg string, key crypto.PublicKey, signed, sig []byte) error {
	if len(alg) != 5 {
		return fmt.Errorf("unsupported algorithm %q", alg)
	}
	var h crypto.Hash
	switch alg[2:] {
	case "256":
		h = crypto.SHA256
	case "384":
		h = crypto.SHA384
	case "512":
		h = crypto.SHA512
	}
	if h == 0 {
		return fmt.Errorf("unsupported algorithm %q", alg)
	}
	hh := h.New()
	hh.Write(signed)
	digest := hh.Sum(nil)
	switch k := key.(type) {
	case *rsa.PublicKey:
		switch alg[:2] {
		case "RS":
			return rsa.VerifyPKCS1v15(k, h, digest, sig)
		case "PS":
			return rsa.VerifyPSS(k, h, digest, sig, &rsa.PSSOptions{SaltLength: rsa.PSSSaltLengthEqualsHash})
		}
	case *ecdsa.PublicKey:
		size := (k.Curve.Params().BitSize + 7) / 8
		if alg[:2] != "ES" || len(sig) != 2*size {
			break
		}
		r, s := new(big.Int).SetBytes(sig[:size]), new(big.Int).SetBytes(sig[size:])
		if ecdsa.Verify(k, digest, r, s) {
			return nil
		}
		return errors.New("bad signature")
	}
	return fmt.Errorf("algorithm %q does not match the key", alg)
}

// discover reads the provider configuration once (and again after a failure).
func (o *OIDC) discover() (*oidcMeta, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.meta != nil {
		return o.meta, nil
	}
	u := strings.TrimSuffix(o.cfg.issuer, "/") + "/.well-known/openid-configuration"
	var m oidcMeta
	if err := o.getJSON(u, &m); err != nil {
		return nil, fmt.Errorf("discovery: %w", err)
	}
	if m.Issuer != o.cfg.issuer {
		return nil, fmt.Errorf("discovery: the provider says its issuer is %q, set OIDC_ISSUER to that exact value", m.Issuer)
	}
	if m.AuthEndpoint == "" || m.TokenEndpoint == "" || m.JWKSURI == "" {
		return nil, errors.New("discovery: incomplete provider configuration")
	}
	o.meta = &m
	return o.meta, nil
}

// keyFor returns the signing key kid; the key set is reloaded when the provider rotates its keys.
func (o *OIDC) keyFor(kid string) (crypto.PublicKey, error) {
	meta, err := o.discover()
	if err != nil {
		return nil, err
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	pick := func() crypto.PublicKey {
		if k, ok := o.keys[kid]; ok {
			return k
		}
		if kid == "" && len(o.keys) == 1 {
			for _, k := range o.keys {
				return k
			}
		}
		return nil
	}
	if k := pick(); k != nil {
		return k, nil
	}
	if time.Since(o.keysAt) < 10*time.Second {
		return nil, fmt.Errorf("id_token: unknown key %q", kid)
	}
	var set struct {
		Keys []struct {
			Kty string `json:"kty"`
			Kid string `json:"kid"`
			Use string `json:"use"`
			N   string `json:"n"`
			E   string `json:"e"`
			Crv string `json:"crv"`
			X   string `json:"x"`
			Y   string `json:"y"`
		} `json:"keys"`
	}
	if err := o.getJSON(meta.JWKSURI, &set); err != nil {
		return nil, fmt.Errorf("jwks: %w", err)
	}
	keys := map[string]crypto.PublicKey{}
	for _, k := range set.Keys {
		if k.Use != "" && k.Use != "sig" {
			continue
		}
		switch k.Kty {
		case "RSA":
			n, e := decodeBig(k.N), decodeBig(k.E)
			if n != nil && e != nil && e.IsInt64() {
				keys[k.Kid] = &rsa.PublicKey{N: n, E: int(e.Int64())}
			}
		case "EC":
			var c elliptic.Curve
			switch k.Crv {
			case "P-256":
				c = elliptic.P256()
			case "P-384":
				c = elliptic.P384()
			case "P-521":
				c = elliptic.P521()
			}
			x, y := decodeBig(k.X), decodeBig(k.Y)
			if c != nil && x != nil && y != nil && c.IsOnCurve(x, y) {
				keys[k.Kid] = &ecdsa.PublicKey{Curve: c, X: x, Y: y}
			}
		}
	}
	o.keys, o.keysAt = keys, time.Now()
	if k := pick(); k != nil {
		return k, nil
	}
	return nil, fmt.Errorf("id_token: unknown key %q", kid)
}

func (o *OIDC) getJSON(u string, v any) error {
	res, err := o.client.Get(u)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("%s: %s", u, res.Status)
	}
	return json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(v)
}

// Cookies are base64url(JSON) + "." + base64url(HMAC-SHA256).
func signCookie(key []byte, v any) string {
	b, _ := json.Marshal(v)
	p := base64.RawURLEncoding.EncodeToString(b)
	m := hmac.New(sha256.New, key)
	m.Write([]byte(p))
	return p + "." + base64.RawURLEncoding.EncodeToString(m.Sum(nil))
}

func verifyCookie(key []byte, s string, v any) bool {
	p, sig, ok := strings.Cut(s, ".")
	if !ok {
		return false
	}
	got, err := base64.RawURLEncoding.DecodeString(sig)
	if err != nil {
		return false
	}
	m := hmac.New(sha256.New, key)
	m.Write([]byte(p))
	if !hmac.Equal(got, m.Sum(nil)) {
		return false
	}
	return decodeSegment(p, v) == nil
}

func decodeSegment(s string, v any) error {
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, v)
}

func decodeBig(s string) *big.Int {
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil || len(b) == 0 {
		return nil
	}
	return new(big.Int).SetBytes(b)
}

func randomString() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

func addQuery(endpoint string, q url.Values) string {
	sep := "?"
	if strings.Contains(endpoint, "?") {
		sep = "&"
	}
	return endpoint + sep + q.Encode()
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
