package server

import (
	"context"
	"net/http"
	"sort"
	"strings"
	"time"
)

const trashRetention = 30 * 24 * time.Hour

func (s *Server) trash(w http.ResponseWriter, r *http.Request, p principal, a Artifact) error {
	if e := s.require(r, p, false, "publish"); e != nil {
		return e
	}
	if !s.own(p, a) {
		return fail(403, "forbidden", "Only the owner or an administrator can delete this artifact")
	}
	if s.inFlight[a.ID] != 0 {
		return fail(409, "artifact_busy", "An upload is using this artifact")
	}
	if a.DeletedAt != nil {
		return fail(409, "already_deleted", "Artifact is already in trash")
	}
	deleted := now()
	a.DeletedAt = &deleted
	a.DeletedBy = p.user.ID
	if e := s.mutate(func() error {
		s.st.Artifacts[a.ID] = a
		s.invalidateArtifactGuests(a.ID)
		s.audit(p, "artifact.trashed", a.ID, "")
		return nil
	}); e != nil {
		return e
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (s *Server) lifecycle(w http.ResponseWriter, r *http.Request, p principal) error {
	scope := "publish"
	if r.Method == "GET" {
		scope = "read"
	}
	if e := s.require(r, p, false, scope); e != nil {
		return e
	}
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/trash"), "/")
	if len(parts) == 1 && r.Method == "GET" {
		list := []Artifact{}
		for _, a := range s.st.Artifacts {
			if a.DeletedAt != nil && s.own(p, a) {
				list = append(list, s.publicArtifact(a))
			}
		}
		sort.Slice(list, func(i, j int) bool { return list[i].DeletedAt.After(*list[j].DeletedAt) })
		writeJSON(w, 200, map[string]any{"artifacts": list, "retentionDays": 30})
		return nil
	}
	if len(parts) < 2 {
		return fail(404, "not_found", "Trash item not found")
	}
	a, ok := s.st.Artifacts[parts[1]]
	if !ok || a.DeletedAt == nil || !s.own(p, a) {
		return fail(404, "not_found", "Trash item not found")
	}
	if s.inFlight[a.ID] != 0 {
		return fail(409, "artifact_busy", "An upload is using this artifact")
	}
	if len(parts) == 3 && parts[2] == "restore" && r.Method == "POST" {
		a.DeletedAt = nil
		a.DeletedBy = ""
		a.UpdatedAt = now()
		if e := s.mutate(func() error { s.st.Artifacts[a.ID] = a; s.audit(p, "artifact.restored", a.ID, ""); return nil }); e != nil {
			return e
		}
		writeJSON(w, 200, map[string]any{"artifact": s.publicArtifact(a)})
		return nil
	}
	if len(parts) == 2 && r.Method == "DELETE" {
		if e := s.mutate(func() error { s.tombstone(a.ID); s.audit(p, "artifact.purged", a.ID, ""); return nil }); e != nil {
			return e
		}
		s.gc(r.Context())
		w.WriteHeader(204)
		return nil
	}
	return fail(404, "not_found", "Trash item not found")
}

// Called under Server.mu, outside an existing mutate transaction. Archived trash
// remains protected from automatic deletion; explicit owner/admin purge is allowed.
func (s *Server) purgeExpiredTrash(ctx context.Context) error {
	ids := []string{}
	cutoff := now().Add(-trashRetention)
	for _, a := range s.st.Artifacts {
		if a.DeletedAt != nil && !a.Archived && !a.DeletedAt.After(cutoff) && s.inFlight[a.ID] == 0 {
			ids = append(ids, a.ID)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	return s.mutate(func() error {
		for _, id := range ids {
			s.tombstone(id)
			s.audit(principal{}, "artifact.trash_expired", id, "")
		}
		return nil
	})
}

// Soft deletion permanently revokes prior guest links and active content grants;
// restoring the artifact requires deliberately issuing any new guest access.
func (s *Server) invalidateArtifactGuests(id string) {
	at := now()
	for key, g := range s.st.GuestLinks {
		if g.ArtifactID == id {
			g.RevokedAt = &at
			s.st.GuestLinks[key] = g
		}
	}
	for key, se := range s.st.GuestSessions {
		if g, ok := s.st.GuestLinks[se.LinkID]; ok && g.ArtifactID == id {
			delete(s.st.GuestSessions, key)
		}
	}
	for key, v := range s.st.Grants {
		if v.ArtifactID == id {
			delete(s.st.Grants, key)
		}
	}
	for key, v := range s.st.Viewers {
		if v.ArtifactID == id {
			delete(s.st.Viewers, key)
		}
	}
}
