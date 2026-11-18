package server

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/url"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"
)

// AuditChange contains only explicitly selected non-secret fields, never a raw record.
type AuditChange struct {
	Field  string `json:"field"`
	Before any    `json:"before"`
	After  any    `json:"after"`
}

func auditCategory(action string) string {
	switch {
	case action == "login" || action == "login.google" || action == "logout" || action == "setup" || strings.HasPrefix(action, "password.") || strings.HasPrefix(action, "token.") || strings.HasPrefix(action, "auth.") || action == "member.password_reset":
		return "security"
	case action == "artifact.sharing" || strings.HasPrefix(action, "guest_link.") || action == "artifact.reviewers":
		return "sharing"
	case strings.HasPrefix(action, "member."):
		return "members"
	case strings.HasPrefix(action, "storage.") || strings.HasPrefix(action, "backup.") || strings.HasPrefix(action, "cleanup."):
		return "storage"
	case strings.HasPrefix(action, "domain.") || strings.HasPrefix(action, "dns."):
		return "domain"
	case strings.HasPrefix(action, "settings.") || strings.HasPrefix(action, "profile.") || strings.HasPrefix(action, "organization."):
		return "settings"
	default:
		return "content"
	}
}

// auditSnapshot is an allowlist. Adding fields requires an explicit privacy review.
// Descriptions, comment bodies, source URLs, credentials, hashes and raw configs stay out.
func auditSnapshot(st *state, a Audit) (string, string, map[string]any) {
	id := a.TargetID
	group := strings.SplitN(a.Action, ".", 2)[0]
	switch group {
	case "artifact", "review", "comment":
		v, ok := st.Artifacts[id]
		if !ok {
			return "artifact", "", nil
		}
		values := map[string]any{"title": v.Title, "visibility": v.Visibility, "members": auditMemberNames(st, v.MemberIDs), "archived": v.Archived, "reviewEnabled": v.ReviewEnabled, "reviewers": auditMemberNames(st, v.ReviewerIDs), "currentVersion": auditVersionNumber(st, id, v.CurrentVersionID), "publishedVersion": auditVersionNumber(st, id, v.PublishedVersionID), "tags": v.Tags, "deleted": v.DeletedAt != nil}
		if group == "review" {
			versionID := strings.SplitN(a.Detail, ":", 2)[0]
			if r, ok := st.Reviews[versionID]; ok {
				values["reviewStatus"] = r.Status
			}
		}
		if a.Action == "artifact.favorite" {
			values = map[string]any{"favorite": st.Favorites[a.ActorID][id]}
		}
		if group == "comment" {
			cid := strings.SplitN(a.Detail, ":", 2)[0]
			values = map[string]any{}
			if c, ok := st.Comments[cid]; ok {
				values["resolved"] = c.Resolved
				values["deleted"] = c.Deleted
			}
		}
		return "artifact", v.Title, values
	case "member", "profile", "password", "login", "logout", "setup":
		v, ok := st.Users[id]
		if !ok {
			return "member", "", nil
		}
		return "member", v.Name, map[string]any{"name": v.Name, "role": v.Role, "disabled": v.Disabled, "passwordChangeRequired": v.MustChangePassword, "photoConfigured": v.Photo != nil}
	case "collection":
		v, ok := st.Collections[id]
		if !ok {
			return "collection", "", nil
		}
		return "collection", v.Name, map[string]any{"name": v.Name, "visibility": v.Visibility, "artifacts": auditArtifactNames(st, v.ArtifactIDs)}
	case "storage":
		v, ok := st.Backends[id]
		if !ok {
			return "storage", "", nil
		}
		return "storage", v.Name, map[string]any{"name": v.Name, "provider": v.Provider, "isDefault": v.IsDefault}
	case "domain":
		v, ok := st.Domains[id]
		if !ok {
			return "domain", "", nil
		}
		return "domain", v.Hostname, map[string]any{"hostname": v.Hostname, "status": v.Status, "artifactId": v.ArtifactID}
	case "guest_link":
		v, ok := st.GuestLinks[id]
		if !ok {
			return "guest_link", "", nil
		}
		return "guest_link", st.Artifacts[v.ArtifactID].Title, map[string]any{"versionId": v.VersionID, "expiresAt": v.ExpiresAt, "revoked": v.RevokedAt != nil, "passwordProtected": v.PasswordHash != ""}
	case "token":
		v, ok := st.Tokens[id]
		if !ok {
			return "token", "", nil
		}
		return "token", v.Name, map[string]any{"name": v.Name, "scopes": v.Scopes, "expiresAt": v.ExpiresAt}
	case "settings", "organization":
		return "settings", st.Organization.Name, map[string]any{"name": st.Organization.Name, "quotaBytes": st.Policy.QuotaBytes, "maxUploadBytes": st.Policy.MaxUploadBytes, "retentionDays": st.Policy.RetentionDays, "overflowPolicy": st.Policy.OverflowPolicy, "photoConfigured": st.OrganizationPhoto != nil}
	case "auth":
		return "settings", st.Organization.Name, map[string]any{"googleEnabled": st.GoogleAuth.Enabled, "allowedDomains": st.GoogleAuth.AllowedDomains}
	}
	return group, "", nil
}
func auditVersionNumber(st *state, artifactID, versionID string) any {
	for _, v := range st.Versions[artifactID] {
		if v.ID == versionID {
			return v.Number
		}
	}
	return nil
}
func auditArtifactNames(st *state, ids []string) []string {
	out := []string{}
	for _, id := range ids {
		if a, ok := st.Artifacts[id]; ok {
			out = append(out, a.Title)
		} else {
			out = append(out, id)
		}
	}
	return out
}
func auditMemberNames(st *state, ids []string) []string {
	out := []string{}
	for _, id := range ids {
		if u, ok := st.Users[id]; ok {
			out = append(out, u.Name)
		} else {
			out = append(out, id)
		}
	}
	sort.Strings(out)
	return out
}

// enrichAudit runs inside mutate before the same SQLite commit as the actual change.
func (s *Server) enrichAudit(raw []byte, start int) {
	// Recovery replaces the state, including its historical audit list. Enrich
	// only the newly appended recovery event, never imported historical entries.
	last := len(s.st.Audit) - 1
	if start > last && (last < 0 || s.st.Audit[last].Action != "backup.restored") {
		return
	}
	var before state
	if json.Unmarshal(raw, &before) != nil {
		return
	}
	if last >= 0 && s.st.Audit[last].Action == "backup.restored" {
		oldLast := len(before.Audit) - 1
		if oldLast < 0 || before.Audit[oldLast].ID != s.st.Audit[last].ID {
			start = last
		}
	}
	if start > last {
		return
	}
	for i := start; i < len(s.st.Audit); i++ {
		a := &s.st.Audit[i]
		kind, name, after := auditSnapshot(&s.st, *a)
		oldKind, oldName, old := auditSnapshot(&before, *a)
		if kind == "" {
			kind = oldKind
		}
		if name == "" {
			name = oldName
		}
		a.Category = auditCategory(a.Action)
		a.TargetType = kind
		a.TargetName = name
		a.Snapshot = true
		keys := map[string]bool{}
		for k := range old {
			keys[k] = true
		}
		for k := range after {
			keys[k] = true
		}
		sorted := []string{}
		for k := range keys {
			sorted = append(sorted, k)
		}
		sort.Strings(sorted)
		for _, k := range sorted {
			if !reflect.DeepEqual(old[k], after[k]) {
				a.Changes = append(a.Changes, AuditChange{Field: k, Before: old[k], After: after[k]})
			}
		}
	}
}

// resolveAudit adds live navigation only; it never rewrites persisted historical names.
func (s *Server) resolveAudit(a Audit) Audit {
	if a.Category == "" {
		a.Category = auditCategory(a.Action)
	}
	if a.TargetType == "" || a.TargetName == "" {
		kind, name, _ := auditSnapshot(&s.st, a)
		if a.TargetType == "" {
			a.TargetType = kind
		}
		if a.TargetName == "" {
			a.TargetName = name
			a.TargetNameCurrent = name != ""
		}
	}
	id := a.TargetID
	if a.TargetType == "guest_link" {
		if g, ok := s.st.GuestLinks[id]; ok {
			id = g.ArtifactID
		}
	}
	if a.TargetType == "artifact" || a.TargetType == "guest_link" {
		if v, ok := s.st.Artifacts[id]; ok && v.DeletedAt == nil && v.Slug != "" {
			a.TargetURL = "/s/" + url.PathEscape(v.Slug)
		}
		return a
	}
	switch a.TargetType {
	case "member":
		a.TargetURL = "/#members"
	case "storage":
		a.TargetURL = "/#storage"
	case "domain":
		a.TargetURL = "/#domains"
	case "settings":
		a.TargetURL = "/#settings"
	case "collection":
		a.TargetURL = "/#collections"
	case "token":
		a.TargetURL = "/#account"
	}
	return a
}

type auditPage struct {
	Events     []Audit      `json:"events"`
	Total      int          `json:"total"`
	NextCursor string       `json:"nextCursor,omitempty"`
	Actors     []auditActor `json:"actors"`
}
type auditActor struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func (s *Server) auditEvents(w http.ResponseWriter, r *http.Request, p principal) error {
	if e := s.require(r, p, true, ""); e != nil {
		return e
	}
	q := r.URL.Query()
	limit := 100
	if v := q.Get("limit"); v != "" {
		fmtSscan(v, &limit)
	}
	if limit < 1 || limit > 1000 {
		limit = 100
	}
	// Legacy calls without filters still return the newest events of every category.
	category, actor, search := q.Get("category"), q.Get("actor"), strings.ToLower(strings.TrimSpace(q.Get("q")))
	allowed := map[string]bool{"": true, "all": true, "workspace": true, "content": true, "sharing": true, "members": true, "settings": true, "security": true, "storage": true, "domain": true}
	if !allowed[category] {
		return fail(400, "invalid_audit_filter", "Invalid activity category")
	}
	var from, to time.Time
	var e error
	if q.Get("from") != "" {
		from, e = time.Parse(time.RFC3339, q.Get("from"))
		if e != nil {
			return fail(400, "invalid_audit_filter", "from must be RFC3339")
		}
	}
	if q.Get("to") != "" {
		to, e = time.Parse(time.RFC3339, q.Get("to"))
		if e != nil {
			return fail(400, "invalid_audit_filter", "to must be RFC3339")
		}
	}
	if !from.IsZero() && !to.IsZero() && !from.Before(to) {
		return fail(400, "invalid_audit_filter", "from must precede to")
	}
	cursor := ""
	if q.Get("cursor") != "" {
		v, err := base64.RawURLEncoding.DecodeString(q.Get("cursor"))
		if err != nil || len(v) > 256 {
			return fail(400, "invalid_audit_filter", "Invalid activity cursor")
		}
		cursor = string(v)
	}
	out := auditPage{Events: []Audit{}, Actors: []auditActor{}}
	actors := map[string]string{}
	past := cursor == ""
	found := past
	for i := len(s.st.Audit) - 1; i >= 0; i-- {
		a := s.resolveAudit(s.st.Audit[i])
		actors[a.ActorID] = a.ActorName
		matches := (category == "" || category == "all" || category == "workspace" && a.Category != "security" || category == a.Category) && (actor == "" || actor == a.ActorID || actor == "__system__" && a.ActorID == "") && (from.IsZero() || !a.CreatedAt.Before(from)) && (to.IsZero() || a.CreatedAt.Before(to))
		if search != "" {
			raw, _ := json.Marshal(a.Changes)
			haystack := strings.ToLower(strings.Join([]string{a.ActorName, a.TargetName, a.TargetID, a.Action, string(raw)}, " "))
			matches = matches && strings.Contains(haystack, search)
		}
		if matches {
			out.Total++
		}
		if !past {
			if a.ID == cursor {
				past = true
				found = true
			}
			continue
		}
		if !matches {
			continue
		}
		if len(out.Events) < limit {
			out.Events = append(out.Events, a)
		} else if out.NextCursor == "" {
			out.NextCursor = base64.RawURLEncoding.EncodeToString([]byte(out.Events[len(out.Events)-1].ID))
		}
	}
	if !found {
		return fail(400, "invalid_audit_filter", "Activity cursor not found")
	}
	for id, name := range actors {
		out.Actors = append(out.Actors, auditActor{ID: id, Name: name})
	}
	sort.Slice(out.Actors, func(i, j int) bool {
		if out.Actors[i].Name == out.Actors[j].Name {
			return out.Actors[i].ID < out.Actors[j].ID
		}
		return out.Actors[i].Name < out.Actors[j].Name
	})
	// Explicit page metadata is additive and leaves the legacy events shape intact.
	w.Header().Set("X-Total-Count", strconv.Itoa(out.Total))
	writeJSON(w, 200, out)
	return nil
}
