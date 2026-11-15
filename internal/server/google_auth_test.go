package server

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/oauth2"
)

const testGoogleClientID = "123-test.apps.googleusercontent.com"

type googleTransport struct {
	t               *testing.T
	key             *rsa.PrivateKey
	claims          map[string]any
	challenge       string
	tamperSignature bool
	calls           atomic.Int32
	tokenWait       <-chan struct{}
	tokenEntered    chan<- struct{}
}

func newGoogleTransport(t *testing.T) *googleTransport {
	t.Helper()
	key, e := rsa.GenerateKey(rand.Reader, 2048)
	if e != nil {
		t.Fatal(e)
	}
	return &googleTransport{t: t, key: key}
}
func (f *googleTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	f.calls.Add(1)
	var body string
	switch r.URL.String() {
	case googleTokenURL:
		if f.tokenEntered != nil {
			f.tokenEntered <- struct{}{}
		}
		if f.tokenWait != nil {
			select {
			case <-f.tokenWait:
			case <-r.Context().Done():
				return nil, r.Context().Err()
			}
		}
		if e := r.ParseForm(); e != nil {
			f.t.Fatal(e)
		}
		if r.Form.Get("client_id") != testGoogleClientID || r.Form.Get("client_secret") != "test-secret-do-not-leak" || r.Form.Get("redirect_uri") != "http://app.test/api/auth/google/callback" || r.Form.Get("grant_type") != "authorization_code" || oauth2.S256ChallengeFromVerifier(r.Form.Get("code_verifier")) != f.challenge {
			f.t.Errorf("invalid code exchange values")
		}
		header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256","kid":"test"}`))
		raw, _ := json.Marshal(f.claims)
		unsigned := header + "." + base64.RawURLEncoding.EncodeToString(raw)
		hash := sha256.Sum256([]byte(unsigned))
		signature, e := rsa.SignPKCS1v15(rand.Reader, f.key, crypto.SHA256, hash[:])
		if e != nil {
			f.t.Fatal(e)
		}
		if f.tamperSignature {
			signature[0] ^= 1
		}
		token := unsigned + "." + base64.RawURLEncoding.EncodeToString(signature)
		raw, _ = json.Marshal(map[string]any{"access_token": "unused", "token_type": "Bearer", "id_token": token})
		body = string(raw)
	case "https://www.googleapis.com/oauth2/v3/certs":
		raw, _ := json.Marshal(map[string]any{"keys": []map[string]any{{"kty": "RSA", "alg": "RS256", "kid": "test", "use": "sig", "n": base64.RawURLEncoding.EncodeToString(f.key.N.Bytes()), "e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(f.key.E)).Bytes())}}})
		body = string(raw)
	default:
		f.t.Errorf("unexpected Google network endpoint %s", r.URL)
		body = `{}`
	}
	return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/json"}, "Cache-Control": []string{"max-age=300"}}, Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
}
func enableGoogle(t *testing.T, c *client, f *googleTransport, domains []string) {
	t.Helper()
	c.s.cfg.GoogleHTTPClient = &http.Client{Transport: f}
	status, _, w := c.call("PATCH", "/api/admin/auth/google", map[string]any{"enabled": true, "clientId": testGoogleClientID, "clientSecret": "test-secret-do-not-leak", "allowedDomains": domains})
	if status != 200 {
		t.Fatalf("config %d %s", status, w.Body)
	}
}
func startGoogle(t *testing.T, c *client, f *googleTransport) (string, *http.Cookie) {
	t.Helper()
	_, _, w := c.call("GET", "/api/auth/google/start", nil)
	target, e := url.Parse(w.Header().Get("Location"))
	if e != nil || target.Scheme != "https" || target.Host != "accounts.google.com" {
		t.Fatalf("start %s", w.Body)
	}
	q := target.Query()
	f.challenge = q.Get("code_challenge")
	if q.Get("scope") != "openid email" || q.Get("code_challenge_method") != "S256" || q.Get("redirect_uri") != "http://app.test/api/auth/google/callback" {
		t.Fatal(q)
	}
	f.claims = map[string]any{"iss": googleIssuer, "aud": testGoogleClientID, "exp": now().Add(time.Hour).Unix(), "iat": now().Unix(), "sub": "subject-one", "email": "user@gmail.com", "email_verified": true, "nonce": q.Get("nonce")}
	for _, cookie := range w.Result().Cookies() {
		if cookie.Name == c.s.googleCookieName() {
			return q.Get("state"), cookie
		}
	}
	t.Fatal("missing Google state cookie")
	return "", nil
}
func finishGoogle(c *client, state string, cookie *http.Cookie) *httptest.ResponseRecorder {
	r := httptest.NewRequest("GET", "http://app.test/api/auth/google/callback?code=test-code&state="+url.QueryEscape(state), nil)
	if cookie != nil {
		r.AddCookie(cookie)
	}
	w := httptest.NewRecorder()
	c.s.Handler().ServeHTTP(w, r)
	return w
}
func createGoogleUser(t *testing.T, c *client, email string) string {
	t.Helper()
	status, out, w := c.call("POST", "/api/admin/members", map[string]any{"name": "Google user", "email": email, "role": "member", "loginMethod": "google"})
	if status != 201 {
		t.Fatalf("create %d %s", status, w.Body)
	}
	if out["temporaryPassword"] != nil || out["user"].(map[string]any)["mustChangePassword"] != false {
		t.Fatal(out)
	}
	return out["user"].(map[string]any)["id"].(string)
}
func TestGoogleDisabledAndConfigurationSecrets(t *testing.T) {
	c := fresh(t)
	f := newGoogleTransport(t)
	c.s.cfg.GoogleHTTPClient = &http.Client{Transport: f}
	status, out, _ := c.call("GET", "/api/auth/providers", nil)
	if status != 200 || out["google"].(map[string]any)["enabled"] != false {
		t.Fatal(out)
	}
	_, _, w := c.call("GET", "/api/auth/google/start", nil)
	if w.Header().Get("Location") != "/?auth_error=google_disabled" || f.calls.Load() != 0 {
		t.Fatal("disabled provider made network request")
	}
	status, _, _ = c.call("GET", "/api/admin/auth/google", nil)
	if status != 401 {
		t.Fatal(status)
	}
	c.setup()
	enableGoogle(t, c, f, nil)
	status, out, w = c.call("GET", "/api/admin/auth/google", nil)
	if status != 200 || out["clientSecretConfigured"] != true || strings.Contains(w.Body.String(), "test-secret") {
		t.Fatal(out)
	}
	var raw []byte
	if e := c.s.db.QueryRow("SELECT value FROM state WHERE id=1").Scan(&raw); e != nil {
		t.Fatal(e)
	}
	if strings.Contains(string(raw), "test-secret-do-not-leak") {
		t.Fatal("plaintext secret persisted")
	}
	encrypted := c.s.st.GoogleAuth.Secret
	status, _, _ = c.call("PATCH", "/api/admin/auth/google", map[string]any{"enabled": true, "clientId": testGoogleClientID, "clientSecret": "", "allowedDomains": []string{}})
	if status != 200 || c.s.st.GoogleAuth.Secret != encrypted {
		t.Fatal("blank did not retain secret")
	}
	r := httptest.NewRequest("PATCH", "http://app.test/api/admin/auth/google", strings.NewReader(`{"enabled":false}`))
	r.AddCookie(c.cookie)
	rr := httptest.NewRecorder()
	c.s.Handler().ServeHTTP(rr, r)
	if rr.Code != 403 {
		t.Fatal("missing CSRF accepted")
	}
	if f.calls.Load() != 0 {
		t.Fatal("configuration made network request")
	}
}
func TestGoogleSignedLoginBindingAndPasswordBoundary(t *testing.T) {
	c := fresh(t)
	c.setup()
	f := newGoogleTransport(t)
	enableGoogle(t, c, f, nil)
	id := createGoogleUser(t, c, "user@gmail.com")
	if c.s.st.Users[id].Hash != "" {
		t.Fatal("Google-only account has password")
	}
	status, _, _ := c.call("POST", "/api/auth/login", map[string]any{"email": "user@gmail.com", "password": "strong-password-123"})
	if status != 401 {
		t.Fatal(status)
	}
	status, _, _ = c.call("POST", "/api/admin/members/"+id+"/reset-password", map[string]any{})
	if status != 409 {
		t.Fatal(status)
	}
	state, cookie := startGoogle(t, c, f)
	wrong := *cookie
	wrong.Value = random()
	w := finishGoogle(c, state, &wrong)
	if w.Header().Get("Location") != "/?auth_error=google_state_invalid" || f.calls.Load() != 0 {
		t.Fatal("wrong browser accepted")
	}
	w = finishGoogle(c, state, cookie)
	if w.Header().Get("Location") != "/#library" {
		t.Fatal(w.Header())
	}
	if !c.s.st.Users[id].GoogleLinked || c.s.st.GoogleSubjects[googleIssuer+"/subject-one"] != id {
		t.Fatal("missing persistent subject")
	}
	var googleSession *http.Cookie
	for _, cc := range w.Result().Cookies() {
		if cc.Name == c.s.sessionCookieName() {
			googleSession = cc
		}
	}
	if googleSession == nil {
		t.Fatal("missing Google session")
	}
	gc := &client{t: t, s: c.s, cookie: googleSession}
	status, me, _ := gc.call("GET", "/api/me", nil)
	if status != 200 || me["user"].(map[string]any)["id"] != id {
		t.Fatal(me)
	}
	gc.csrf = me["csrfToken"].(string)
	status, _, _ = gc.call("POST", "/api/auth/password", map[string]any{"currentPassword": "anything", "newPassword": "strong-password-123"})
	if status != 409 {
		t.Fatal("Google-only password change allowed")
	}
	calls := f.calls.Load()
	w = finishGoogle(c, state, cookie)
	if w.Header().Get("Location") != "/?auth_error=google_state_invalid" || f.calls.Load() != calls {
		t.Fatal("state replay accepted")
	}
	state, cookie = startGoogle(t, c, f)
	f.claims["sub"] = "subject-two"
	w = finishGoogle(c, state, cookie)
	if w.Header().Get("Location") != "/?auth_error=google_account_denied" {
		t.Fatal("reassigned email linked")
	}
	state, cookie = startGoogle(t, c, f)
	c.s.mu.Lock()
	u := c.s.st.Users[id]
	u.Disabled = true
	c.s.st.Users[id] = u
	c.s.mu.Unlock()
	w = finishGoogle(c, state, cookie)
	if w.Header().Get("Location") != "/?auth_error=google_account_denied" {
		t.Fatal("disabled user signed in")
	}
}
func TestGoogleSignedClaimRejections(t *testing.T) {
	cases := []struct {
		name   string
		modify func(map[string]any)
	}{
		{"issuer", func(c map[string]any) { c["iss"] = "https://evil.test" }},
		{"audience", func(c map[string]any) { c["aud"] = "another-client" }},
		{"expiry", func(c map[string]any) { c["exp"] = now().Add(-time.Minute).Unix() }},
		{"unverified", func(c map[string]any) { c["email_verified"] = false }},
		{"nonce", func(c map[string]any) { c["nonce"] = "wrong" }},
		{"non-authoritative", func(c map[string]any) { c["email"] = "user@outside.test" }},
		{"not-precreated", func(c map[string]any) { c["email"] = "unknown@gmail.com" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := fresh(t)
			c.setup()
			f := newGoogleTransport(t)
			enableGoogle(t, c, f, nil)
			createGoogleUser(t, c, "user@gmail.com")
			state, cookie := startGoogle(t, c, f)
			tc.modify(f.claims)
			w := finishGoogle(c, state, cookie)
			if w.Header().Get("Location") == "/#library" || len(c.s.st.GoogleSubjects) > 0 {
				t.Fatalf("accepted %s", tc.name)
			}
			if strings.Contains(w.Body.String(), "test-secret") {
				t.Fatal("secret leak")
			}
		})
	}
}
func TestGoogleWorkspaceAndPendingRevision(t *testing.T) {
	c := fresh(t)
	c.setup()
	f := newGoogleTransport(t)
	enableGoogle(t, c, f, []string{"company.test"})
	createGoogleUser(t, c, "user@alias.test")
	state, cookie := startGoogle(t, c, f)
	f.claims["email"] = "user@alias.test"
	w := finishGoogle(c, state, cookie)
	if w.Header().Get("Location") == "/#library" {
		t.Fatal("email suffix used instead of hd")
	}
	state, cookie = startGoogle(t, c, f)
	f.claims["email"] = "user@alias.test"
	f.claims["hd"] = "other.test"
	w = finishGoogle(c, state, cookie)
	if w.Header().Get("Location") == "/#library" {
		t.Fatal("wrong hd accepted")
	}
	state, cookie = startGoogle(t, c, f)
	f.claims["email"] = "user@alias.test"
	f.claims["hd"] = "company.test"
	w = finishGoogle(c, state, cookie)
	if w.Header().Get("Location") != "/#library" {
		t.Fatal(w.Header())
	}
	state, cookie = startGoogle(t, c, f)
	enableGoogle(t, c, f, []string{"company.test"})
	w = finishGoogle(c, state, cookie)
	if w.Header().Get("Location") != "/?auth_error=google_state_invalid" {
		t.Fatal("old config pending state survived")
	}
}
func TestGoogleNetworkDoesNotHoldMutationLock(t *testing.T) {
	c := fresh(t)
	c.setup()
	f := newGoogleTransport(t)
	enableGoogle(t, c, f, nil)
	createGoogleUser(t, c, "user@gmail.com")
	state, cookie := startGoogle(t, c, f)
	entered := make(chan struct{}, 1)
	release := make(chan struct{})
	f.tokenEntered = entered
	f.tokenWait = release
	done := make(chan *httptest.ResponseRecorder, 1)
	go func() { done <- finishGoogle(c, state, cookie) }()
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("exchange not started")
	}
	readDone := make(chan int, 1)
	go func() { status, _, _ := c.call("GET", "/api/auth/providers", nil); readDone <- status }()
	select {
	case status := <-readDone:
		if status != 200 {
			t.Fatal(status)
		}
	case <-time.After(time.Second):
		close(release)
		t.Fatal("network holds global mutex")
	}
	status, _, _ := c.call("PATCH", "/api/admin/auth/google", map[string]any{"enabled": false, "clientId": testGoogleClientID, "allowedDomains": []string{}})
	if status != 200 {
		t.Fatal(status)
	}
	close(release)
	w := <-done
	if w.Header().Get("Location") != "/?auth_error=google_disabled" {
		t.Fatal("disabled configuration used after network")
	}
}
func TestGoogleRateLimitExpiryAndAdminLockout(t *testing.T) {
	c := fresh(t)
	c.setup()
	f := newGoogleTransport(t)
	enableGoogle(t, c, f, nil)
	state, cookie := startGoogle(t, c, f)
	a := c.s.googlePending[digest(state)]
	a.ExpiresAt = now().Add(-time.Second)
	c.s.googlePending[digest(state)] = a
	w := finishGoogle(c, state, cookie)
	if w.Header().Get("Location") != "/?auth_error=google_state_invalid" {
		t.Fatal("expired accepted")
	}
	for id, u := range c.s.st.Users {
		u.LoginMethod = "google"
		u.Email = "admin@gmail.com"
		c.s.st.Users[id] = u
	}
	status, _, _ := c.call("PATCH", "/api/admin/auth/google", map[string]any{"enabled": true, "clientId": "another.apps.googleusercontent.com", "allowedDomains": []string{}})
	if status != 409 {
		t.Fatal("last Google admin credentials change allowed", status)
	}
	status, _, _ = c.call("PATCH", "/api/admin/auth/google", map[string]any{"enabled": false, "clientId": testGoogleClientID, "allowedDomains": []string{}})
	if status != 409 {
		t.Fatal("last Google admin lockout allowed", status)
	}
}
func TestGoogleExchangeCancellation(t *testing.T) {
	c := fresh(t)
	f := newGoogleTransport(t)
	c.s.cfg.GoogleHTTPClient = &http.Client{Transport: f}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	f.tokenWait = make(chan struct{})
	_, e := c.s.exchangeGoogle(ctx, testGoogleClientID, "secret", "code", "verifier")
	if e == nil {
		t.Fatal("cancellation ignored")
	}
}

func TestGoogleSignatureAndBindingSurviveRestart(t *testing.T) {
	c := fresh(t)
	c.setup()
	f := newGoogleTransport(t)
	enableGoogle(t, c, f, nil)
	id := createGoogleUser(t, c, "user@gmail.com")
	state, cookie := startGoogle(t, c, f)
	f.tamperSignature = true
	if w := finishGoogle(c, state, cookie); w.Header().Get("Location") == "/#library" {
		t.Fatal("invalid RSA signature accepted")
	}
	f.tamperSignature = false
	state, cookie = startGoogle(t, c, f)
	f.claims["iss"] = "accounts.google.com"
	if w := finishGoogle(c, state, cookie); w.Header().Get("Location") != "/#library" {
		t.Fatal(w.Header())
	}
	cfg := c.s.cfg
	if e := c.s.Close(); e != nil {
		t.Fatal(e)
	}
	reopened, e := New(cfg)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { reopened.Close() })
	c.s = reopened
	if !reopened.st.Users[id].GoogleLinked || reopened.st.GoogleSubjects[googleIssuer+"/subject-one"] != id {
		t.Fatal("subject binding lost across restart")
	}
	state, cookie = startGoogle(t, c, f)
	f.claims["sub"] = "different-after-restart"
	if w := finishGoogle(c, state, cookie); w.Header().Get("Location") != "/?auth_error=google_account_denied" {
		t.Fatal("binding bypassed after restart")
	}
}

func TestGooglePasswordAccountKeepsTemporaryPasswordGate(t *testing.T) {
	c := fresh(t)
	c.setup()
	f := newGoogleTransport(t)
	enableGoogle(t, c, f, nil)
	status, out, w := c.call("POST", "/api/admin/members", map[string]any{"name": "Password user", "email": "user@gmail.com", "role": "member", "password": "strong-password-123"})
	if status != 201 {
		t.Fatal(status, w.Body)
	}
	id := out["user"].(map[string]any)["id"].(string)
	state, cookie := startGoogle(t, c, f)
	w = finishGoogle(c, state, cookie)
	if w.Header().Get("Location") != "/#library" {
		t.Fatal(w.Header())
	}
	var session *http.Cookie
	for _, cc := range w.Result().Cookies() {
		if cc.Name == c.s.sessionCookieName() {
			session = cc
		}
	}
	gc := &client{t: t, s: c.s, cookie: session}
	status, me, _ := gc.call("GET", "/api/me", nil)
	if status != 200 {
		t.Fatal(status)
	}
	if me["user"].(map[string]any)["mustChangePassword"] != true || me["user"].(map[string]any)["loginMethod"] != nil || c.s.st.Users[id].Hash == "" {
		t.Fatal("password account semantics changed", me)
	}
	status, _, _ = gc.call("GET", "/api/artifacts", nil)
	if status != 403 {
		t.Fatal("temporary-password account bypassed gate", status)
	}
}

func TestGoogleSessionPreservesArtifactAndAdminBoundaries(t *testing.T) {
	c := fresh(t)
	c.setup()
	f := newGoogleTransport(t)
	enableGoogle(t, c, f, nil)
	id := createGoogleUser(t, c, "user@gmail.com")
	status, out := c.upload("", "", "index.html", "private admin content")
	if status != 201 {
		t.Fatal(status, out)
	}
	artifactID := out["artifact"].(map[string]any)["id"].(string)
	state, cookie := startGoogle(t, c, f)
	w := finishGoogle(c, state, cookie)
	if w.Header().Get("Location") != "/#library" {
		t.Fatal(w.Header())
	}
	var session *http.Cookie
	for _, cc := range w.Result().Cookies() {
		if cc.Name == c.s.sessionCookieName() {
			session = cc
		}
	}
	gc := &client{t: t, s: c.s, cookie: session}
	status, me, _ := gc.call("GET", "/api/me", nil)
	if status != 200 {
		t.Fatal(status)
	}
	gc.csrf = me["csrfToken"].(string)
	for _, path := range []string{"/api/artifacts/" + artifactID, "/api/admin/settings", "/api/admin/auth/google"} {
		status, _, _ = gc.call("GET", path, nil)
		expected := 403
		if strings.HasPrefix(path, "/api/artifacts/") {
			expected = 404
		}
		if status != expected {
			t.Fatal("identity boundary", path, status)
		}
	}
	status, _, _ = c.call("PATCH", "/api/artifacts/"+artifactID+"/sharing", map[string]any{"visibility": "selected", "memberIds": []string{id}, "expiresAt": nil})
	if status != 200 {
		t.Fatal(status)
	}
	status, launch, _ := gc.call("POST", "/api/artifacts/"+artifactID+"/launch", map[string]any{})
	if status != 200 {
		t.Fatal(status, launch)
	}
	grant := launch["url"].(string)
	content := httptest.NewRecorder()
	c.s.ContentHandler().ServeHTTP(content, httptest.NewRequest("GET", grant, nil))
	if content.Code != 303 {
		t.Fatal(content.Code)
	}
	viewerCookie := content.Result().Cookies()[0]
	render := strings.Split(grant, "/grant/")[0] + content.Header().Get("Location")
	request := httptest.NewRequest("GET", render, nil)
	request.AddCookie(viewerCookie)
	content = httptest.NewRecorder()
	c.s.ContentHandler().ServeHTTP(content, request)
	if content.Code != 200 {
		t.Fatal(content.Code)
	}
	status, _, _ = gc.call("POST", "/api/auth/logout", map[string]any{})
	if status != 204 {
		t.Fatal(status)
	}
	request = httptest.NewRequest("GET", render, nil)
	request.AddCookie(viewerCookie)
	content = httptest.NewRecorder()
	c.s.ContentHandler().ServeHTTP(content, request)
	if content.Code != 404 {
		t.Fatal("Google session revocation did not revoke content", content.Code)
	}
}

func TestGoogleBrowserRateLimitAllowsSharedReverseProxy(t *testing.T) {
	c := fresh(t)
	c.setup()
	f := newGoogleTransport(t)
	enableGoogle(t, c, f, nil)
	type browserCookies map[string]*http.Cookie
	start := func(cookies browserCookies, forwarded string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("GET", "http://app.test/api/auth/google/start", nil)
		r.RemoteAddr = "127.0.0.1:43100" // Every visitor arrives through one local proxy.
		r.Header.Set("X-Forwarded-For", forwarded)
		r.Header.Set("CF-Connecting-IP", forwarded)
		for _, cookie := range cookies {
			r.AddCookie(cookie)
		}
		w := httptest.NewRecorder()
		c.s.Handler().ServeHTTP(w, r)
		for _, cookie := range w.Result().Cookies() {
			cookies[cookie.Name] = cookie
		}
		return w
	}
	browsers := make([]browserCookies, 25)
	for i := range browsers {
		browsers[i] = browserCookies{}
		w := start(browsers[i], "198.51.100.1")
		if !strings.HasPrefix(w.Header().Get("Location"), googleAuthorizationURL) {
			t.Fatalf("shared proxy visitor %d blocked", i)
		}
		cookie := browsers[i][c.s.googleBrowserCookieName()]
		if cookie == nil || !cookie.HttpOnly || cookie.Domain != "" || cookie.Path != "/" || cookie.SameSite != http.SameSiteLaxMode {
			t.Fatal("browser quota cookie attributes", cookie)
		}
	}
	for i := 0; i < 9; i++ {
		w := start(browsers[0], "203.0.113."+big.NewInt(int64(i+1)).String())
		if !strings.HasPrefix(w.Header().Get("Location"), googleAuthorizationURL) {
			t.Fatal("browser blocked before ten starts", i)
		}
	}
	w := start(browsers[0], "192.0.2.200")
	if w.Header().Get("Location") != "/?auth_error=google_rate_limited" {
		t.Fatal("forwarded headers bypassed browser quota")
	}
	w = start(browsers[1], "192.0.2.200")
	if !strings.HasPrefix(w.Header().Get("Location"), googleAuthorizationURL) {
		t.Fatal("one browser locked out another proxy visitor")
	}
	if len(c.s.googleBrowsers) != 25 || len(c.s.googlePending) != 25 || f.calls.Load() != 0 {
		t.Fatal("unexpected resource/network use", len(c.s.googleBrowsers), len(c.s.googlePending), f.calls.Load())
	}
	// A stale cookie is replaced after expiry, without sharing the proxy's quota.
	key := digest(browsers[0][c.s.googleBrowserCookieName()].Value)
	a := c.s.googleBrowsers[key]
	a.Until = now().Add(-time.Second)
	c.s.googleBrowsers[key] = a
	w = start(browsers[0], "192.0.2.200")
	if !strings.HasPrefix(w.Header().Get("Location"), googleAuthorizationURL) {
		t.Fatal("expired browser quota did not reset")
	}
}

func TestGoogleResourceCapsAndSuccessReset(t *testing.T) {
	c := fresh(t)
	c.setup()
	f := newGoogleTransport(t)
	enableGoogle(t, c, f, nil)
	createGoogleUser(t, c, "user@gmail.com")
	state, cookie := startGoogle(t, c, f)
	browserKey := c.s.googlePending[digest(state)].BrowserKey
	if c.s.googleBrowsers[browserKey].Count != 1 {
		t.Fatal("missing browser counter")
	}
	w := finishGoogle(c, state, cookie)
	if w.Header().Get("Location") != "/#library" {
		t.Fatal(w.Header())
	}
	if _, exists := c.s.googleBrowsers[browserKey]; exists {
		t.Fatal("successful login retained retry counter")
	}
	state, cookie = startGoogle(t, c, f)
	c.s.googleExchanges = 16
	calls := f.calls.Load()
	w = finishGoogle(c, state, cookie)
	if w.Header().Get("Location") != "/?auth_error=google_rate_limited" || f.calls.Load() != calls || c.s.googleExchanges != 16 {
		t.Fatal("concurrent exchange limit did not reject before network")
	}
	c.s.googleExchanges = 0
	w = finishGoogle(c, state, cookie)
	if w.Header().Get("Location") != "/?auth_error=google_state_invalid" {
		t.Fatal("over-limit callback state was reusable")
	}
	c.s.googlePending = map[string]googleLoginAttempt{}
	for i := 0; i < 1000; i++ {
		c.s.googlePending[random()] = googleLoginAttempt{ExpiresAt: now().Add(time.Minute)}
	}
	_, _, w = c.call("GET", "/api/auth/google/start", nil)
	if w.Header().Get("Location") != "/?auth_error=google_rate_limited" || len(c.s.googlePending) != 1000 {
		t.Fatal("pending state capacity exceeded")
	}
	// Counter memory stays bounded without rejecting every new visitor.
	c.s.googlePending = nil
	c.s.googleBrowsers = map[string]attempt{}
	for i := 0; i < 10000; i++ {
		c.s.googleBrowsers[random()] = attempt{Count: 1, Until: now().Add(time.Minute)}
	}
	_, _, w = c.call("GET", "/api/auth/google/start", nil)
	if !strings.HasPrefix(w.Header().Get("Location"), googleAuthorizationURL) || len(c.s.googleBrowsers) != 10000 {
		t.Fatal("browser counter cache capacity", len(c.s.googleBrowsers))
	}
}
