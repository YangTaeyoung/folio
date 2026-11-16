package server

import (
	"crypto/subtle"
	"golang.org/x/crypto/bcrypt"
	"net"
	"net/http"
	"net/mail"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

func passwordHash(p string) (string, error) {
	if len(p) < 12 || len(p) > 72 {
		return "", fail(400, "weak_password", "Password must contain between 12 and 72 bytes")
	}
	v, e := bcrypt.GenerateFromPassword([]byte(p), bcrypt.DefaultCost)
	return string(v), e
}
func emailValue(v string) (string, error) {
	v = strings.ToLower(strings.TrimSpace(v))
	a, e := mail.ParseAddress(v)
	if e != nil || a.Address != v || len(v) > 254 {
		return "", fail(400, "invalid_email", "Enter a valid email address")
	}
	return v, nil
}
func (s *Server) startSession(w http.ResponseWriter, u User) (map[string]any, error) {
	token := random()
	csrf := random()
	s.st.Sessions[digest(token)] = Session{u.ID, csrf, now().Add(24 * time.Hour)}
	if e := s.save(); e != nil {
		delete(s.st.Sessions, digest(token))
		return nil, e
	}
	http.SetCookie(w, &http.Cookie{Name: s.sessionCookieName(), Value: token, Path: "/", HttpOnly: true, Secure: strings.HasPrefix(s.cfg.AppURL, "https:"), SameSite: http.SameSiteLaxMode, MaxAge: 86400})
	return map[string]any{"user": u, "csrfToken": csrf, "organization": s.st.Organization}, nil
}
func (s *Server) revoke(userID string) {
	for id, se := range s.st.Sessions {
		if se.UserID == userID {
			delete(s.st.Sessions, id)
		}
	}
	for id, t := range s.st.Tokens {
		if t.OwnerID == userID {
			delete(s.st.Tokens, id)
		}
	}
}
func (s *Server) route(w http.ResponseWriter, r *http.Request, p principal) error {
	path := r.URL.Path
	switch {
	case path == "/api/auth/providers" && r.Method == "GET":
		writeJSON(w, 200, map[string]any{"google": map[string]any{"enabled": s.st.GoogleAuth.Enabled, "startUrl": "/api/auth/google/start"}})
		return nil
	case path == "/api/auth/google/start" && r.Method == "GET":
		return s.googleStart(w, r)
	case path == "/api/auth/google/callback" && r.Method == "GET":
		return s.googleCallback(w, r)
	case path == "/api/admin/auth/google":
		return s.googleSettings(w, r, p)
	case path == "/api/bootstrap" && r.Method == "GET":
		writeJSON(w, 200, map[string]any{"needsSetup": len(s.st.Users) == 0, "version": s.cfg.Version})
		return nil
	case path == "/api/setup" && r.Method == "POST":
		if len(s.st.Users) > 0 {
			return fail(409, "already_configured", "Setup is already complete")
		}
		var in struct{ Token, OrganizationName, Name, Email, Password string }
		if e := decode(r, &in); e != nil {
			return e
		}
		if s.bootstrap == "" || subtle.ConstantTimeCompare([]byte(in.Token), []byte(s.bootstrap)) != 1 {
			return fail(403, "invalid_bootstrap_token", "Invalid setup token")
		}
		email, e := emailValue(in.Email)
		if e != nil {
			return e
		}
		hash, e := passwordHash(in.Password)
		if e != nil {
			return e
		}
		if strings.TrimSpace(in.Name) == "" || strings.TrimSpace(in.OrganizationName) == "" {
			return fail(400, "required_field", "Name and organization are required")
		}
		u := User{ID: random(), Name: in.Name, Email: email, Role: "admin", CreatedAt: now()}
		if e = s.mutate(func() error {
			s.st.Organization.Name = in.OrganizationName
			s.st.Users[u.ID] = userRecord{u, hash}
			s.audit(principal{user: &u}, "setup", u.ID, "")
			return nil
		}); e != nil {
			return e
		}
		os.Remove(filepath.Join(s.cfg.DataDir, "bootstrap.token"))
		s.bootstrap = ""
		out, e := s.startSession(w, u)
		if e != nil {
			return e
		}
		writeJSON(w, 200, out)
		return nil
	case path == "/api/auth/login" && r.Method == "POST":
		if origin := r.Header.Get("Origin"); origin != "" && origin != s.origin {
			return fail(403, "invalid_origin", "Invalid request origin")
		}
		var in struct{ Email, Password string }
		if e := decode(r, &in); e != nil {
			return e
		}
		key, _, splitErr := net.SplitHostPort(r.RemoteAddr)
		if splitErr != nil {
			key = r.RemoteAddr
		}
		a := s.loginAttempts[key]
		if a.Until.After(now()) && a.Count >= 10 {
			return fail(429, "rate_limited", "Too many login attempts; retry later")
		}
		if !a.Until.After(now()) {
			a = attempt{Until: now().Add(15 * time.Minute)}
		}
		a.Count++
		if len(s.loginAttempts) > 10000 {
			for k, v := range s.loginAttempts {
				if v.Until.Before(now()) {
					delete(s.loginAttempts, k)
				}
			}
		}
		s.loginAttempts[key] = a
		for _, u := range s.st.Users {
			if u.Email == strings.ToLower(strings.TrimSpace(in.Email)) && !u.Disabled && u.LoginMethod != "google" && bcrypt.CompareHashAndPassword([]byte(u.Hash), []byte(in.Password)) == nil {
				delete(s.loginAttempts, key)
				s.audit(principal{user: &u.User}, "login", u.ID, "")
				out, e := s.startSession(w, u.User)
				if e != nil {
					return e
				}
				writeJSON(w, 200, out)
				return nil
			}
		}
		return fail(401, "invalid_credentials", "Invalid email or password")
	case path == "/api/me" && r.Method == "GET":
		if e := s.require(r, p, false, ""); e != nil {
			return e
		}
		csrf := ""
		if p.kind == "session" {
			csrf = s.st.Sessions[p.credential].CSRF
		}
		writeJSON(w, 200, map[string]any{"user": p.user, "csrfToken": csrf, "organization": s.st.Organization})
		return nil
	case path == "/api/auth/logout" && r.Method == "POST":
		if e := s.require(r, p, false, ""); e != nil {
			return e
		}
		if e := s.mutate(func() error { delete(s.st.Sessions, p.credential); s.audit(p, "logout", p.user.ID, ""); return nil }); e != nil {
			return e
		}
		http.SetCookie(w, &http.Cookie{Name: s.sessionCookieName(), Path: "/", Value: "", MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: strings.HasPrefix(s.cfg.AppURL, "https:")})
		w.WriteHeader(204)
		return nil
	case path == "/api/auth/password" && r.Method == "POST":
		if e := s.require(r, p, false, ""); e != nil {
			return e
		}
		if p.kind != "session" {
			return fail(403, "forbidden", "Password changes require a session")
		}
		var in struct{ CurrentPassword, NewPassword string }
		if e := decode(r, &in); e != nil {
			return e
		}
		u := s.st.Users[p.user.ID]
		if u.LoginMethod == "google" {
			return fail(409, "google_only_account", "This account uses Google sign-in")
		}
		if bcrypt.CompareHashAndPassword([]byte(u.Hash), []byte(in.CurrentPassword)) != nil {
			return fail(403, "invalid_password", "Current password is incorrect")
		}
		hash, e := passwordHash(in.NewPassword)
		if e != nil {
			return e
		}
		if e = s.mutate(func() error {
			u.Hash = hash
			u.MustChangePassword = false
			s.st.Users[u.ID] = u
			s.revoke(u.ID)
			s.audit(p, "password.changed", u.ID, "")
			return nil
		}); e != nil {
			return e
		}
		out, e := s.startSession(w, u.User)
		if e != nil {
			return e
		}
		writeJSON(w, 200, out)
		return nil
	case path == "/api/capabilities" && r.Method == "GET":
		writeJSON(w, 200, map[string]any{"version": s.cfg.Version, "offline": true, "features": []string{"reviews", "drafts", "collections", "version-diff", "body-search", "provenance", "trash", "portable-backup", "guest-links", "i18n"}, "locales": []string{"ko", "en", "ja", "zh-CN", "fr"}, "tokenScopes": []string{"read", "publish", "share", "review"}, "maxUploadBytes": s.st.Policy.MaxUploadBytes, "maxFiles": 2500, "shareModes": []string{"private", "organization", "selected", "public"}, "apiUrl": s.cfg.AppURL + "/api", "cli": map[string]string{"command": "folio", "mcp": "folio mcp", "skill": "folio skills install"}})
		return nil
	case path == "/api/members" && r.Method == "GET":
		if e := s.require(r, p, false, "read"); e != nil {
			return e
		}
		members := []User{}
		for _, u := range s.st.Users {
			if !u.Disabled || p.user.Role == "admin" {
				members = append(members, u.User)
			}
		}
		sort.Slice(members, func(i, j int) bool { return members[i].Name < members[j].Name })
		writeJSON(w, 200, map[string]any{"members": members})
		return nil
	case strings.HasPrefix(path, "/api/admin/members"):
		return s.members(w, r, p)
	case path == "/api/admin/settings":
		return s.adminSettings(w, r, p)
	case path == "/api/admin/audit" && r.Method == "GET":
		if e := s.require(r, p, true, ""); e != nil {
			return e
		}
		limit := 100
		if v := r.URL.Query().Get("limit"); v != "" {
			fmtSscan(v, &limit)
		}
		if limit < 1 || limit > 1000 {
			limit = 100
		}
		events := []Audit{}
		for i := len(s.st.Audit) - 1; i >= 0 && len(events) < limit; i-- {
			events = append(events, s.st.Audit[i])
		}
		writeJSON(w, 200, map[string]any{"events": events})
		return nil
	case path == "/api/admin/cleanup" && r.Method == "POST":
		if e := s.require(r, p, true, ""); e != nil {
			return e
		}
		var in struct {
			DryRun *bool `json:"dryRun"`
		}
		if e := decode(r, &in); e != nil {
			return e
		}
		if in.DryRun == nil {
			return fail(400, "required_field", "dryRun is required")
		}
		out, e := s.cleanup(r.Context(), *in.DryRun, p)
		if e != nil {
			return e
		}
		writeJSON(w, 200, out)
		return nil
	case path == "/api/collections" || strings.HasPrefix(path, "/api/collections/"):
		return s.collections(w, r, p)
	case path == "/api/trash" || strings.HasPrefix(path, "/api/trash/"):
		return s.lifecycle(w, r, p)
	case path == "/api/backups" || strings.HasPrefix(path, "/api/backups/"):
		return s.backups(w, r, p)
	case strings.HasPrefix(path, "/api/guest/"):
		return s.guest(w, r)
	case path == "/api/tokens" || strings.HasPrefix(path, "/api/tokens/"):
		return s.tokens(w, r, p)
	case path == "/api/storage" || strings.HasPrefix(path, "/api/storage/"):
		return s.storage(w, r, p)
	case path == "/api/artifacts" || strings.HasPrefix(path, "/api/artifacts/") || strings.HasPrefix(path, "/api/share/"):
		return s.artifacts(w, r, p)
	case strings.HasPrefix(path, "/api/domains") || strings.HasPrefix(path, "/api/admin/dns/") || path == "/api/dns/domainconnect/callback":
		return s.domains(w, r, p)
	}
	return fail(404, "not_found", "Not found")
}
func (s *Server) members(w http.ResponseWriter, r *http.Request, p principal) error {
	if e := s.require(r, p, true, ""); e != nil {
		return e
	}
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/admin/members"), "/")
	id := ""
	if len(parts) > 1 {
		id = parts[1]
	}
	if id == "" && r.Method == "POST" {
		var in struct{ Name, Email, Role, Password, LoginMethod string }
		if e := decode(r, &in); e != nil {
			return e
		}
		email, e := emailValue(in.Email)
		if e != nil {
			return e
		}
		for _, u := range s.st.Users {
			if u.Email == email {
				return fail(409, "email_exists", "This email is already a member")
			}
		}
		if strings.TrimSpace(in.Name) == "" {
			return fail(400, "required_field", "Name is required")
		}
		if in.Role == "" {
			in.Role = "member"
		}
		if in.Role != "admin" && in.Role != "member" {
			return fail(400, "invalid_role", "Invalid role")
		}
		if in.LoginMethod != "" && in.LoginMethod != "password" && in.LoginMethod != "google" {
			return fail(400, "invalid_login_method", "Unknown login method")
		}
		googleOnly := in.LoginMethod == "google"
		if googleOnly && (!s.st.GoogleAuth.Enabled || in.Password != "") {
			return fail(400, "google_account_unavailable", "Enable Google sign-in and omit the password")
		}
		temp := ""
		if !googleOnly && in.Password == "" {
			temp = random()[:24]
			in.Password = temp
		}
		hash := ""
		if !googleOnly {
			hash, e = passwordHash(in.Password)
			if e != nil {
				return e
			}
		}
		u := User{ID: random(), Name: in.Name, Email: email, Role: in.Role, MustChangePassword: !googleOnly, CreatedAt: now()}
		if googleOnly {
			u.LoginMethod = "google"
		}
		if e = s.mutate(func() error {
			s.st.Users[u.ID] = userRecord{u, hash}
			s.audit(p, "member.created", u.ID, "")
			return nil
		}); e != nil {
			return e
		}
		out := map[string]any{"user": u}
		if temp != "" {
			out["temporaryPassword"] = temp
		}
		writeJSON(w, 201, out)
		return nil
	}
	u, ok := s.st.Users[id]
	if !ok {
		return fail(404, "not_found", "Member not found")
	}
	if len(parts) == 3 && parts[2] == "reset-password" && r.Method == "POST" {
		if u.LoginMethod == "google" {
			return fail(409, "google_only_account", "This account uses Google sign-in")
		}
		if id == p.user.ID {
			return fail(400, "use_password_change", "Use the account password-change form to change your own password")
		}
		temp := random()[:24]
		hash, e := passwordHash(temp)
		if e != nil {
			return e
		}
		if e = s.mutate(func() error {
			u.Hash = hash
			u.MustChangePassword = true
			s.st.Users[id] = u
			s.revoke(id)
			s.audit(p, "member.password_reset", id, "")
			return nil
		}); e != nil {
			return e
		}
		writeJSON(w, 200, map[string]any{"temporaryPassword": temp})
		return nil
	}
	if len(parts) == 2 && r.Method == "PATCH" {
		var in struct {
			Role     *string
			Disabled *bool
			Name     *string
		}
		if e := decode(r, &in); e != nil {
			return e
		}
		if in.Role != nil {
			if *in.Role != "admin" && *in.Role != "member" {
				return fail(400, "invalid_role", "Invalid role")
			}
			u.Role = *in.Role
		}
		if in.Disabled != nil {
			u.Disabled = *in.Disabled
		}
		if in.Name != nil {
			if strings.TrimSpace(*in.Name) == "" {
				return fail(400, "required_field", "Name is required")
			}
			u.Name = *in.Name
		}
		admins := 0
		for uid, v := range s.st.Users {
			if uid != id && v.Role == "admin" && !v.Disabled && (v.LoginMethod != "google" || s.googleAccountEligible(v.User, s.st.GoogleAuth)) {
				admins++
			}
		}
		if (u.Role != "admin" || u.Disabled) && admins == 0 {
			return fail(409, "last_admin", "The last active administrator cannot be disabled or demoted")
		}
		if e := s.mutate(func() error { s.st.Users[id] = u; s.revoke(id); s.audit(p, "member.updated", id, ""); return nil }); e != nil {
			return e
		}
		writeJSON(w, 200, map[string]any{"user": u.User})
		return nil
	}
	return fail(404, "not_found", "Not found")
}
func (s *Server) adminSettings(w http.ResponseWriter, r *http.Request, p principal) error {
	if e := s.require(r, p, true, ""); e != nil {
		return e
	}
	if r.Method == "GET" {
		out := s.settings()
		var pending int64
		for _, g := range s.st.Garbage {
			pending += g.Size
		}
		out["usage"] = map[string]int64{"usedBytes": s.used(), "quotaBytes": s.st.Policy.QuotaBytes, "pendingDeletionBytes": pending}
		writeJSON(w, 200, out)
		return nil
	}
	if r.Method != "PATCH" {
		return fail(405, "method_not_allowed", "Method not allowed")
	}
	var in struct {
		OrganizationName *string
		Policy           *Policy
	}
	if e := decode(r, &in); e != nil {
		return e
	}
	if in.Policy != nil {
		v := in.Policy
		if v.QuotaBytes < 1 || v.QuotaBytes > 1<<60 || v.MaxUploadBytes < 1 || v.MaxUploadBytes > 1<<40 || v.RetentionDays < 0 || v.RetentionDays > 36500 || (v.OverflowPolicy != "reject" && v.OverflowPolicy != "delete_oldest") {
			return fail(400, "invalid_policy", "Invalid storage policy")
		}
	}
	if in.OrganizationName != nil && strings.TrimSpace(*in.OrganizationName) == "" {
		return fail(400, "required_field", "Organization name is required")
	}
	if e := s.mutate(func() error {
		if in.Policy != nil {
			s.st.Policy = *in.Policy
		}
		if in.OrganizationName != nil {
			s.st.Organization.Name = *in.OrganizationName
		}
		s.audit(p, "settings.updated", s.st.Organization.ID, "")
		return nil
	}); e != nil {
		return e
	}
	writeJSON(w, 200, s.settings())
	return nil
}
func (s *Server) tokens(w http.ResponseWriter, r *http.Request, p principal) error {
	if e := s.require(r, p, false, ""); e != nil {
		return e
	}
	if p.kind != "session" {
		return fail(403, "forbidden", "Token management requires a session")
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/tokens")
	if id == "" && r.Method == "GET" {
		list := []Token{}
		for _, v := range s.st.Tokens {
			if v.OwnerID == p.user.ID {
				list = append(list, v.Token)
			}
		}
		sort.Slice(list, func(i, j int) bool { return list[i].CreatedAt.Before(list[j].CreatedAt) })
		writeJSON(w, 200, map[string]any{"tokens": list})
		return nil
	}
	if id == "" && r.Method == "POST" {
		var in struct {
			Name      string
			Scopes    []string
			ExpiresAt *time.Time
		}
		if e := decode(r, &in); e != nil {
			return e
		}
		if strings.TrimSpace(in.Name) == "" || len(in.Scopes) == 0 {
			return fail(400, "required_field", "Name and scopes are required")
		}
		for _, v := range in.Scopes {
			if v != "read" && v != "publish" && v != "share" && v != "review" {
				return fail(400, "invalid_scope", "Unknown token scope")
			}
		}
		if in.ExpiresAt != nil && !in.ExpiresAt.After(now()) {
			return fail(400, "invalid_expiry", "Expiry must be in the future")
		}
		plain := "folio_" + random()
		t := Token{ID: random(), Name: in.Name, CreatedAt: now(), Scopes: in.Scopes, ExpiresAt: in.ExpiresAt}
		if e := s.mutate(func() error {
			s.st.Tokens[t.ID] = tokenRecord{Token: t, OwnerID: p.user.ID, Digest: digest(plain)}
			s.audit(p, "token.created", t.ID, "")
			return nil
		}); e != nil {
			return e
		}
		writeJSON(w, 201, map[string]any{"token": plain, "record": t})
		return nil
	}
	if strings.HasPrefix(id, "/") && r.Method == "DELETE" {
		id = strings.TrimPrefix(id, "/")
		t, ok := s.st.Tokens[id]
		if !ok || t.OwnerID != p.user.ID {
			return fail(404, "not_found", "Token not found")
		}
		if e := s.mutate(func() error { delete(s.st.Tokens, id); s.audit(p, "token.revoked", id, ""); return nil }); e != nil {
			return e
		}
		w.WriteHeader(204)
		return nil
	}
	return fail(404, "not_found", "Not found")
}
