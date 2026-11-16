package server

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
)

func fmtSscan(v string, out *int) {
	n, e := strconv.Atoi(v)
	if e == nil {
		*out = n
	}
}
func mimeFor(p string) string {
	v := mime.TypeByExtension(path.Ext(p))
	if v == "" {
		return "application/octet-stream"
	}
	return v
}
func safePath(p string) bool {
	if p == "" || len(p) > 1024 || strings.ContainsAny(p, "\\\x00") || strings.HasPrefix(p, "/") || path.Clean(p) != p {
		return false
	}
	for _, part := range strings.Split(p, "/") {
		if part == "" || part == "." || part == ".." || strings.HasPrefix(part, ".") || strings.Contains(part, ":") {
			return false
		}
		for _, r := range part {
			if unicode.IsControl(r) {
				return false
			}
		}
	}
	return true
}
func slugOK(s string) bool {
	if len(s) < 1 || len(s) > 63 || s[0] == '-' || s[len(s)-1] == '-' {
		return false
	}
	for _, r := range s {
		if !(r >= 'a' && r <= 'z') && !(r >= '0' && r <= '9') && r != '-' {
			return false
		}
	}
	return true
}
func (s *Server) validateSharing(visibility string, ids []string, expiry *time.Time) error {
	if visibility != "private" && visibility != "organization" && visibility != "selected" && visibility != "public" {
		return fail(400, "invalid_visibility", "Invalid sharing scope")
	}
	if visibility == "selected" && len(ids) == 0 {
		return fail(400, "members_required", "Choose at least one active member")
	}
	for _, id := range ids {
		u, ok := s.st.Users[id]
		if !ok || u.Disabled {
			return fail(400, "invalid_member", "Sharing requires active members")
		}
	}
	if expiry != nil {
		*expiry = expiry.UTC()
	}
	if expiry != nil && !expiry.After(now()) {
		return fail(400, "invalid_expiry", "Expiry must be in the future")
	}
	return nil
}
func (s *Server) artifacts(w http.ResponseWriter, r *http.Request, p principal) error {
	if strings.HasPrefix(r.URL.Path, "/api/share/") && r.Method == "GET" {
		if p.kind == "token" {
			if e := s.require(r, p, false, "read"); e != nil {
				return e
			}
		}
		slug := strings.TrimPrefix(r.URL.Path, "/api/share/")
		for _, a := range s.st.Artifacts {
			if a.Slug == slug && s.read(p, a) {
				shared := s.publicArtifactFor(principal{}, a)
				if vid := r.URL.Query().Get("version"); vid != "" {
					v, ok := s.version(a, vid)
					if !ok || !s.readVersion(p, a, v) {
						return fail(404, "not_found", "Version not found")
					}
					shared = s.publicArtifactFor(p, a)
					shared.CurrentVersionID = v.ID
					shared.SizeBytes = v.SizeBytes
					shared.Provenance = v.Provenance
					if v.MetadataSnapshot {
						shared.Title = v.Title
						shared.Description = v.Description
						shared.Tags = append([]string{}, v.Tags...)
					}
				}
				if shared.CurrentVersionID == "" {
					return fail(404, "not_found", "No published version")
				}
				writeJSON(w, 200, map[string]any{"artifact": shared})
				return nil
			}
		}
		return fail(404, "not_found", "Artifact not found")
	}
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/artifacts"), "/")
	id := ""
	if len(parts) > 1 {
		id = parts[1]
	}
	if id == "" && r.Method == "GET" {
		if e := s.require(r, p, false, "read"); e != nil {
			return e
		}
		view := r.URL.Query().Get("view")
		if view == "" {
			view = "library"
		}
		if view != "library" && view != "mine" && view != "shared" && view != "archive" && view != "all" {
			return fail(400, "invalid_view", "Unknown collection")
		}
		if view == "all" && p.user.Role != "admin" {
			return fail(403, "forbidden", "Administrator required")
		}
		items := []Artifact{}
		q := strings.ToLower(r.URL.Query().Get("q"))
		for _, a := range s.st.Artifacts {
			if !s.read(p, a) {
				continue
			}
			if (view == "library" && (a.Archived || (a.OwnerID != p.user.ID && a.Visibility == "private"))) || (view == "mine" && (a.OwnerID != p.user.ID || a.Archived)) || (view == "shared" && (a.OwnerID == p.user.ID || a.Archived || a.Visibility == "private")) || (view == "archive" && !a.Archived) || (view == "all" && a.Archived) {
				continue
			}
			visible := s.publicArtifactFor(p, a)
			if r.URL.Query().Get("favorite") == "true" && !visible.Favorite {
				continue
			}
			if tag := r.URL.Query().Get("tag"); tag != "" && !contains(visible.Tags, tag) {
				continue
			}
			if q != "" {
				snippet, match := s.matchArtifact(r.Context(), p, a, q)
				if !match {
					continue
				}
				visible.SearchSnippet = snippet
			}
			items = append(items, visible)
		}
		sort.Slice(items, func(i, j int) bool {
			if r.URL.Query().Get("sort") == "title" {
				return strings.ToLower(items[i].Title) < strings.ToLower(items[j].Title)
			}
			if items[i].UpdatedAt.Equal(items[j].UpdatedAt) {
				return items[i].ID < items[j].ID
			}
			return items[i].UpdatedAt.After(items[j].UpdatedAt)
		})
		var pending int64
		for _, g := range s.st.Garbage {
			pending += g.Size
		}
		writeJSON(w, 200, map[string]any{"artifacts": items, "usage": map[string]int64{"usedBytes": s.used(), "quotaBytes": s.st.Policy.QuotaBytes, "pendingDeletionBytes": pending}, "policy": s.st.Policy})
		return nil
	}
	if id == "" && r.Method == "POST" {
		if e := s.require(r, p, false, "publish"); e != nil {
			return e
		}
		return s.publish(w, r, p, "")
	}
	a, ok := s.st.Artifacts[id]
	if !ok || !s.read(p, a) {
		return fail(404, "not_found", "Artifact not found")
	}
	if len(parts) >= 3 {
		switch parts[2] {
		case "comments", "review", "promote", "reviewers":
			return s.reviews(w, r, p, a, parts)
		case "guest-links":
			return s.guestLinks(w, r, p, a)
		case "metadata", "favorite":
			return s.artifactMetadata(w, r, p, a, parts[2])
		case "diff":
			return s.diff(w, r, p, a)
		}
	}
	if len(parts) == 3 && parts[2] == "launch" && r.Method == "POST" {
		if p.user != nil {
			if e := s.require(r, p, false, "read"); e != nil {
				return e
			}
		}
		return s.launch(w, r, p, a)
	}
	if r.Method == "GET" && p.user != nil {
		if e := s.require(r, p, false, "read"); e != nil {
			return e
		}
	}
	if len(parts) == 2 && r.Method == "GET" {
		v := []Version{}
		for _, x := range s.st.Versions[id] {
			if s.readVersion(p, a, x) {
				v = append(v, s.publicVersion(x))
			}
		}
		writeJSON(w, 200, map[string]any{"artifact": s.publicArtifactFor(p, a), "versions": v})
		return nil
	}
	if len(parts) == 3 && parts[2] == "download" && r.Method == "GET" {
		v, ok := s.version(a, r.URL.Query().Get("versionId"))
		if r.URL.Query().Get("versionId") == "" {
			v, ok = s.version(a, s.effectiveVersionID(p, a))
		}
		if !ok || !s.readVersion(p, a, v) {
			return fail(404, "not_found", "Version not found")
		}
		a.CurrentVersionID = v.ID
		return s.download(w, r, a)
	}
	scope := "publish"
	if len(parts) == 3 && parts[2] == "sharing" {
		scope = "share"
	}
	if e := s.require(r, p, false, scope); e != nil {
		return e
	}
	if !s.own(p, a) {
		return fail(403, "forbidden", "Only the owner or an administrator can modify this artifact")
	}
	if len(parts) == 3 && parts[2] == "versions" && r.Method == "POST" {
		return s.publish(w, r, p, id)
	}
	if len(parts) == 2 && r.Method == "PATCH" {
		var in struct{ Title, Description *string }
		if e := decode(r, &in); e != nil {
			return e
		}
		if in.Title != nil {
			if strings.TrimSpace(*in.Title) == "" || len(*in.Title) > 300 {
				return fail(400, "invalid_title", "Title must contain 1 to 300 bytes")
			}
			a.Title = *in.Title
		}
		if in.Description != nil {
			if len(*in.Description) > 10000 {
				return fail(400, "invalid_description", "Description is too long")
			}
			a.Description = *in.Description
		}
		if e := s.mutate(func() error { s.st.Artifacts[id] = a; s.audit(p, "artifact.updated", id, ""); return nil }); e != nil {
			return e
		}
		writeJSON(w, 200, map[string]any{"artifact": s.publicArtifact(a)})
		return nil
	}
	if len(parts) == 3 && parts[2] == "sharing" && r.Method == "PATCH" {
		var in struct {
			Visibility string
			MemberIDs  []string
			ExpiresAt  *time.Time
		}
		if e := decode(r, &in); e != nil {
			return e
		}
		if e := s.validateSharing(in.Visibility, in.MemberIDs, in.ExpiresAt); e != nil {
			return e
		}
		a.Visibility = in.Visibility
		a.MemberIDs = []string{}
		if in.Visibility == "selected" {
			a.MemberIDs = append(a.MemberIDs, in.MemberIDs...)
		}
		a.ExpiresAt = in.ExpiresAt
		if e := s.mutate(func() error { s.st.Artifacts[id] = a; s.audit(p, "artifact.sharing", id, a.Visibility); return nil }); e != nil {
			return e
		}
		writeJSON(w, 200, map[string]any{"artifact": s.publicArtifact(a)})
		return nil
	}
	if len(parts) == 3 && (parts[2] == "archive" || parts[2] == "restore") && r.Method == "POST" {
		a.Archived = parts[2] == "archive"
		if e := s.mutate(func() error { s.st.Artifacts[id] = a; s.audit(p, "artifact."+parts[2], id, ""); return nil }); e != nil {
			return e
		}
		writeJSON(w, 200, map[string]any{"artifact": s.publicArtifact(a)})
		return nil
	}
	if len(parts) == 2 && r.Method == "DELETE" {
		return s.trash(w, r, p, a)
	}
	if len(parts) == 5 && parts[2] == "versions" && parts[4] == "restore" && r.Method == "POST" {
		var v *Version
		for _, x := range s.st.Versions[id] {
			if x.ID == parts[3] {
				copy := x
				v = &copy
				break
			}
		}
		if v == nil {
			return fail(404, "not_found", "Version not found")
		}
		a.CurrentVersionID = v.ID
		if !a.ReviewEnabled {
			a.PublishedVersionID = v.ID
		}
		a.SizeBytes = v.SizeBytes
		a.UpdatedAt = now()
		if e := s.mutate(func() error { s.st.Artifacts[id] = a; s.audit(p, "artifact.rollback", id, v.ID); return nil }); e != nil {
			return e
		}
		writeJSON(w, 200, map[string]any{"artifact": s.publicArtifact(a)})
		return nil
	}
	return fail(404, "not_found", "Not found")
}

type uploadMeta struct {
	Title                    string      `json:"title"`
	Description              string      `json:"description"`
	Slug                     string      `json:"slug"`
	Visibility               string      `json:"visibility"`
	MemberIDs                []string    `json:"memberIds"`
	ExpiresAt                *time.Time  `json:"expiresAt"`
	Message                  string      `json:"message"`
	BaseVersionID            string      `json:"baseVersionId"`
	Draft                    bool        `json:"draft"`
	Provenance               *Provenance `json:"provenance,omitempty"`
	Tags                     *[]string   `json:"tags,omitempty"`
	titleSet, descriptionSet bool
	Files                    []struct {
		Path     string `json:"path"`
		MIMEType string `json:"mimeType"`
	} `json:"files"`
}
type staged struct {
	meta  uploadMeta
	files []File
	size  int64
}

// parseUpload keeps staged bytes on bounded temporary files, not in process memory.
func parseUpload(r *http.Request, limit int64, stageDir string) (st staged, temps []string, err error) {
	if limit > 1<<40 {
		return st, nil, fail(400, "invalid_limit", "Upload limit exceeds supported maximum")
	}
	r.Body = http.MaxBytesReader(nil, r.Body, limit+(8<<20))
	defer r.Body.Close()
	mr, e := r.MultipartReader()
	if e != nil {
		return st, nil, fail(400, "invalid_multipart", "Expected multipart upload")
	}
	defer func() {
		if err != nil {
			for _, p := range temps {
				os.Remove(p)
			}
		}
	}()
	part, e := mr.NextPart()
	if e != nil || part.FormName() != "metadata" {
		return st, nil, fail(400, "metadata_required", "The first multipart part must be metadata")
	}
	raw, e := io.ReadAll(io.LimitReader(part, (2<<20)+1))
	if e != nil || len(raw) > 2<<20 {
		return st, nil, fail(400, "invalid_metadata", "Metadata too large")
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if e = d.Decode(&st.meta); e != nil {
		return st, nil, fail(400, "invalid_metadata", "Invalid upload metadata")
	}
	var tail any
	if e = d.Decode(&tail); !errors.Is(e, io.EOF) {
		return st, nil, fail(400, "invalid_metadata", "Expected one metadata object")
	}
	var fields map[string]json.RawMessage
	json.Unmarshal(raw, &fields)
	_, st.meta.titleSet = fields["title"]
	_, st.meta.descriptionSet = fields["description"]
	if len(st.meta.Files) == 0 || len(st.meta.Files) > 2500 {
		return st, nil, fail(400, "invalid_file_count", "Upload must contain 1 to 2500 files")
	}
	seen := map[string]bool{}
	for _, f := range st.meta.Files {
		if !safePath(f.Path) || seen[f.Path] {
			return st, nil, fail(400, "invalid_path", "Paths must be unique, relative and free of dot or secret components")
		}
		seen[f.Path] = true
	}
	if !seen["index.html"] {
		return st, nil, fail(400, "index_required", "An index.html entry file is required")
	}
	os.MkdirAll(stageDir, 0700)
	for i, f := range st.meta.Files {
		p, e := mr.NextPart()
		if e != nil || p.FormName() != fmt.Sprintf("file%d", i) {
			return st, temps, fail(400, "missing_file", "File parts must match metadata order")
		}
		tmp, e := os.CreateTemp(stageDir, "upload-")
		if e != nil {
			return st, temps, e
		}
		temps = append(temps, tmp.Name())
		n, e := io.Copy(tmp, io.LimitReader(p, limit-st.size+1))
		closeErr := tmp.Close()
		if e != nil || closeErr != nil {
			return st, temps, fail(400, "upload_failed", "Cannot read upload")
		}
		st.size += n
		if st.size > limit {
			return st, temps, fail(413, "upload_too_large", "Upload exceeds policy limit")
		}
		mt := f.MIMEType
		if mt == "" {
			mt = mimeFor(f.Path)
		}
		if _, _, e := mime.ParseMediaType(mt); e != nil {
			return st, temps, fail(400, "invalid_mime", "Invalid content type")
		}
		st.files = append(st.files, File{Path: f.Path, MIMEType: mt, Size: n, Key: "versions/" + random()})
	}
	if _, e := mr.NextPart(); !errors.Is(e, io.EOF) {
		return st, temps, fail(400, "extra_file", "Unexpected multipart part")
	}
	return st, temps, nil
}
func (s *Server) publish(w http.ResponseWriter, r *http.Request, p principal, id string) error {
	backendID := ""
	for _, b := range s.st.Backends {
		if b.IsDefault {
			backendID = b.ID
		}
	}
	store, e := s.store(backendID)
	if e != nil {
		return fail(502, "storage_unavailable", "Storage backend is unavailable")
	}
	limit := s.st.Policy.MaxUploadBytes
	uploadID := id
	s.pendingBackends[backendID]++
	if uploadID != "" {
		s.inFlight[uploadID]++
	}
	defer func() {
		s.pendingBackends[backendID]--
		if uploadID != "" {
			s.inFlight[uploadID]--
		}
	}()
	// Release the state mutex for potentially slow request streaming and remote blob IO.
	s.mu.Unlock()
	st, temps, stageErr := parseUpload(r, limit, s.cfg.DataDir+"/staging")
	// Persist every potential provider key before the first Put. A crash leaves
	// recoverable tombstones; active leases prevent concurrent GC during staging.
	s.mu.Lock()
	lease := random()
	registered := false
	if stageErr == nil {
		stageErr = s.mutate(func() error {
			for _, f := range st.files {
				s.st.Garbage = append(s.st.Garbage, Garbage{BackendID: backendID, Key: f.Key, Size: f.Size, UploadID: lease})
			}
			return nil
		})
		if stageErr == nil {
			registered = true
			s.pendingUploads[lease] = true
		}
	}
	s.mu.Unlock()
	if stageErr == nil {
		for i, f := range st.files {
			file, e := os.Open(temps[i])
			if e == nil {
				e = store.Put(r.Context(), f.Key, file, f.Size, f.MIMEType)
				file.Close()
			}
			if e != nil {
				stageErr = fail(502, "storage_failed", "Upload could not be stored")
				break
			}
		}
	}
	for _, v := range temps {
		os.Remove(v)
	}
	s.mu.Lock()
	delete(s.pendingUploads, lease)
	queueOrphans := func() {
		if registered {
			s.gc(context.WithoutCancel(r.Context()))
		}
	}

	if stageErr != nil {
		queueOrphans()
		return stageErr
	}
	if s.closed {
		queueOrphans()
		return fail(503, "closed", "Service closed during publication; staged objects will be recovered by maintenance")
	}
	p = s.principal(r)
	if e = s.require(r, p, false, "publish"); e != nil {
		queueOrphans()
		return e
	}
	if e = s.csrf(r, p); e != nil {
		queueOrphans()
		return e
	}
	if st.size > s.st.Policy.MaxUploadBytes {
		queueOrphans()
		return fail(413, "upload_too_large", "Upload exceeds current policy limit")
	}
	if _, exists := s.st.Backends[backendID]; !exists {
		queueOrphans()
		return fail(409, "backend_changed", "Storage configuration changed; retry publication")
	}
	m := st.meta
	if id == "" && m.Visibility != "" && m.Visibility != "private" && p.kind == "token" && !contains(p.scopes, "share") {
		queueOrphans()
		return fail(403, "insufficient_scope", "Creating a shared artifact requires the share scope")
	}
	if id == "" {
		if strings.TrimSpace(m.Title) == "" || len(m.Title) > 300 || len(m.Description) > 10000 {
			queueOrphans()
			return fail(400, "invalid_metadata", "Title or description is invalid")
		}
		if m.Visibility == "" {
			m.Visibility = "private"
		}
		if e = s.validateSharing(m.Visibility, m.MemberIDs, m.ExpiresAt); e != nil {
			queueOrphans()
			return e
		}
		if m.Slug == "" {
			m.Slug = "a-" + digest(random())[:16]
		}
		if !slugOK(m.Slug) || strings.HasPrefix(m.Slug, "v--") {
			queueOrphans()
			return fail(400, "invalid_slug", "Slug must be a DNS-safe lowercase label")
		}
		for _, a := range s.st.Artifacts {
			if a.Slug == m.Slug {
				queueOrphans()
				return fail(409, "slug_exists", "Slug is already in use")
			}
		}
		id = random()
	} else {
		a, ok := s.st.Artifacts[id]
		if !ok || a.DeletedAt != nil || !s.own(p, a) {
			queueOrphans()
			return fail(404, "not_found", "Artifact not found")
		}
		if m.BaseVersionID == "" || a.CurrentVersionID != m.BaseVersionID {
			queueOrphans()
			return fail(409, "stale_version", "Artifact changed; refresh before publishing")
		}
	}
	a, existing := s.st.Artifacts[id]
	if existing {
		if m.titleSet {
			if strings.TrimSpace(m.Title) == "" || len(m.Title) > 300 {
				queueOrphans()
				return fail(400, "invalid_title", "Title must contain 1 to 300 bytes")
			}
			a.Title = m.Title
		}
		if m.descriptionSet {
			if len(m.Description) > 10000 {
				queueOrphans()
				return fail(400, "invalid_description", "Description is too long")
			}
			a.Description = m.Description
		}
	}
	if !existing {
		a = Artifact{ID: id, Slug: m.Slug, Title: m.Title, Description: m.Description, OwnerID: p.user.ID, Visibility: m.Visibility, MemberIDs: []string{}, ExpiresAt: m.ExpiresAt, CreatedAt: now(), Kind: "html"}
		if m.Visibility == "selected" {
			a.MemberIDs = m.MemberIDs
		}
	}
	if m.Provenance != nil {
		if e := validateProvenance(*m.Provenance); e != nil {
			queueOrphans()
			return e
		}
		a.Provenance = *m.Provenance
	}
	if m.Tags != nil {
		tags, e := cleanTags(*m.Tags)
		if e != nil {
			queueOrphans()
			return e
		}
		a.Tags = tags
	}
	v := Version{ID: random(), ArtifactID: id, Number: len(s.st.Versions[id]) + 1, CreatedAt: now(), SizeBytes: st.size, BackendID: backendID, Files: st.files, Message: m.Message, Provenance: a.Provenance, Title: a.Title, Description: a.Description, Tags: append([]string{}, a.Tags...), MetadataSnapshot: true}
	// Text is indexed before publication and persisted with the version, never executed.
	v.SearchText = s.extractVersionText(r.Context(), v)
	v.SearchIndexed = true
	e = s.mutate(func() error {
		need := s.used() + v.SizeBytes - s.st.Policy.QuotaBytes
		if need > 0 {
			if s.st.Policy.OverflowPolicy != "delete_oldest" {
				return fail(413, "quota_exceeded", "Storage quota exceeded")
			}
			var freed int64
			candidates := s.eligible(id)
			for _, old := range candidates {
				freed += s.artifactBytes(old.ID)
				s.tombstone(old.ID)
				s.audit(p, "artifact.quota_evicted", old.ID, "")
				if freed >= need {
					break
				}
			}
			if freed < need {
				return fail(413, "quota_exceeded", "Archived or protected data prevents quota admission")
			}
		}
		if m.Draft && !a.ReviewEnabled {
			a.ReviewEnabled = true
			a.PublishedVersionID = a.CurrentVersionID
			for i := range s.st.Versions[id] {
				s.st.Versions[id][i].Published = true
			}
		}
		v.Published = !a.ReviewEnabled
		if v.Published {
			a.PublishedVersionID = v.ID
		}
		a.CurrentVersionID = v.ID
		a.SizeBytes = v.SizeBytes
		a.UpdatedAt = v.CreatedAt
		s.st.Artifacts[id] = a
		s.st.Versions[id] = append(s.st.Versions[id], v)
		retained := s.st.Garbage[:0]
		for _, g := range s.st.Garbage {
			if g.UploadID != lease {
				retained = append(retained, g)
			}
		}
		s.st.Garbage = retained
		s.audit(p, "artifact.version_created", id, v.ID)
		return nil
	})
	if e != nil {
		queueOrphans()
		return e
	}
	s.gc(context.WithoutCancel(r.Context()))
	writeJSON(w, 201, map[string]any{"artifact": s.publicArtifact(a), "version": s.publicVersion(v)})
	return nil
}
func (s *Server) version(a Artifact, id string) (Version, bool) {
	if id == "" {
		id = a.CurrentVersionID
	}
	for _, v := range s.st.Versions[a.ID] {
		if v.ID == id {
			return v, true
		}
	}
	return Version{}, false
}
func (s *Server) download(w http.ResponseWriter, r *http.Request, a Artifact) error {
	v, ok := s.version(a, "")
	if !ok {
		return fail(404, "not_found", "Version not found")
	}
	store, e := s.store(v.BackendID)
	if e != nil {
		return fail(502, "storage_unavailable", "Storage is unavailable")
	}
	tmp, e := os.CreateTemp(s.cfg.DataDir, "download-")
	if e != nil {
		return e
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()
	z := zip.NewWriter(tmp)
	for _, f := range v.Files {
		rd, e := store.Open(r.Context(), f.Key)
		if e != nil {
			z.Close()
			return fail(502, "storage_failed", "Cannot read stored content")
		}
		dst, e := z.Create(f.Path)
		if e == nil {
			_, e = io.Copy(dst, rd)
		}
		rd.Close()
		if e != nil {
			z.Close()
			return e
		}
	}
	if e = z.Close(); e != nil {
		return e
	}
	tmp.Seek(0, 0)
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", a.Slug+".zip"))
	_, e = io.Copy(w, tmp)
	return e
}

type CleanupCandidate struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Reason    string `json:"reason"`
	SizeBytes int64  `json:"sizeBytes"`
}
type CleanupResult struct {
	Candidates           []CleanupCandidate `json:"candidates"`
	FreedBytes           int64              `json:"freedBytes"`
	PendingDeletionBytes int64              `json:"pendingDeletionBytes"`
}

func (s *Server) artifactBytes(id string) int64 {
	var n int64
	for _, v := range s.st.Versions[id] {
		n += v.SizeBytes
	}
	return n
}
func (s *Server) eligible(exclude string) []Artifact {
	out := []Artifact{}
	for _, a := range s.st.Artifacts {
		u, ok := s.st.Users[a.OwnerID]
		if a.ID != exclude && s.inFlight[a.ID] == 0 && a.DeletedAt == nil && !a.Archived && ok && !u.Disabled {
			out = append(out, a)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].UpdatedAt.Equal(out[j].UpdatedAt) {
			return out[i].ID < out[j].ID
		}
		return out[i].UpdatedAt.Before(out[j].UpdatedAt)
	})
	return out
}
func (s *Server) tombstone(id string) {
	for key, c := range s.st.Comments {
		if c.ArtifactID == id {
			delete(s.st.Comments, key)
		}
	}
	for key, r := range s.st.Reviews {
		if r.ArtifactID == id {
			delete(s.st.Reviews, key)
		}
	}
	for key, g := range s.st.GuestLinks {
		if g.ArtifactID == id {
			delete(s.st.GuestLinks, key)
		}
	}
	for key, col := range s.st.Collections {
		ids := []string{}
		for _, a := range col.ArtifactIDs {
			if a != id {
				ids = append(ids, a)
			}
		}
		col.ArtifactIDs = ids
		s.st.Collections[key] = col
	}
	for uid, favs := range s.st.Favorites {
		delete(favs, id)
		s.st.Favorites[uid] = favs
	}
	for _, v := range s.st.Versions[id] {
		for _, f := range v.Files {
			s.st.Garbage = append(s.st.Garbage, Garbage{BackendID: v.BackendID, Key: f.Key, Size: f.Size})
		}
	}
	delete(s.st.Versions, id)
	delete(s.st.Artifacts, id)
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
	for key, d := range s.st.Domains {
		if d.ArtifactID == id {
			d.Status = "error"
			d.VerifiedAt = nil
			d.LastError = "The bound artifact was deleted. Detach this domain, optionally deleting its Folio-owned DNS records."
			s.st.Domains[key] = d
		}
	}
}
func (s *Server) gc(ctx context.Context) {
	remaining := []Garbage{}
	changed := false
	for _, g := range s.st.Garbage {
		if g.UploadID != "" && s.pendingUploads[g.UploadID] {
			remaining = append(remaining, g)
			continue
		}
		store, e := s.store(g.BackendID)
		if e == nil {
			e = store.Delete(ctx, g.Key)
		}
		if e != nil {
			if g.Error == "" {
				s.audit(principal{}, "storage.gc_failed", g.BackendID, "Provider deletion failed; retained in cleanup queue")
			}
			g.Error = "Storage deletion failed"
			remaining = append(remaining, g)
		} else {
			if g.Error != "" {
				s.audit(principal{}, "storage.gc_recovered", g.BackendID, "Pending provider deletion completed")
			}
			changed = true
		}
	}
	s.st.Garbage = remaining
	if changed || len(remaining) > 0 {
		s.save()
	}
}
func (s *Server) cleanup(ctx context.Context, dry bool, p principal) (CleanupResult, error) {
	out := CleanupResult{Candidates: []CleanupCandidate{}}
	cutoffTrash := now().Add(-trashRetention)
	for _, a := range s.st.Artifacts {
		if a.DeletedAt != nil && !a.Archived && !a.DeletedAt.After(cutoffTrash) && s.inFlight[a.ID] == 0 {
			n := s.artifactBytes(a.ID)
			out.Candidates = append(out.Candidates, CleanupCandidate{a.ID, a.Title, "trash_expired", n})
			out.FreedBytes += n
		}
	}
	if s.st.Policy.RetentionDays > 0 {
		cutoff := now().Add(-time.Duration(s.st.Policy.RetentionDays) * 24 * time.Hour)
		for _, a := range s.eligible("") {
			if a.UpdatedAt.Before(cutoff) {
				n := s.artifactBytes(a.ID)
				out.Candidates = append(out.Candidates, CleanupCandidate{a.ID, a.Title, "retention", n})
				out.FreedBytes += n
			}
		}
	}
	if !dry {
		for key, a := range s.loginAttempts {
			if !a.Until.After(now()) {
				delete(s.loginAttempts, key)
			}
		}
		if e := s.mutate(func() error {
			s.pruneGuestSessions()
			for key, se := range s.st.Sessions {
				if !se.ExpiresAt.After(now()) {
					delete(s.st.Sessions, key)
				}
			}
			for key, v := range s.st.Grants {
				if !v.ExpiresAt.After(now()) {
					delete(s.st.Grants, key)
				}
			}
			for key, v := range s.st.Viewers {
				if !v.ExpiresAt.After(now()) {
					delete(s.st.Viewers, key)
				}
			}
			for _, c := range out.Candidates {
				s.tombstone(c.ID)
				action := "artifact.retention_deleted"
				if c.Reason == "trash_expired" {
					action = "artifact.trash_expired"
				}
				s.audit(p, action, c.ID, "")
			}
			s.audit(p, "cleanup.executed", "", "")
			return nil
		}); e != nil {
			return out, e
		}
		s.gc(ctx)
	}
	for _, g := range s.st.Garbage {
		out.PendingDeletionBytes += g.Size
	}
	return out, nil
}
