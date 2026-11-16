package server

import (
	"net/http"
	"net/url"
	"sort"
	"strings"
	"unicode/utf8"
)

func cleanTags(in []string) ([]string, error) {
	if len(in) > 30 {
		return nil, fail(400, "invalid_tags", "Use at most 30 tags")
	}
	out := []string{}
	for _, tag := range in {
		tag = strings.TrimSpace(tag)
		if tag == "" || utf8.RuneCountInString(tag) > 40 || strings.ContainsAny(tag, "\x00\r\n") {
			return nil, fail(400, "invalid_tags", "Tags must contain 1 to 40 characters")
		}
		if !contains(out, tag) {
			out = append(out, tag)
		}
	}
	return out, nil
}
func validSourceURL(v string) bool {
	u, e := url.Parse(v)
	return e == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != "" && u.User == nil && len(v) <= 2048
}
func validateProvenance(v Provenance) error {
	if len(v.Agent) > 200 || len(v.Model) > 200 || len(v.Summary) > 8000 || len(v.Commit) > 200 || len(v.Sources) > 20 {
		return fail(400, "invalid_provenance", "Creation context exceeds its limits")
	}
	for _, link := range append(append([]string{}, v.Sources...), v.Repository, v.TaskURL) {
		if link != "" && !validSourceURL(link) {
			return fail(400, "invalid_provenance", "Source links must be HTTP(S) URLs without embedded credentials")
		}
	}
	return nil
}
func (s *Server) artifactMetadata(w http.ResponseWriter, r *http.Request, p principal, a Artifact, section string) error {
	if e := s.require(r, p, false, "publish"); e != nil {
		return e
	}
	if section == "favorite" && r.Method == "PUT" {
		var in struct {
			Favorite bool `json:"favorite"`
		}
		if e := decode(r, &in); e != nil {
			return e
		}
		if e := s.mutate(func() error {
			if s.st.Favorites[p.user.ID] == nil {
				s.st.Favorites[p.user.ID] = map[string]bool{}
			}
			if in.Favorite {
				s.st.Favorites[p.user.ID][a.ID] = true
			} else {
				delete(s.st.Favorites[p.user.ID], a.ID)
			}
			s.audit(p, "artifact.favorite", a.ID, "")
			return nil
		}); e != nil {
			return e
		}
		writeJSON(w, 200, map[string]any{"artifact": s.publicArtifactFor(p, a)})
		return nil
	}
	if section != "metadata" || r.Method != "PATCH" {
		return fail(404, "not_found", "Not found")
	}
	if !s.own(p, a) {
		return fail(403, "forbidden", "Only the owner or administrator can change creation context")
	}
	var in struct {
		Tags       *[]string   `json:"tags"`
		Provenance *Provenance `json:"provenance"`
	}
	if e := decode(r, &in); e != nil {
		return e
	}
	if in.Tags != nil {
		tags, e := cleanTags(*in.Tags)
		if e != nil {
			return e
		}
		a.Tags = tags
	}
	if in.Provenance != nil {
		if e := validateProvenance(*in.Provenance); e != nil {
			return e
		}
		a.Provenance = *in.Provenance
	}
	if e := s.mutate(func() error { s.st.Artifacts[a.ID] = a; s.audit(p, "artifact.context_updated", a.ID, ""); return nil }); e != nil {
		return e
	}
	writeJSON(w, 200, map[string]any{"artifact": s.publicArtifactFor(p, a)})
	return nil
}
func (s *Server) collectionReadable(p principal, c Collection) bool {
	return p.user != nil && (p.user.ID == c.OwnerID || p.user.Role == "admin" || c.Visibility == "organization")
}
func (s *Server) publicCollection(p principal, c Collection) Collection {
	ids := []string{}
	for _, id := range c.ArtifactIDs {
		if a, ok := s.st.Artifacts[id]; ok && s.read(p, a) {
			ids = append(ids, id)
		}
	}
	c.ArtifactIDs = ids
	return c
}
func (s *Server) collections(w http.ResponseWriter, r *http.Request, p principal) error {
	scope := "publish"
	if r.Method == "GET" {
		scope = "read"
	}
	if e := s.require(r, p, false, scope); e != nil {
		return e
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/collections")
	id = strings.TrimPrefix(id, "/")
	if id == "" && r.Method == "GET" {
		items := []Collection{}
		for _, c := range s.st.Collections {
			if s.collectionReadable(p, c) {
				items = append(items, s.publicCollection(p, c))
			}
		}
		sort.Slice(items, func(i, j int) bool { return items[i].UpdatedAt.After(items[j].UpdatedAt) })
		writeJSON(w, 200, map[string]any{"collections": items})
		return nil
	}
	c, exists := s.st.Collections[id]
	if id != "" && (!exists || !s.collectionReadable(p, c)) {
		return fail(404, "not_found", "Collection not found")
	}
	if r.Method == "GET" {
		items := []Artifact{}
		c = s.publicCollection(p, c)
		for _, aid := range c.ArtifactIDs {
			items = append(items, s.publicArtifactFor(p, s.st.Artifacts[aid]))
		}
		writeJSON(w, 200, map[string]any{"collection": c, "artifacts": items})
		return nil
	}
	if exists && c.OwnerID != p.user.ID && p.user.Role != "admin" {
		return fail(403, "forbidden", "Only the collection owner or administrator can modify it")
	}
	if exists && r.Method == "DELETE" {
		if e := s.mutate(func() error { delete(s.st.Collections, id); s.audit(p, "collection.deleted", id, ""); return nil }); e != nil {
			return e
		}
		w.WriteHeader(204)
		return nil
	}
	if !(id == "" && r.Method == "POST") && !(exists && r.Method == "PATCH") {
		return fail(404, "not_found", "Not found")
	}
	var in struct {
		Name, Description, Visibility *string
		ArtifactIDs                   *[]string `json:"artifactIds"`
		BaseUpdatedAt                 string    `json:"baseUpdatedAt"`
	}
	if e := decode(r, &in); e != nil {
		return e
	}
	if !exists {
		c = Collection{ID: random(), OwnerID: p.user.ID, Visibility: "private", ArtifactIDs: []string{}, CreatedAt: now()}
	}
	if exists && in.BaseUpdatedAt != "" && in.BaseUpdatedAt != c.UpdatedAt.Format("2006-01-02T15:04:05.999999999Z07:00") {
		return fail(409, "stale_collection", "Collection changed; refresh before saving")
	}
	if in.Name != nil {
		c.Name = strings.TrimSpace(*in.Name)
	}
	if c.Name == "" || len(c.Name) > 200 {
		return fail(400, "invalid_collection", "Collection name must contain 1 to 200 bytes")
	}
	if in.Description != nil {
		c.Description = *in.Description
	}
	if len(c.Description) > 4000 {
		return fail(400, "invalid_collection", "Collection description is too long")
	}
	if in.Visibility != nil {
		c.Visibility = *in.Visibility
	}
	if c.Visibility != "private" && c.Visibility != "organization" {
		return fail(400, "invalid_visibility", "Collections may be private or organization-visible")
	}
	if in.ArtifactIDs != nil {
		if len(*in.ArtifactIDs) > 500 {
			return fail(400, "invalid_collection", "A collection supports at most 500 artifacts")
		}
		ids := []string{}
		for _, aid := range *in.ArtifactIDs {
			a, ok := s.st.Artifacts[aid]
			if !ok || !s.read(p, a) {
				return fail(404, "not_found", "Artifact not found")
			}
			if !contains(ids, aid) {
				ids = append(ids, aid)
			}
		}
		// Preserve hidden items when a manager reorders only the items they can see.
		for _, aid := range c.ArtifactIDs {
			a, ok := s.st.Artifacts[aid]
			if ok && !s.read(p, a) && !contains(ids, aid) {
				ids = append(ids, aid)
			}
		}
		c.ArtifactIDs = ids
	}
	c.UpdatedAt = now()
	if e := s.mutate(func() error {
		s.st.Collections[c.ID] = c
		s.audit(p, "collection.saved", c.ID, c.Visibility)
		return nil
	}); e != nil {
		return e
	}
	status := 200
	if !exists {
		status = 201
	}
	writeJSON(w, status, map[string]any{"collection": s.publicCollection(p, c)})
	return nil
}
