// Package server implements Folio's shared, authorization-enforcing HTTP API.
package server

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/YangTaeyoung/folio/internal/blob"
	"github.com/YangTaeyoung/folio/internal/dns"
	_ "modernc.org/sqlite"
)

type DNSService interface{ Close() error }
type Config struct {
	DataDir, AppURL, ContentURL, BootstrapToken, Version string
	WebFS                                                fs.FS
	BlobFactory                                          func(blob.Config) (blob.Store, error)
	DNS                                                  DNSService
	DNSResolver                                          dns.Resolver
	DomainConnect                                        *dns.DomainConnect
	CloudflareAPIBase                                    string
	HTTPClient                                           *http.Client
}
type User struct {
	ID                 string    `json:"id"`
	Email              string    `json:"email"`
	Name               string    `json:"name"`
	Role               string    `json:"role"`
	Disabled           bool      `json:"disabled"`
	MustChangePassword bool      `json:"mustChangePassword"`
	CreatedAt          time.Time `json:"createdAt"`
	PasswordHash       string    `json:"-"`
}
type userRecord struct {
	User
	Hash string `json:"passwordHash"`
}
type Organization struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}
type Policy struct {
	QuotaBytes     int64  `json:"quotaBytes"`
	MaxUploadBytes int64  `json:"maxUploadBytes"`
	RetentionDays  int    `json:"retentionDays"`
	OverflowPolicy string `json:"overflowPolicy"`
}
type File struct {
	Path     string `json:"path"`
	MIMEType string `json:"mimeType"`
	Size     int64  `json:"size"`
	Key      string `json:"key,omitempty"`
}
type Version struct {
	ID         string    `json:"id"`
	ArtifactID string    `json:"artifactId"`
	Number     int       `json:"number"`
	CreatedAt  time.Time `json:"createdAt"`
	SizeBytes  int64     `json:"sizeBytes"`
	BackendID  string    `json:"backendId"`
	Files      []File    `json:"files"`
	Message    string    `json:"message"`
}
type Artifact struct {
	ID               string     `json:"id"`
	Slug             string     `json:"slug"`
	Title            string     `json:"title"`
	Description      string     `json:"description"`
	OwnerID          string     `json:"ownerId"`
	OwnerName        string     `json:"ownerName"`
	Visibility       string     `json:"visibility"`
	MemberIDs        []string   `json:"memberIds"`
	Archived         bool       `json:"archived"`
	ExpiresAt        *time.Time `json:"expiresAt"`
	CreatedAt        time.Time  `json:"createdAt"`
	UpdatedAt        time.Time  `json:"updatedAt"`
	SizeBytes        int64      `json:"sizeBytes"`
	CurrentVersionID string     `json:"currentVersionId"`
	ShareURL         string     `json:"shareUrl"`
	Kind             string     `json:"kind"`
}
type Session struct {
	UserID, CSRF string
	ExpiresAt    time.Time
}
type Token struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	UserID     string     `json:"-"`
	Hash       string     `json:"-"`
	CreatedAt  time.Time  `json:"createdAt"`
	ExpiresAt  *time.Time `json:"expiresAt"`
	LastUsedAt *time.Time `json:"lastUsedAt"`
	Scopes     []string   `json:"scopes"`
}
type tokenRecord struct {
	Token
	OwnerID string
	Digest  string
}
type Viewer struct {
	ArtifactID, VersionID, Host, UserID, Credential, Kind string
	ExpiresAt                                             time.Time
}
type Backend struct {
	ID                    string    `json:"id"`
	Name                  string    `json:"name"`
	Provider              string    `json:"provider"`
	Bucket                string    `json:"bucket"`
	Region                string    `json:"region"`
	Endpoint              string    `json:"endpoint"`
	Prefix                string    `json:"prefix"`
	AccountName           string    `json:"accountName"`
	CredentialsConfigured bool      `json:"credentialsConfigured"`
	IsDefault             bool      `json:"isDefault"`
	CreatedAt             time.Time `json:"createdAt"`
	Encrypted             string    `json:"-"`
}
type backendRecord struct {
	Backend
	Secret string
}
type Audit struct {
	ID        string    `json:"id"`
	ActorID   string    `json:"actorId"`
	ActorName string    `json:"actorName"`
	Action    string    `json:"action"`
	TargetID  string    `json:"targetId"`
	CreatedAt time.Time `json:"createdAt"`
	Detail    string    `json:"detail"`
}
type Garbage struct {
	BackendID, Key string
	UploadID       string
	Size           int64
	Error          string
}
type state struct {
	Organization Organization
	Policy       Policy
	Users        map[string]userRecord
	Sessions     map[string]Session
	Tokens       map[string]tokenRecord
	Artifacts    map[string]Artifact
	Versions     map[string][]Version
	Backends     map[string]backendRecord
	Grants       map[string]Viewer
	Viewers      map[string]Viewer
	Garbage      []Garbage
	Audit        []Audit
	Domains      map[string]domainRecord
	DNSSecret    string
}
type Server struct {
	mu              sync.Mutex
	cfg             Config
	db              *sql.DB
	st              state
	key             []byte
	bootstrap       string
	origin          string
	appHost         string
	factory         func(blob.Config) (blob.Store, error)
	dns             DNSService
	loginAttempts   map[string]attempt
	inFlight        map[string]int
	pendingBackends map[string]int
	pendingUploads  map[string]bool
	closed          bool
}
type attempt struct {
	Count int
	Until time.Time
}
type principal struct {
	user             *User
	credential, kind string
	scopes           []string
}
type apiError struct {
	status    int
	code, msg string
}

func (e *apiError) Error() string             { return e.msg }
func fail(status int, code, msg string) error { return &apiError{status, code, msg} }
func random() string {
	b := make([]byte, 32)
	if _, e := rand.Read(b); e != nil {
		panic(e)
	}
	return base64.RawURLEncoding.EncodeToString(b)
}
func digest(v string) string { h := sha256.Sum256([]byte(v)); return hex.EncodeToString(h[:]) }
func now() time.Time         { return time.Now().UTC() }
func New(c Config) (*Server, error) {
	if c.DataDir == "" {
		return nil, errors.New("DataDir is required")
	}
	if c.AppURL == "" {
		c.AppURL = "http://folio.localhost:4310"
	}
	if c.ContentURL == "" {
		c.ContentURL = "http://{artifact}.content.folio.localhost:4311"
	}
	c.AppURL = strings.TrimRight(c.AppURL, "/")
	a, e := url.Parse(c.AppURL)
	if e != nil || a.Host == "" || (a.Scheme != "http" && a.Scheme != "https") || a.Path != "" || a.RawQuery != "" || a.User != nil {
		return nil, errors.New("AppURL must be an HTTP(S) origin")
	}
	if strings.Count(c.ContentURL, "{artifact}") != 1 {
		return nil, errors.New("ContentURL must contain one {artifact} hostname placeholder")
	}
	cu, e := url.Parse(strings.ReplaceAll(c.ContentURL, "{artifact}", "sample"))
	if e != nil || cu.Host == "" || (cu.Scheme != "http" && cu.Scheme != "https") || cu.Path != "" || cu.RawQuery != "" || cu.User != nil || cu.Hostname() == a.Hostname() || !strings.Contains(strings.Split(c.ContentURL, "://")[1], "{artifact}") {
		return nil, errors.New("ContentURL must be a distinct artifact-isolated HTTP(S) origin")
	}
	if e = os.MkdirAll(c.DataDir, 0700); e != nil {
		return nil, e
	}
	if e = os.Chmod(c.DataDir, 0700); e != nil {
		return nil, e
	}
	keyPath := filepath.Join(c.DataDir, "data.key")
	key, e := os.ReadFile(keyPath)
	if os.IsNotExist(e) {
		key = make([]byte, 32)
		rand.Read(key)
		e = os.WriteFile(keyPath, key, 0600)
	}
	if e != nil || len(key) != 32 {
		return nil, errors.New("cannot load installation data key")
	}
	db, e := sql.Open("sqlite", filepath.Join(c.DataDir, "folio.db"))
	if e != nil {
		return nil, e
	}
	db.SetMaxOpenConns(1)
	if _, e = db.Exec("PRAGMA journal_mode=WAL; PRAGMA synchronous=FULL; CREATE TABLE IF NOT EXISTS state(id INTEGER PRIMARY KEY CHECK(id=1), value BLOB NOT NULL)"); e != nil {
		db.Close()
		return nil, e
	}
	os.Chmod(filepath.Join(c.DataDir, "folio.db"), 0600)
	s := &Server{cfg: c, db: db, key: key, bootstrap: c.BootstrapToken, origin: a.Scheme + "://" + a.Host, appHost: strings.ToLower(a.Host), factory: c.BlobFactory, dns: c.DNS, loginAttempts: map[string]attempt{}, inFlight: map[string]int{}, pendingBackends: map[string]int{}, pendingUploads: map[string]bool{}}
	if s.factory == nil {
		s.factory = blob.New
	}
	if s.dns == nil {
		s.dns = &dns.Manager{DataDir: c.DataDir, AppURL: c.AppURL, ContentURL: c.ContentURL}
	}
	if s.cfg.DomainConnect != nil {
		s.cfg.DomainConnect.StateKey = s.key
		s.cfg.DomainConnect.CallbackURL = c.AppURL + "/api/dns/domainconnect/callback"
	}
	var raw []byte
	e = db.QueryRow("SELECT value FROM state WHERE id=1").Scan(&raw)
	if e == nil {
		e = json.Unmarshal(raw, &s.st)
	} else if errors.Is(e, sql.ErrNoRows) {
		s.st = state{Organization: Organization{ID: random(), Name: "Folio"}, Policy: Policy{1 << 30, 128 << 20, 0, "reject"}, Users: map[string]userRecord{}, Sessions: map[string]Session{}, Tokens: map[string]tokenRecord{}, Artifacts: map[string]Artifact{}, Versions: map[string][]Version{}, Backends: map[string]backendRecord{}, Grants: map[string]Viewer{}, Viewers: map[string]Viewer{}, Domains: map[string]domainRecord{}, Audit: []Audit{}, Garbage: []Garbage{}}
		b := Backend{ID: random(), Name: "Local storage", Provider: "local", IsDefault: true, CreatedAt: now()}
		s.st.Backends[b.ID] = backendRecord{Backend: b}
		e = s.save()
	}
	if e != nil {
		db.Close()
		return nil, e
	}
	if len(s.st.Users) == 0 && s.bootstrap == "" {
		p := filepath.Join(c.DataDir, "bootstrap.token")
		v, e := os.ReadFile(p)
		if os.IsNotExist(e) {
			v = []byte(random())
			e = os.WriteFile(p, v, 0600)
		}
		if e != nil {
			db.Close()
			return nil, e
		}
		s.bootstrap = strings.TrimSpace(string(v))
	}
	return s, nil
}
func (s *Server) BootstrapToken() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.st.Users) > 0 {
		return ""
	}
	return s.bootstrap
}
func (s *Server) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	if s.dns != nil {
		s.dns.Close()
	}
	return s.db.Close()
}
func (s *Server) save() error {
	raw, e := json.Marshal(s.st)
	if e != nil {
		return e
	}
	_, e = s.db.Exec("INSERT INTO state(id,value) VALUES(1,?) ON CONFLICT(id) DO UPDATE SET value=excluded.value", raw)
	return e
}

// mutate rolls memory back on SQLite commit failure, keeping admission and tombstones atomic.
func (s *Server) mutate(fn func() error) error {
	raw, e := json.Marshal(s.st)
	if e != nil {
		return e
	}
	if e = fn(); e == nil {
		e = s.save()
	}
	if e != nil {
		var restored state
		json.Unmarshal(raw, &restored)
		s.st = restored
	}
	return e
}
func (s *Server) audit(p principal, action, target, detail string) {
	a := Audit{ID: random(), Action: action, TargetID: target, CreatedAt: now(), Detail: detail}
	if p.user != nil {
		a.ActorID = p.user.ID
		a.ActorName = p.user.Name
	}
	s.st.Audit = append(s.st.Audit, a)
}
func (s *Server) encrypt(v any) (string, error) {
	raw, e := json.Marshal(v)
	if e != nil {
		return "", e
	}
	b, e := aes.NewCipher(s.key)
	if e != nil {
		return "", e
	}
	g, e := cipher.NewGCM(b)
	if e != nil {
		return "", e
	}
	nonce := make([]byte, g.NonceSize())
	rand.Read(nonce)
	return base64.RawStdEncoding.EncodeToString(g.Seal(nonce, nonce, raw, []byte("folio-secrets-v1"))), nil
}
func (s *Server) decrypt(v string, out any) error {
	if v == "" {
		return nil
	}
	raw, e := base64.RawStdEncoding.DecodeString(v)
	if e != nil {
		return e
	}
	b, _ := aes.NewCipher(s.key)
	g, _ := cipher.NewGCM(b)
	if len(raw) < g.NonceSize() {
		return errors.New("invalid encrypted config")
	}
	raw, e = g.Open(nil, raw[:g.NonceSize()], raw[g.NonceSize():], []byte("folio-secrets-v1"))
	if e != nil {
		return e
	}
	return json.Unmarshal(raw, out)
}
func (s *Server) store(id string) (blob.Store, error) {
	b, ok := s.st.Backends[id]
	if !ok {
		return nil, errors.New("storage backend missing")
	}
	c := blob.Config{}
	if e := s.decrypt(b.Secret, &c); e != nil {
		return nil, e
	}
	c.ID = b.ID
	c.Name = b.Name
	c.Provider = b.Provider
	c.Root = filepath.Join(s.cfg.DataDir, "objects", b.ID)
	c.Bucket = b.Bucket
	c.Region = b.Region
	c.Endpoint = b.Endpoint
	c.Prefix = b.Prefix
	c.AccountName = b.AccountName
	return s.factory(c)
}
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if v != nil {
		json.NewEncoder(w).Encode(v)
	}
}
func errorJSON(w http.ResponseWriter, e error) {
	a := &apiError{500, "internal_error", "Operation failed"}
	errors.As(e, &a)
	writeJSON(w, a.status, map[string]any{"error": map[string]string{"code": a.code, "message": a.msg}})
}
func decode(r *http.Request, v any) error {
	defer r.Body.Close()
	r.Body = http.MaxBytesReader(nil, r.Body, 1<<20)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if e := d.Decode(v); e != nil {
		return fail(400, "invalid_json", "Invalid JSON body")
	}
	var rest any
	if e := d.Decode(&rest); !errors.Is(e, io.EOF) {
		return fail(400, "invalid_json", "Expected one JSON object")
	}
	return nil
}
func contains(v []string, w string) bool {
	for _, x := range v {
		if x == w {
			return true
		}
	}
	return false
}
func (s *Server) publicVersion(v Version) Version {
	v.Files = append([]File(nil), v.Files...)
	for i := range v.Files {
		v.Files[i].Key = ""
	}
	return v
}
func (s *Server) publicArtifact(a Artifact) Artifact {
	a.MemberIDs = append([]string{}, a.MemberIDs...)
	if u, ok := s.st.Users[a.OwnerID]; ok {
		a.OwnerName = u.Name
	}
	a.ShareURL = s.cfg.AppURL + "/s/" + a.Slug
	return a
}
func (s *Server) read(p principal, a Artifact) bool {
	if p.user != nil && (p.user.Role == "admin" || a.OwnerID == p.user.ID) {
		return true
	}
	if a.ExpiresAt != nil && !a.ExpiresAt.After(now()) {
		return false
	}
	if a.Visibility == "public" {
		return true
	}
	if p.user == nil {
		return false
	}
	return a.Visibility == "organization" || (a.Visibility == "selected" && contains(a.MemberIDs, p.user.ID))
}
func (s *Server) own(p principal, a Artifact) bool {
	return p.user != nil && (p.user.ID == a.OwnerID || p.user.Role == "admin")
}
func (s *Server) used() int64 {
	var n int64
	for _, versions := range s.st.Versions {
		for _, v := range versions {
			n += v.SizeBytes
		}
	}
	return n
}
func (s *Server) settings() map[string]any {
	return map[string]any{"organization": s.st.Organization, "policy": s.st.Policy, "appUrl": s.cfg.AppURL, "contentUrl": s.cfg.ContentURL}
}
func (s *Server) sessionCookieName() string {
	if strings.HasPrefix(s.cfg.AppURL, "https:") {
		return "__Host-folio_session"
	}
	return "folio_session"
}
func (s *Server) principal(r *http.Request) principal {
	if h := r.Header.Get("Authorization"); h != "" {
		if !strings.HasPrefix(h, "Bearer ") {
			return principal{}
		}
		hash := digest(strings.TrimPrefix(h, "Bearer "))
		for id, t := range s.st.Tokens {
			if t.Digest == hash && (t.ExpiresAt == nil || t.ExpiresAt.After(now())) {
				u, ok := s.st.Users[t.OwnerID]
				if ok && !u.Disabled && !u.MustChangePassword {
					used := now()
					t.LastUsedAt = &used
					s.st.Tokens[id] = t
					s.save()
					return principal{&u.User, hash, "token", t.Scopes}
				}
			}
		}
		return principal{}
	}
	if len(r.CookiesNamed(s.sessionCookieName())) != 1 {
		return principal{}
	}
	c, e := r.Cookie(s.sessionCookieName())
	if e != nil {
		return principal{}
	}
	hash := digest(c.Value)
	se, ok := s.st.Sessions[hash]
	if !ok || !se.ExpiresAt.After(now()) {
		return principal{}
	}
	u, ok := s.st.Users[se.UserID]
	if !ok || u.Disabled {
		return principal{}
	}
	return principal{&u.User, hash, "session", nil}
}
func (s *Server) require(r *http.Request, p principal, admin bool, scope string) error {
	if p.user == nil {
		return fail(401, "unauthorized", "Sign in to continue")
	}
	if p.user.MustChangePassword && r.URL.Path != "/api/auth/password" && r.URL.Path != "/api/auth/logout" && r.URL.Path != "/api/me" {
		return fail(403, "password_change_required", "Change your temporary password first")
	}
	if admin && (p.user.Role != "admin" || p.kind == "token") {
		return fail(403, "forbidden", "Administrator session required")
	}
	if p.kind == "token" && scope != "" && !contains(p.scopes, scope) {
		return fail(403, "insufficient_scope", "API token lacks required scope")
	}
	return nil
}
func (s *Server) csrf(r *http.Request, p principal) error {
	if p.kind == "token" {
		return nil
	}
	if p.user == nil {
		return nil
	}
	se := s.st.Sessions[p.credential]
	if r.Header.Get("Origin") != s.origin || r.Header.Get("X-CSRF-Token") != se.CSRF {
		return fail(403, "csrf_failed", "Request origin and CSRF token must match")
	}
	return nil
}
func (s *Server) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Cache-Control", "no-store")
		if strings.ToLower(r.Host) != s.appHost {
			errorJSON(w, fail(404, "not_found", "Not found"))
			return
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		if s.closed {
			errorJSON(w, fail(503, "closed", "Service is closed"))
			return
		}
		p := s.principal(r)
		if r.Method != "GET" && r.Method != "HEAD" && r.Method != "OPTIONS" {
			if e := s.csrf(r, p); e != nil {
				errorJSON(w, e)
				return
			}
		}
		if strings.HasPrefix(r.URL.Path, "/api/") {
			if e := s.route(w, r, p); e != nil {
				errorJSON(w, e)
			}
			return
		}
		s.web(w, r)
	})
}
func (s *Server) web(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" && r.Method != "HEAD" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; connect-src 'self'; frame-src http: https:; object-src 'none'; base-uri 'none'; frame-ancestors 'none'")
	if s.cfg.WebFS == nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, "<!doctype html><title>Folio</title><h1>Folio</h1><p>Frontend assets have not been built.</p>")
		return
	}
	name := strings.TrimPrefix(r.URL.Path, "/")
	if name == "" || strings.HasPrefix(name, "s/") {
		name = "index.html"
	}
	raw, e := fs.ReadFile(s.cfg.WebFS, name)
	if e != nil {
		if strings.Contains(filepath.Base(name), ".") {
			http.NotFound(w, r)
			return
		}
		raw, e = fs.ReadFile(s.cfg.WebFS, "index.html")
	}
	if e != nil {
		http.NotFound(w, r)
		return
	}
	if strings.HasSuffix(name, ".html") {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
	} else {
		w.Header().Set("Content-Type", mimeFor(name))
	}
	w.Write(raw)
}
func (s *Server) Maintenance(ctx context.Context, dryRun bool) (CleanupResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cleanup(ctx, dryRun, principal{})
}
