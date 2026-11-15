package server

import (
	"context"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"golang.org/x/oauth2"
	"google.golang.org/api/idtoken"
	"google.golang.org/api/option"
)

const googleAuthorizationURL = "https://accounts.google.com/o/oauth2/v2/auth"
const googleTokenURL = "https://oauth2.googleapis.com/token"
const googleIssuer = "https://accounts.google.com"

type googleAuthConfig struct {
	Enabled        bool
	ClientID       string
	Secret         string // AES-GCM ciphertext, never a response or audit value.
	AllowedDomains []string
	Revision       string
}
type googleTokenVerifier struct {
	once      sync.Once
	validator *idtoken.Validator
	err       error
}

type googleLoginAttempt struct {
	Nonce, Verifier, Revision, BrowserKey string
	ExpiresAt                             time.Time
}

var googleClientIDPattern = regexp.MustCompile(`^[A-Za-z0-9._-]+\.apps\.googleusercontent\.com$`)
var googleDomainPattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]*[a-z0-9])?(?:\.[a-z0-9](?:[a-z0-9-]*[a-z0-9])?)+$`)

func (s *Server) googlePublicConfig() map[string]any {
	c := s.st.GoogleAuth
	return map[string]any{"enabled": c.Enabled, "clientId": c.ClientID, "clientSecretConfigured": c.Secret != "", "allowedDomains": append([]string{}, c.AllowedDomains...), "redirectURI": s.cfg.AppURL + "/api/auth/google/callback"}
}

// Only an account that has completed Google login counts as a working
// administrator fallback. Domain policy uses its previously verified hd claim.
func (s *Server) googleAccountEligible(u User, c googleAuthConfig) bool {
	if !c.Enabled || !u.GoogleLinked {
		return false
	}
	return len(c.AllowedDomains) == 0 || contains(c.AllowedDomains, s.st.GoogleHostedDomains[u.ID])
}
func (s *Server) googleSettings(w http.ResponseWriter, r *http.Request, p principal) error {
	if e := s.require(r, p, true, ""); e != nil {
		return e
	}
	if r.Method == "GET" {
		writeJSON(w, 200, s.googlePublicConfig())
		return nil
	}
	if r.Method != "PATCH" {
		return fail(405, "method_not_allowed", "Method not allowed")
	}
	var in struct {
		Enabled        bool
		ClientID       string
		ClientSecret   *string
		AllowedDomains []string
	}
	if e := decode(r, &in); e != nil {
		return e
	}
	c := s.st.GoogleAuth
	c.Enabled = in.Enabled
	c.ClientID = strings.TrimSpace(in.ClientID)
	c.AllowedDomains = []string{}
	if c.ClientID != "" && (len(c.ClientID) > 255 || !googleClientIDPattern.MatchString(c.ClientID)) {
		return fail(400, "invalid_client_id", "Enter a Google web OAuth client ID")
	}
	if len(in.AllowedDomains) > 100 {
		return fail(400, "invalid_domains", "Too many hosted domains")
	}
	for _, d := range in.AllowedDomains {
		d = strings.ToLower(strings.TrimSpace(d))
		if len(d) > 253 || !googleDomainPattern.MatchString(d) {
			return fail(400, "invalid_domains", "Enter valid hosted domains without wildcards")
		}
		for _, label := range strings.Split(d, ".") {
			if len(label) > 63 {
				return fail(400, "invalid_domains", "Enter valid hosted domains")
			}
		}
		if !contains(c.AllowedDomains, d) {
			c.AllowedDomains = append(c.AllowedDomains, d)
		}
	}
	if in.ClientSecret != nil && strings.TrimSpace(*in.ClientSecret) != "" {
		if len(*in.ClientSecret) > 4096 {
			return fail(400, "invalid_client_secret", "Client secret is too long")
		}
		encrypted, e := s.encrypt(*in.ClientSecret)
		if e != nil {
			return e
		}
		c.Secret = encrypted
	}
	if c.Enabled && (c.ClientID == "" || c.Secret == "") {
		return fail(400, "google_config_required", "Google client ID and secret are required")
	}
	usableAdmin := false
	passwordAdmin := false
	for _, u := range s.st.Users {
		if !u.Disabled && u.Role == "admin" && u.LoginMethod != "google" {
			passwordAdmin = true
		}
		if !u.Disabled && u.Role == "admin" && (u.LoginMethod != "google" || s.googleAccountEligible(u.User, c)) {
			usableAdmin = true
		}
	}
	if !passwordAdmin && (c.ClientID != s.st.GoogleAuth.ClientID || c.Secret != s.st.GoogleAuth.Secret) {
		return fail(409, "password_admin_required", "Keep a password administrator before changing Google client credentials")
	}
	if !usableAdmin {
		return fail(409, "last_admin", "Keep an active administrator with an available sign-in method")
	}
	c.Revision = random()
	if e := s.mutate(func() error {
		s.st.GoogleAuth = c
		s.audit(p, "auth.google.updated", s.st.Organization.ID, "")
		return nil
	}); e != nil {
		return e
	}
	s.googlePending = nil
	s.googleBrowsers = nil
	writeJSON(w, 200, s.googlePublicConfig())
	return nil
}
func (s *Server) googleCookieName() string {
	if strings.HasPrefix(s.cfg.AppURL, "https:") {
		return "__Host-folio_google_state"
	}
	return "folio_google_state"
}
func (s *Server) googleStateCookie(w http.ResponseWriter, value string, age int) {
	http.SetCookie(w, &http.Cookie{Name: s.googleCookieName(), Value: value, Path: "/", HttpOnly: true, Secure: strings.HasPrefix(s.cfg.AppURL, "https:"), SameSite: http.SameSiteLaxMode, MaxAge: age})
}
func googleRedirect(w http.ResponseWriter, r *http.Request, code string) {
	http.Redirect(w, r, "/?auth_error="+code, http.StatusSeeOther)
}
func (s *Server) googleBrowserCookieName() string {
	if strings.HasPrefix(s.cfg.AppURL, "https:") {
		return "__Host-folio_google_browser"
	}
	return "folio_google_browser"
}

// Browser limits are soft retry friction, not a trusted identity boundary:
// clients can discard cookies. Hard pending-state and concurrent-exchange caps
// bound resources independently. No peer-IP quota is shared by proxy users.
func (s *Server) googleRateLimit(w http.ResponseWriter, r *http.Request) (string, bool) {
	if s.googleBrowsers == nil {
		s.googleBrowsers = map[string]attempt{}
	}
	for key, a := range s.googleBrowsers {
		if !a.Until.After(now()) {
			delete(s.googleBrowsers, key)
		}
	}
	browser := ""
	if len(r.CookiesNamed(s.googleBrowserCookieName())) == 1 {
		cookie, _ := r.Cookie(s.googleBrowserCookieName())
		if len(cookie.Value) == 43 {
			if _, known := s.googleBrowsers[digest(cookie.Value)]; known {
				browser = cookie.Value
			}
		}
	}
	if browser == "" {
		// This cache is bounded even when cookie-discarding clients complete many
		// flows. Eviction resets soft friction instead of locking out new browsers.
		if len(s.googleBrowsers) >= 10000 {
			oldestKey := ""
			var oldest time.Time
			for key, a := range s.googleBrowsers {
				if oldestKey == "" || a.Until.Before(oldest) {
					oldestKey = key
					oldest = a.Until
				}
			}
			delete(s.googleBrowsers, oldestKey)
		}
		browser = random()
	}
	key := digest(browser)
	a := s.googleBrowsers[key]
	if !a.Until.After(now()) {
		a = attempt{Until: now().Add(15 * time.Minute)}
	}
	if a.Count >= 10 {
		return key, false
	}
	a.Count++
	s.googleBrowsers[key] = a
	http.SetCookie(w, &http.Cookie{Name: s.googleBrowserCookieName(), Value: browser, Path: "/", HttpOnly: true, Secure: strings.HasPrefix(s.cfg.AppURL, "https:"), SameSite: http.SameSiteLaxMode, MaxAge: 900})
	return key, true
}
func (s *Server) googleStart(w http.ResponseWriter, r *http.Request) error {
	c := s.st.GoogleAuth
	if !c.Enabled {
		googleRedirect(w, r, "google_disabled")
		return nil
	}
	for k, a := range s.googlePending {
		if !a.ExpiresAt.After(now()) {
			delete(s.googlePending, k)
		}
	}
	if len(s.googlePending) >= 1000 {
		googleRedirect(w, r, "google_rate_limited")
		return nil
	}
	browserKey, permitted := s.googleRateLimit(w, r)
	if !permitted {
		googleRedirect(w, r, "google_rate_limited")
		return nil
	}
	state := random()
	nonce := random()
	verifier := oauth2.GenerateVerifier()
	if s.googlePending == nil {
		s.googlePending = map[string]googleLoginAttempt{}
	}
	// A new attempt in the same browser invalidates its prior state.
	if old, e := r.Cookie(s.googleCookieName()); e == nil {
		delete(s.googlePending, digest(old.Value))
	}
	s.googlePending[digest(state)] = googleLoginAttempt{Nonce: nonce, Verifier: verifier, Revision: c.Revision, BrowserKey: browserKey, ExpiresAt: now().Add(5 * time.Minute)}
	s.googleStateCookie(w, state, 300)
	config := oauth2.Config{ClientID: c.ClientID, RedirectURL: s.cfg.AppURL + "/api/auth/google/callback", Scopes: []string{"openid", "email"}, Endpoint: oauth2.Endpoint{AuthURL: googleAuthorizationURL, TokenURL: googleTokenURL, AuthStyle: oauth2.AuthStyleInParams}}
	target := config.AuthCodeURL(state, oauth2.SetAuthURLParam("nonce", nonce), oauth2.SetAuthURLParam("prompt", "select_account"), oauth2.S256ChallengeOption(verifier))
	http.Redirect(w, r, target, http.StatusSeeOther)
	return nil
}
func (s *Server) googleCallback(w http.ResponseWriter, r *http.Request) error {
	s.googleStateCookie(w, "", -1)
	if !s.st.GoogleAuth.Enabled {
		googleRedirect(w, r, "google_disabled")
		return nil
	}
	q, e := url.ParseQuery(r.URL.RawQuery)
	if e != nil || len(q["state"]) != 1 || len(r.CookiesNamed(s.googleCookieName())) != 1 {
		googleRedirect(w, r, "google_state_invalid")
		return nil
	}
	cookie, e := r.Cookie(s.googleCookieName())
	state := q.Get("state")
	a, ok := s.googlePending[digest(state)]
	if e != nil || !ok || !a.ExpiresAt.After(now()) || a.Revision != s.st.GoogleAuth.Revision || subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(state)) != 1 {
		googleRedirect(w, r, "google_state_invalid")
		return nil
	}
	delete(s.googlePending, digest(state)) // consume before any network request
	if len(q["error"]) > 0 {
		googleRedirect(w, r, "google_denied")
		return nil
	}
	if len(q["code"]) != 1 || q.Get("code") == "" || len(q.Get("code")) > 8192 {
		googleRedirect(w, r, "google_failed")
		return nil
	}
	if s.googleExchanges >= 16 {
		googleRedirect(w, r, "google_rate_limited")
		return nil
	}
	c := s.st.GoogleAuth
	var secret string
	if e = s.decrypt(c.Secret, &secret); e != nil {
		googleRedirect(w, r, "google_failed")
		return nil
	}
	// Handler owns the mutex. Release it around bounded provider network work,
	// then recheck provider revision and account state before changing anything.
	s.googleExchanges++
	s.mu.Unlock()
	payload, verifyErr := s.exchangeGoogle(r.Context(), c.ClientID, secret, q.Get("code"), a.Verifier)
	s.mu.Lock()
	s.googleExchanges--
	if s.closed || !s.st.GoogleAuth.Enabled || s.st.GoogleAuth.Revision != a.Revision {
		googleRedirect(w, r, "google_disabled")
		return nil
	}
	if verifyErr != nil || !validGoogleClaims(payload, c.ClientID, a.Nonce, c.AllowedDomains) {
		googleRedirect(w, r, "google_failed")
		return nil
	}
	email, _ := payload.Claims["email"].(string)
	email, e = emailValue(email)
	if e != nil {
		googleRedirect(w, r, "google_account_denied")
		return nil
	}
	subject := googleIssuer + "/" + payload.Subject
	userID := s.st.GoogleSubjects[subject]
	var u userRecord
	if userID != "" {
		u, ok = s.st.Users[userID]
		if !ok || u.Email != email {
			googleRedirect(w, r, "google_account_denied")
			return nil
		}
	} else {
		ok = false
		for _, candidate := range s.st.Users {
			if candidate.Email == email {
				u = candidate
				ok = true
				break
			}
		}
		if !ok {
			googleRedirect(w, r, "google_account_denied")
			return nil
		}
		for linked, id := range s.st.GoogleSubjects {
			if id == u.ID && linked != subject {
				googleRedirect(w, r, "google_account_denied")
				return nil
			}
		}
	}
	if u.Disabled {
		googleRedirect(w, r, "google_account_denied")
		return nil
	}
	if e = s.mutate(func() error {
		if s.st.GoogleSubjects == nil {
			s.st.GoogleSubjects = map[string]string{}
		}
		if s.st.GoogleHostedDomains == nil {
			s.st.GoogleHostedDomains = map[string]string{}
		}
		hd, _ := payload.Claims["hd"].(string)
		s.st.GoogleHostedDomains[u.ID] = strings.ToLower(hd)
		s.st.GoogleSubjects[subject] = u.ID
		u.GoogleLinked = true
		s.st.Users[u.ID] = u
		s.audit(principal{user: &u.User}, "login.google", u.ID, "")
		return nil
	}); e != nil {
		googleRedirect(w, r, "google_failed")
		return nil
	}
	if _, e = s.startSession(w, u.User); e != nil {
		googleRedirect(w, r, "google_failed")
		return nil
	}
	delete(s.googleBrowsers, a.BrowserKey)
	http.Redirect(w, r, "/#library", http.StatusSeeOther)
	return nil
}
func validGoogleClaims(p *idtoken.Payload, audience, nonce string, domains []string) bool {
	if p == nil || (p.Issuer != googleIssuer && p.Issuer != "accounts.google.com") || p.Audience != audience || p.Expires <= now().Unix() || p.Subject == "" || len(p.Subject) > 255 {
		return false
	}
	n, ok := p.Claims["nonce"].(string)
	if !ok || subtle.ConstantTimeCompare([]byte(n), []byte(nonce)) != 1 {
		return false
	}
	verified, _ := p.Claims["email_verified"].(bool)
	email, _ := p.Claims["email"].(string)
	hd, _ := p.Claims["hd"].(string)
	email = strings.ToLower(email)
	hd = strings.ToLower(hd)
	if !verified || (!strings.HasSuffix(email, "@gmail.com") && hd == "") {
		return false
	}
	return len(domains) == 0 || (hd != "" && contains(domains, hd))
}
func (s *Server) exchangeGoogle(ctx context.Context, clientID, secret, code, verifier string) (*idtoken.Payload, error) {
	ctx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	client := &http.Client{Timeout: 10 * time.Second}
	if s.cfg.GoogleHTTPClient != nil {
		*client = *s.cfg.GoogleHTTPClient
		client.Timeout = 10 * time.Second
	}
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return errors.New("redirect refused") }
	ctx = context.WithValue(ctx, oauth2.HTTPClient, client)
	config := oauth2.Config{ClientID: clientID, ClientSecret: secret, RedirectURL: s.cfg.AppURL + "/api/auth/google/callback", Endpoint: oauth2.Endpoint{TokenURL: googleTokenURL, AuthStyle: oauth2.AuthStyleInParams}}
	token, e := config.Exchange(ctx, code, oauth2.VerifierOption(verifier))
	if e != nil {
		return nil, e
	}
	raw, ok := token.Extra("id_token").(string)
	if !ok || len(raw) > 32768 {
		return nil, errors.New("invalid identity token")
	}
	// Only Google's OIDC RS256 tokens are accepted. This also avoids passing
	// arbitrary ES256 input to the generic Google/IAP validator.
	parts := strings.Split(raw, ".")
	if len(parts) != 3 {
		return nil, errors.New("invalid identity token")
	}
	headerRaw, e := base64.RawURLEncoding.DecodeString(parts[0])
	if e != nil {
		return nil, e
	}
	var header struct {
		Alg string `json:"alg"`
	}
	if json.Unmarshal(headerRaw, &header) != nil || header.Alg != "RS256" {
		return nil, errors.New("invalid identity token algorithm")
	}
	s.googleVerifier.once.Do(func() {
		s.googleVerifier.validator, s.googleVerifier.err = idtoken.NewValidator(context.Background(), option.WithHTTPClient(client))
	})
	if s.googleVerifier.err != nil {
		return nil, s.googleVerifier.err
	}
	return s.googleVerifier.validator.Validate(ctx, raw, clientID)
}
