package server

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func (s *Server) canonical(a Artifact) *url.URL {
	u, _ := url.Parse(strings.ReplaceAll(s.cfg.ContentURL, "{artifact}", a.Slug))
	return u
}

// canonicalVersion isolates browser-origin state as well as root-absolute URLs.
func (s *Server) canonicalVersion(a Artifact, versionID string) *url.URL {
	u, _ := url.Parse(strings.ReplaceAll(s.cfg.ContentURL, "{artifact}", "v--"+digest(a.ID + "/" + versionID)[:32]))
	return u
}

// siteKey identifies one content site: its lowercase host plus the path prefix that addresses
// the artifact on that host, if any.
func siteKey(u *url.URL) string {
	return strings.ToLower(u.Host) + strings.TrimRight(u.Path, "/")
}
func (s *Server) versionForSite(a Artifact, key string) string {
	for _, v := range s.st.Versions[a.ID] {
		if siteKey(s.canonicalVersion(a, v.ID)) == key {
			return v.ID
		}
	}
	return ""
}
func (s *Server) hostFor(a Artifact, host string) (*url.URL, bool) {
	u := s.canonical(a)
	if host == "" || strings.ToLower(host) == strings.ToLower(u.Host) {
		return u, true
	}
	if s.versionForSite(a, strings.ToLower(host)) != "" {
		u, _ := url.Parse(strings.ReplaceAll(s.cfg.ContentURL, "{artifact}", "sample"))
		u.Host = strings.ToLower(host)
		return u, true
	}
	if strings.ContainsAny(host, "/\\@?# ") || strings.Contains(host, ":") {
		return nil, false
	}
	for _, d := range s.st.Domains {
		if d.ArtifactID == a.ID && d.Status == "verified" && d.Hostname == strings.ToLower(host) {
			return &url.URL{Scheme: "https", Host: d.Hostname}, true
		}
	}
	return nil, false
}
func (s *Server) launch(w http.ResponseWriter, r *http.Request, p principal, a Artifact) error {
	var in struct {
		VersionID, Hostname string
		IsolatedVersion     bool `json:"isolatedVersion"`
	}
	if e := decode(r, &in); e != nil {
		return e
	}
	if in.VersionID == "" {
		in.VersionID = s.effectiveVersionID(p, a)
	}
	v, ok := s.version(a, in.VersionID)
	if !ok || !s.readVersion(p, a, v) {
		return fail(404, "not_found", "Version not found")
	}
	host, ok := s.hostFor(a, in.Hostname)
	if in.IsolatedVersion {
		if in.Hostname != "" {
			return fail(400, "invalid_hostname", "Version isolation uses the configured content origin")
		}
		host = s.canonicalVersion(a, v.ID)
		ok = true
	}
	if !ok || host == nil {
		return fail(400, "invalid_hostname", "Destination must be this artifact's canonical or verified custom hostname")
	}
	if bound := s.versionForSite(a, siteKey(host)); bound != "" && bound != v.ID {
		return fail(400, "invalid_hostname", "Hostname belongs to a different version")
	}
	nonce := random()
	view := Viewer{ArtifactID: a.ID, VersionID: v.ID, Host: siteKey(host), ExpiresAt: now().Add(60 * time.Second)}
	if p.kind == "guest" {
		view.Kind = "guest"
		view.Credential = p.credential
	}
	if p.user != nil {
		view.UserID = p.user.ID
		view.Credential = p.credential
		view.Kind = p.kind
	}
	if e := s.mutate(func() error { s.st.Grants[digest(nonce)] = view; return nil }); e != nil {
		return e
	}
	writeJSON(w, 200, map[string]any{"url": host.String() + "/grant/" + nonce, "hostname": host.Host, "isolatedVersion": in.IsolatedVersion})
	return nil
}
func (s *Server) viewerPrincipal(v Viewer) (principal, bool) {
	if !v.ExpiresAt.After(now()) {
		return principal{}, false
	}
	if v.Kind == "guest" {
		return s.guestViewerPrincipal(v)
	}
	if v.UserID == "" {
		return principal{}, true
	}
	u, ok := s.st.Users[v.UserID]
	if !ok || u.Disabled || u.MustChangePassword {
		return principal{}, false
	}
	if v.Kind == "session" {
		se, ok := s.st.Sessions[v.Credential]
		if !ok || se.UserID != v.UserID || !se.ExpiresAt.After(now()) {
			return principal{}, false
		}
		return principal{user: &u.User, credential: v.Credential, kind: "session"}, true
	}
	if v.Kind == "token" {
		for _, t := range s.st.Tokens {
			if t.OwnerID == v.UserID && t.Digest == v.Credential && (t.ExpiresAt == nil || t.ExpiresAt.After(now())) && contains(t.Scopes, "read") {
				return principal{user: &u.User, credential: v.Credential, kind: "token", scopes: t.Scopes}, true
			}
		}
	}
	return principal{}, false
}
func (s *Server) artifactForSite(key string) (Artifact, bool) {
	key = strings.ToLower(key)
	for _, a := range s.st.Artifacts {
		if siteKey(s.canonical(a)) == key || s.versionForSite(a, key) != "" {
			return a, true
		}
		for _, d := range s.st.Domains {
			if d.ArtifactID == a.ID && d.Status == "verified" && d.Hostname == key {
				return a, true
			}
		}
	}
	return Artifact{}, false
}

// ContentHandler serves artifact content addressed by request host.
func (s *Server) ContentHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.serveContent(w, r, strings.ToLower(r.Host), "")
	})
}

// serveContent serves the content site identified by key. prefix is the URL path that
// addresses the site on its host; r.URL.Path is relative to it.
func (s *Server) serveContent(w http.ResponseWriter, r *http.Request, key, prefix string) {
	cookiePath := "/"
	if prefix != "" {
		cookiePath = prefix + "/"
	}
	pathScoped := prefix != ""
	s.mu.Lock()
	defer s.mu.Unlock()
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Content-Security-Policy", "sandbox allow-scripts allow-forms allow-downloads allow-same-origin; default-src 'self' data: blob:; script-src 'self' 'unsafe-inline' 'unsafe-eval' blob:; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob: https: http:; connect-src 'self'; worker-src 'self' blob:; object-src 'none'; base-uri 'self'")
	w.Header().Set("Cache-Control", "no-store")
	if r.Method != "GET" && r.Method != "HEAD" {
		http.NotFound(w, r)
		return
	}
	if s.closed {
		http.NotFound(w, r)
		return
	}
	// The Service Workers update algorithm mandates this header for the
	// registration script fetch. Persisted interception would bypass future
	// network authorization; ordinary Worker fetches remain allowed.
	// https://www.w3.org/TR/service-workers/#service-worker-script-request
	if strings.EqualFold(strings.TrimSpace(r.Header.Get("Service-Worker")), "script") {
		http.Error(w, "Service worker registration is disabled for artifact content", http.StatusForbidden)
		return
	}
	if strings.HasPrefix(r.URL.Path, "/.well-known/folio-challenge/") {
		for _, d := range s.st.Domains {
			if d.Hostname == key && r.URL.Path == "/.well-known/folio-challenge/"+d.Challenge {
				w.Header().Set("Content-Type", "text/plain; charset=utf-8")
				fmt.Fprint(w, "folio-ingress:"+d.ID+":"+d.Challenge)
				return
			}
		}
		http.NotFound(w, r)
		return
	}
	a, ok := s.artifactForSite(key)
	if !ok {
		http.NotFound(w, r)
		return
	}
	boundVersion := s.versionForSite(a, key)
	if strings.HasPrefix(r.URL.Path, "/grant/") {
		nonce := strings.TrimPrefix(r.URL.Path, "/grant/")
		v, ok := s.st.Grants[digest(nonce)]
		if !ok || v.Host != key || v.ArtifactID != a.ID || (boundVersion != "" && boundVersion != v.VersionID) {
			http.NotFound(w, r)
			return
		}
		if v.ViewerDigest != "" {
			// A consumed grant may only redirect the exact browser credential
			// that already exchanged it. No new session or lifetime is issued.
			secure := s.contentSecure(a, key)
			matched := false
			for _, name := range []string{viewerVersionCookieName(secure, pathScoped, v.VersionID), viewerCookieName(secure, pathScoped)} {
				if len(r.CookiesNamed(name)) != 1 {
					continue
				}
				if c, e := r.Cookie(name); e == nil && digest(c.Value) == v.ViewerDigest {
					matched = true
				}
			}
			current, exists := s.st.Viewers[v.ViewerDigest]
			pp, active := s.viewerPrincipal(current)
			ver, vok := s.version(a, v.VersionID)
			if !matched || !exists || current.Host != v.Host || current.ArtifactID != a.ID || current.VersionID != v.VersionID || !active || !vok || !s.readVersion(pp, a, ver) {
				http.NotFound(w, r)
				return
			}
			http.Redirect(w, r, prefix+"/render/"+a.ID+"/"+v.VersionID+"/index.html", http.StatusSeeOther)
			return
		}
		p, active := s.viewerPrincipal(v)
		if !active || !s.read(p, a) {
			http.NotFound(w, r)
			return
		}
		if ver, exists := s.version(a, v.VersionID); !exists || !s.readVersion(p, a, ver) {
			http.NotFound(w, r)
			return
		}
		plain := random()
		v.ExpiresAt = now().Add(12 * time.Hour)
		if e := s.mutate(func() error {
			s.st.Viewers[digest(plain)] = v
			receipt := v
			receipt.ViewerDigest = digest(plain)
			s.st.Grants[digest(nonce)] = receipt
			return nil
		}); e != nil {
			errorJSON(w, e)
			return
		}
		secure := s.contentSecure(a, key)
		sameSite := http.SameSiteLaxMode
		if secure {
			sameSite = http.SameSiteNoneMode
		}
		http.SetCookie(w, &http.Cookie{Name: viewerCookieName(secure, pathScoped), Value: plain, Path: cookiePath, HttpOnly: true, Secure: secure, SameSite: sameSite, Partitioned: secure, MaxAge: 12 * 3600})
		http.SetCookie(w, &http.Cookie{Name: viewerVersionCookieName(secure, pathScoped, v.VersionID), Value: plain, Path: cookiePath, HttpOnly: true, Secure: secure, SameSite: sameSite, Partitioned: secure, MaxAge: 12 * 3600})
		http.Redirect(w, r, prefix+"/render/"+a.ID+"/"+v.VersionID+"/index.html", http.StatusSeeOther)
		return
	}
	p := principal{}
	viewer := Viewer{}
	validViewer := false
	secureHost := s.contentSecure(a, key)
	cookieNames := []string{}
	if strings.HasPrefix(r.URL.Path, "/render/") {
		parts := strings.SplitN(strings.TrimPrefix(r.URL.Path, "/render/"), "/", 3)
		if len(parts) == 3 && parts[0] == a.ID {
			cookieNames = append(cookieNames, viewerVersionCookieName(secureHost, pathScoped, parts[1]))
		}
	}
	cookieNames = append(cookieNames, viewerCookieName(secureHost, pathScoped))
	for _, name := range cookieNames {
		if len(r.CookiesNamed(name)) != 1 {
			continue
		}
		if c, e := r.Cookie(name); e == nil {
			if v, exists := s.st.Viewers[digest(c.Value)]; exists && v.Host == key && v.ArtifactID == a.ID {
				p, validViewer = s.viewerPrincipal(v)
				if validViewer {
					viewer = v
					break
				}
			}
		}
	}

	if !s.read(principal{}, a) {
		if !validViewer || !s.read(p, a) {
			if r.URL.Path == "/" {
				target := s.cfg.AppURL + "/s/" + a.Slug
				if prefix == "" {
					target += "?domain=" + url.QueryEscape(r.Host)
				}
				http.Redirect(w, r, target, 303)
			} else {
				http.NotFound(w, r)
			}
			return
		}
	}
	versionID := boundVersion
	filePath := strings.TrimPrefix(r.URL.Path, "/")
	if strings.HasPrefix(r.URL.Path, "/render/") {
		parts := strings.SplitN(strings.TrimPrefix(r.URL.Path, "/render/"), "/", 3)
		if len(parts) != 3 || parts[0] != a.ID {
			http.NotFound(w, r)
			return
		}
		if boundVersion != "" && parts[1] != boundVersion {
			http.NotFound(w, r)
			return
		}
		versionID = parts[1]
		filePath = parts[2]
	}
	if filePath == "" {
		filePath = "index.html"
	}
	if strings.HasSuffix(filePath, "/") {
		filePath += "index.html"
	}
	if !safePath(filePath) {
		http.NotFound(w, r)
		return
	}
	if versionID == "" {
		if validViewer {
			versionID = viewer.VersionID
		} else {
			versionID = s.effectiveVersionID(principal{}, a)
		}
	}
	v, exists := s.version(a, versionID)
	if !exists || (!s.readVersion(principal{}, a, v) && (!validViewer || !s.readVersion(p, a, v))) {
		http.NotFound(w, r)
		return
	}
	if validViewer && viewer.VersionID != v.ID && !s.readVersion(principal{}, a, v) {
		http.NotFound(w, r)
		return
	}
	var file *File
	for _, f := range v.Files {
		if f.Path == filePath {
			copy := f
			file = &copy
			break
		}
	}
	if file == nil {
		http.NotFound(w, r)
		return
	}
	store, e := s.store(v.BackendID)
	if e != nil {
		errorJSON(w, fail(502, "storage_failed", "Content is unavailable"))
		return
	}
	rd, e := store.Open(r.Context(), file.Key)
	if e != nil {
		errorJSON(w, fail(502, "storage_failed", "Content is unavailable"))
		return
	}
	defer rd.Close()
	w.Header().Set("Content-Type", file.MIMEType)
	w.Header().Set("Content-Length", fmt.Sprint(file.Size))
	if r.Method == "GET" {
		io.Copy(w, rd)
	}
}

func viewerCookieName(secure, pathScoped bool) string {
	switch {
	case secure && pathScoped:
		return "__Secure-folio_viewer"
	case secure:
		return "__Host-folio_viewer"
	}
	return "folio_viewer"
}

func viewerVersionCookieName(secure, pathScoped bool, versionID string) string {
	return viewerCookieName(secure, pathScoped) + "_v" + digest(versionID)[:16]
}

func (s *Server) contentSecure(a Artifact, key string) bool {
	if v := s.versionForSite(a, key); v != "" {
		return s.canonicalVersion(a, v).Scheme == "https"
	}
	return s.canonical(a).Scheme == "https" || key != siteKey(s.canonical(a))
}
