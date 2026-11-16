package server

import (
	"bytes"
	"encoding/json"
	"net"
	"net/http"
	"sort"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

func guestLinkPublic(g GuestLink, appURL string) map[string]any {
	return map[string]any{"id": g.ID, "artifactId": g.ArtifactID, "versionId": g.VersionID, "createdBy": g.CreatedBy, "createdAt": g.CreatedAt, "expiresAt": g.ExpiresAt, "revokedAt": g.RevokedAt, "passwordProtected": g.PasswordHash != "", "url": appURL + "/g/" + g.ID}
}

// guestArtifact exposes only the explicitly pinned version and no directory IDs.
func (s *Server) guestArtifact(g GuestLink) map[string]any {
	a := s.st.Artifacts[g.ArtifactID]
	size := int64(0)
	title, description := a.Title, a.Description
	for _, v := range s.st.Versions[a.ID] {
		if v.ID == g.VersionID {
			size = v.SizeBytes
			if v.MetadataSnapshot {
				title = v.Title
				description = v.Description
			}
		}
	}
	return map[string]any{"id": a.ID, "title": title, "description": description, "currentVersionId": g.VersionID, "sizeBytes": size}
}

func (s *Server) guestActive(g GuestLink) bool {
	a, ok := s.st.Artifacts[g.ArtifactID]
	if !ok || a.DeletedAt != nil || g.RevokedAt != nil || !g.ExpiresAt.After(now()) {
		return false
	}
	u, ok := s.st.Users[a.OwnerID]
	if !ok || u.Disabled || u.MustChangePassword {
		return false
	}
	for _, v := range s.st.Versions[a.ID] {
		if v.ID == g.VersionID {
			return true
		}
	}
	return false
}
func (s *Server) guestCookieName() string {
	if strings.HasPrefix(s.cfg.AppURL, "https:") {
		return "__Host-folio_guest"
	}
	return "folio_guest"
}
func (s *Server) guestPrincipal(r *http.Request) principal {
	if len(r.CookiesNamed(s.guestCookieName())) != 1 {
		return principal{}
	}
	c, e := r.Cookie(s.guestCookieName())
	if e != nil {
		return principal{}
	}
	return s.guestCredential(digest(c.Value))
}
func (s *Server) guestCredential(hash string) principal {
	se, ok := s.st.GuestSessions[hash]
	if !ok || !se.ExpiresAt.After(now()) {
		return principal{}
	}
	g, ok := s.st.GuestLinks[se.LinkID]
	if !ok || !s.guestActive(g) {
		return principal{}
	}
	return principal{kind: "guest", credential: hash, guestLinkID: g.ID}
}
func (s *Server) guestViewerPrincipal(v Viewer) (principal, bool) {
	if v.Kind != "guest" || !v.ExpiresAt.After(now()) {
		return principal{}, false
	}
	p := s.guestCredential(v.Credential)
	g, ok := s.st.GuestLinks[p.guestLinkID]
	return p, ok && g.ArtifactID == v.ArtifactID && g.VersionID == v.VersionID
}
func (s *Server) guestCanRead(p principal, a Artifact, versionID string) bool {
	if p.kind != "guest" || p.guestLinkID == "" {
		return false
	}
	active := s.guestCredential(p.credential)
	if active.guestLinkID != p.guestLinkID {
		return false
	}
	g, ok := s.st.GuestLinks[p.guestLinkID]
	return ok && s.guestActive(g) && g.ArtifactID == a.ID && g.VersionID == versionID && a.DeletedAt == nil
}
func (s *Server) guestLinks(w http.ResponseWriter, r *http.Request, p principal, a Artifact) error {
	if e := s.require(r, p, false, "share"); e != nil {
		return e
	}
	if !s.own(p, a) || a.DeletedAt != nil {
		return fail(404, "not_found", "Artifact not found")
	}
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/artifacts/"+a.ID+"/guest-links"), "/")
	if len(parts) == 1 && r.Method == "GET" {
		list := []GuestLink{}
		for _, g := range s.st.GuestLinks {
			if g.ArtifactID == a.ID {
				list = append(list, g)
			}
		}
		sort.Slice(list, func(i, j int) bool { return list[i].CreatedAt.After(list[j].CreatedAt) })
		out := []map[string]any{}
		for _, g := range list {
			out = append(out, guestLinkPublic(g, s.cfg.AppURL))
		}
		writeJSON(w, 200, map[string]any{"guestLinks": out})
		return nil
	}
	if len(parts) == 1 && r.Method == "POST" {
		var in struct {
			VersionID, Password string
			ExpiresAt           time.Time
		}
		if e := decode(r, &in); e != nil {
			return e
		}
		if in.VersionID == "" {
			return fail(400, "version_required", "Choose a specific version")
		}
		valid := false
		for _, v := range s.st.Versions[a.ID] {
			if v.ID == in.VersionID {
				valid = true
			}
		}
		if !valid {
			return fail(404, "not_found", "Version not found")
		}
		if !in.ExpiresAt.After(now()) || in.ExpiresAt.After(now().Add(365*24*time.Hour)) {
			return fail(400, "invalid_expiry", "Guest link expiry must be within one year")
		}
		g := GuestLink{ID: random(), ArtifactID: a.ID, VersionID: in.VersionID, CreatedBy: p.user.ID, CreatedAt: now(), ExpiresAt: in.ExpiresAt.UTC()}
		if in.Password != "" {
			hash, e := passwordHash(in.Password)
			if e != nil {
				return e
			}
			g.PasswordHash = hash
		}
		if e := s.mutate(func() error {
			if s.st.GuestLinks == nil {
				s.st.GuestLinks = map[string]GuestLink{}
			}
			s.st.GuestLinks[g.ID] = g
			s.audit(p, "guest_link.created", g.ID, a.ID)
			return nil
		}); e != nil {
			return e
		}
		writeJSON(w, 201, map[string]any{"guestLink": guestLinkPublic(g, s.cfg.AppURL)})
		return nil
	}
	if len(parts) == 3 && parts[2] == "revoke" && r.Method == "POST" {
		g, ok := s.st.GuestLinks[parts[1]]
		if !ok || g.ArtifactID != a.ID {
			return fail(404, "not_found", "Guest link not found")
		}
		at := now()
		g.RevokedAt = &at
		if e := s.mutate(func() error {
			s.st.GuestLinks[g.ID] = g
			for id, se := range s.st.GuestSessions {
				if se.LinkID == g.ID {
					delete(s.st.GuestSessions, id)
				}
			}
			s.audit(p, "guest_link.revoked", g.ID, a.ID)
			return nil
		}); e != nil {
			return e
		}
		w.WriteHeader(204)
		return nil
	}
	return fail(404, "not_found", "Guest link not found")
}
func (s *Server) guest(w http.ResponseWriter, r *http.Request) error {
	w.Header().Set("Cache-Control", "no-store")
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/guest/"), "/")
	if len(parts) < 1 {
		return fail(404, "not_found", "Guest link not found")
	}
	g, ok := s.st.GuestLinks[parts[0]]
	if !ok || !s.guestActive(g) {
		return fail(404, "not_found", "Guest link not found")
	}
	p := s.guestPrincipal(r)
	unlocked := p.guestLinkID == g.ID
	if len(parts) == 1 && r.Method == "GET" {
		out := map[string]any{"id": g.ID, "passwordProtected": g.PasswordHash != "", "unlocked": unlocked, "expiresAt": g.ExpiresAt}
		if unlocked {
			out["artifact"] = s.guestArtifact(g)
			out["versionId"] = g.VersionID
			out["csrfToken"] = s.st.GuestSessions[p.credential].CSRF
		}
		writeJSON(w, 200, out)
		return nil
	}
	if r.Header.Get("Origin") != s.origin {
		return fail(403, "invalid_origin", "Request origin must match")
	}
	if len(parts) == 2 && parts[1] == "unlock" && r.Method == "POST" {
		host, _, _ := net.SplitHostPort(r.RemoteAddr)
		key := "guest:" + g.ID + ":" + host
		globalKey := "guest-link:" + g.ID
		for _, k := range []string{key, globalKey} {
			a := s.loginAttempts[k]
			limit := 10
			if k == globalKey {
				limit = 50
			}
			if a.Until.After(now()) && a.Count >= limit {
				return fail(429, "rate_limited", "Too many attempts; try again later")
			}
		}
		var in struct{ Password string }
		if e := decode(r, &in); e != nil {
			return e
		}
		if g.PasswordHash != "" && (len(in.Password) > 72 || bcrypt.CompareHashAndPassword([]byte(g.PasswordHash), []byte(in.Password)) != nil) {
			for _, k := range []string{key, globalKey} {
				a := s.loginAttempts[k]
				if !a.Until.After(now()) {
					a = attempt{Until: now().Add(15 * time.Minute)}
				}
				a.Count++
				s.loginAttempts[k] = a
			}
			return fail(403, "invalid_password", "Guest link password is incorrect")
		}
		if unlocked {
			se := s.st.GuestSessions[p.credential]
			writeJSON(w, 200, map[string]any{"unlocked": true, "csrfToken": se.CSRF, "versionId": g.VersionID, "artifact": s.guestArtifact(g)})
			return nil
		}
		// Successful anonymous unlocks use the same budget as failed attempts.
		for _, k := range []string{key, globalKey} {
			a := s.loginAttempts[k]
			if !a.Until.After(now()) {
				a = attempt{Until: now().Add(15 * time.Minute)}
			}
			a.Count++
			s.loginAttempts[k] = a
		}
		active, forLink := 0, 0
		for _, se := range s.st.GuestSessions {
			if se.ExpiresAt.After(now()) {
				if link, ok := s.st.GuestLinks[se.LinkID]; ok && s.guestActive(link) {
					active++
					if se.LinkID == g.ID {
						forLink++
					}
				}
			}
		}
		if active >= 5000 || forLink >= 1000 {
			return fail(429, "rate_limited", "Too many active guest sessions; try again later")
		}
		plain := random()
		hash := digest(plain)
		until := now().Add(12 * time.Hour)
		if g.ExpiresAt.Before(until) {
			until = g.ExpiresAt
		}
		se := GuestSession{LinkID: g.ID, CSRF: random(), ExpiresAt: until}
		if e := s.mutate(func() error {
			if s.st.GuestSessions == nil {
				s.st.GuestSessions = map[string]GuestSession{}
			}
			s.pruneGuestSessions()
			s.st.GuestSessions[hash] = se
			return nil
		}); e != nil {
			return e
		}
		http.SetCookie(w, &http.Cookie{Name: s.guestCookieName(), Value: plain, Path: "/", HttpOnly: true, Secure: strings.HasPrefix(s.cfg.AppURL, "https:"), SameSite: http.SameSiteLaxMode, MaxAge: int(time.Until(until).Seconds())})
		writeJSON(w, 200, map[string]any{"csrfToken": se.CSRF, "versionId": g.VersionID, "artifact": s.guestArtifact(g)})
		return nil
	}
	if len(parts) == 2 && parts[1] == "launch" && r.Method == "POST" {
		if !unlocked || r.Header.Get("X-CSRF-Token") != s.st.GuestSessions[p.credential].CSRF {
			return fail(403, "csrf_failed", "Unlock this guest link and provide its CSRF token")
		}
		var in struct{ Hostname string }
		if e := decode(r, &in); e != nil {
			return e
		}
		raw, _ := json.Marshal(map[string]string{"versionId": g.VersionID, "hostname": in.Hostname})
		clone := r.Clone(r.Context())
		clone.Body = http.NoBody
		if len(raw) > 0 {
			clone.Body = ioNopCloser{bytes.NewReader(raw)}
		}
		return s.launch(w, clone, p, s.st.Artifacts[g.ArtifactID])
	}
	return fail(404, "not_found", "Guest link not found")
}

type ioNopCloser struct{ *bytes.Reader }

func (ioNopCloser) Close() error { return nil }

func (s *Server) pruneGuestSessions() {
	for key, se := range s.st.GuestSessions {
		g, ok := s.st.GuestLinks[se.LinkID]
		if !se.ExpiresAt.After(now()) || !ok || !s.guestActive(g) {
			delete(s.st.GuestSessions, key)
		}
	}
}
