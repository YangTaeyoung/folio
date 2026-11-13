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
func (s *Server) hostFor(a Artifact, host string) (*url.URL, bool) {
	u := s.canonical(a)
	if host == "" || strings.ToLower(host) == strings.ToLower(u.Host) {
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
	var in struct{ VersionID, Hostname string }
	if e := decode(r, &in); e != nil {
		return e
	}
	v, ok := s.version(a, in.VersionID)
	if !ok {
		return fail(404, "not_found", "Version not found")
	}
	host, ok := s.hostFor(a, in.Hostname)
	if !ok {
		return fail(400, "invalid_hostname", "Destination must be this artifact's canonical or verified custom hostname")
	}
	nonce := random()
	view := Viewer{ArtifactID: a.ID, VersionID: v.ID, Host: strings.ToLower(host.Host), ExpiresAt: now().Add(60 * time.Second)}
	if p.user != nil {
		view.UserID = p.user.ID
		view.Credential = p.credential
		view.Kind = p.kind
	}
	if e := s.mutate(func() error { s.st.Grants[digest(nonce)] = view; return nil }); e != nil {
		return e
	}
	writeJSON(w, 200, map[string]any{"url": host.String() + "/grant/" + nonce})
	return nil
}
func (s *Server) viewerPrincipal(v Viewer) (principal, bool) {
	if !v.ExpiresAt.After(now()) {
		return principal{}, false
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
func (s *Server) artifactForHost(host string) (Artifact, bool) {
	host = strings.ToLower(host)
	for _, a := range s.st.Artifacts {
		if u, ok := s.hostFor(a, host); ok && strings.EqualFold(u.Host, host) {
			return a, true
		}
	}
	return Artifact{}, false
}
func (s *Server) ContentHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
				if d.Hostname == strings.ToLower(r.Host) && r.URL.Path == "/.well-known/folio-challenge/"+d.Challenge {
					w.Header().Set("Content-Type", "text/plain; charset=utf-8")
					fmt.Fprint(w, "folio-ingress:"+d.ID+":"+d.Challenge)
					return
				}
			}
			http.NotFound(w, r)
			return
		}
		a, ok := s.artifactForHost(r.Host)
		if !ok {
			http.NotFound(w, r)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/grant/") {
			nonce := strings.TrimPrefix(r.URL.Path, "/grant/")
			v, ok := s.st.Grants[digest(nonce)]
			if !ok || v.Host != strings.ToLower(r.Host) || v.ArtifactID != a.ID {
				http.NotFound(w, r)
				return
			}
			p, active := s.viewerPrincipal(v)
			if !active || !s.read(p, a) {
				http.NotFound(w, r)
				return
			}
			if _, ok = s.version(a, v.VersionID); !ok {
				http.NotFound(w, r)
				return
			}
			plain := random()
			v.ExpiresAt = now().Add(12 * time.Hour)
			if e := s.mutate(func() error { delete(s.st.Grants, digest(nonce)); s.st.Viewers[digest(plain)] = v; return nil }); e != nil {
				errorJSON(w, e)
				return
			}
			secure := s.canonical(a).Scheme == "https" || !strings.EqualFold(r.Host, s.canonical(a).Host)
			sameSite := http.SameSiteLaxMode
			if secure {
				sameSite = http.SameSiteNoneMode
			}
			http.SetCookie(w, &http.Cookie{Name: viewerCookieName(secure), Value: plain, Path: "/", HttpOnly: true, Secure: secure, SameSite: sameSite, Partitioned: secure, MaxAge: 12 * 3600})
			http.Redirect(w, r, "/render/"+a.ID+"/"+v.VersionID+"/index.html", http.StatusSeeOther)
			return
		}
		p := principal{}
		viewer := Viewer{}
		validViewer := false
		secureHost := s.canonical(a).Scheme == "https" || !strings.EqualFold(r.Host, s.canonical(a).Host)
		if c, e := r.Cookie(viewerCookieName(secureHost)); e == nil {
			if v, exists := s.st.Viewers[digest(c.Value)]; exists && v.Host == strings.ToLower(r.Host) && v.ArtifactID == a.ID {
				p, validViewer = s.viewerPrincipal(v)
				viewer = v
			}
		}
		if !s.read(principal{}, a) {
			if !validViewer || !s.read(p, a) {
				if r.URL.Path == "/" {
					target := s.cfg.AppURL + "/s/" + a.Slug + "?domain=" + url.QueryEscape(r.Host)
					http.Redirect(w, r, target, 303)
				} else {
					http.NotFound(w, r)
				}
				return
			}
		}
		versionID := ""
		filePath := strings.TrimPrefix(r.URL.Path, "/")
		if strings.HasPrefix(r.URL.Path, "/render/") {
			parts := strings.SplitN(strings.TrimPrefix(r.URL.Path, "/render/"), "/", 3)
			if len(parts) != 3 || parts[0] != a.ID {
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
		if versionID == "" && validViewer {
			versionID = viewer.VersionID
		}
		v, exists := s.version(a, versionID)
		if !exists {
			http.NotFound(w, r)
			return
		}
		if validViewer && viewer.VersionID != v.ID && !s.read(principal{}, a) {
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
	})
}

func viewerCookieName(secure bool) string {
	if secure {
		return "__Host-folio_viewer"
	}
	return "folio_viewer"
}
