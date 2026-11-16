package server

import (
	"net/http"
	"sort"
	"strings"
	"time"
	"unicode"
)

// Review is a decision about one immutable artifact version, never the latest pointer.
type Review struct {
	ArtifactID    string    `json:"artifactId"`
	VersionID     string    `json:"versionId"`
	Status        string    `json:"status"`
	UpdatedBy     string    `json:"updatedBy"`
	UpdatedByName string    `json:"updatedByName"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

// Comment replies retain their root thread when an author removes its text.
type Comment struct {
	ID         string    `json:"id"`
	ArtifactID string    `json:"artifactId"`
	VersionID  string    `json:"versionId"`
	ParentID   string    `json:"parentId,omitempty"`
	AuthorID   string    `json:"authorId"`
	AuthorName string    `json:"authorName"`
	Body       string    `json:"body"`
	Path       string    `json:"path,omitempty"`
	Anchor     string    `json:"anchor,omitempty"`
	Resolved   bool      `json:"resolved"`
	Deleted    bool      `json:"deleted"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

func (s *Server) reviewWrite(r *http.Request, p principal) error {
	if e := s.require(r, p, false, ""); e != nil {
		return e
	}
	if p.kind == "token" && !contains(p.scopes, "review") && !contains(p.scopes, "publish") {
		return fail(403, "insufficient_scope", "API token requires review or publish scope")
	}
	return nil
}
func (s *Server) reviewVersion(p principal, a Artifact, id string) (Version, error) {
	if id == "" {
		return Version{}, fail(400, "version_required", "An exact versionId is required")
	}
	for _, v := range s.st.Versions[a.ID] {
		if v.ID == id && s.readVersion(p, a, v) {
			return v, nil
		}
	}
	return Version{}, fail(404, "not_found", "Version not found")
}
func validReviewStatus(status string) bool {
	return status == "draft" || status == "in_review" || status == "approved" || status == "changes_requested"
}
func validCommentBody(body string) bool {
	return strings.TrimSpace(body) != "" && len(body) <= 20000 && !strings.ContainsRune(body, 0)
}
func (s *Server) reviewFor(a Artifact, v Version) Review {
	if review, ok := s.st.Reviews[v.ID]; ok && review.ArtifactID == a.ID {
		return review
	}
	status := "draft"
	if v.Published || !a.ReviewEnabled {
		status = "approved"
	}
	return Review{ArtifactID: a.ID, VersionID: v.ID, Status: status}
}

func (s *Server) reviews(w http.ResponseWriter, r *http.Request, p principal, a Artifact, parts []string) error {
	if e := s.require(r, p, false, ""); e != nil {
		return e
	}
	if !s.read(p, a) || a.DeletedAt != nil {
		return fail(404, "not_found", "Artifact not found")
	}
	if len(parts) < 3 {
		return fail(404, "not_found", "Not found")
	}
	if r.Method == "GET" {
		if e := s.require(r, p, false, "read"); e != nil {
			return e
		}
	} else if len(parts) == 3 && parts[2] == "reviewers" && r.Method == "PATCH" {
		return s.setReviewers(w, r, p, a)
	} else if e := s.reviewWrite(r, p); e != nil {
		return e
	}
	if parts[2] == "comments" {
		return s.comments(w, r, p, a, parts)
	}
	if len(parts) != 3 {
		return fail(404, "not_found", "Not found")
	}
	if parts[2] == "review" && r.Method == "GET" {
		v, e := s.reviewVersion(p, a, r.URL.Query().Get("versionId"))
		if e != nil {
			return e
		}
		writeJSON(w, 200, map[string]any{"review": s.reviewFor(a, v)})
		return nil
	}
	if parts[2] == "review" && (r.Method == "PUT" || r.Method == "PATCH") {
		var in struct {
			VersionID string `json:"versionId"`
			Status    string `json:"status"`
		}
		if e := decode(r, &in); e != nil {
			return e
		}
		v, e := s.reviewVersion(p, a, in.VersionID)
		if e != nil {
			return e
		}
		if !validReviewStatus(in.Status) {
			return fail(400, "invalid_review_status", "Choose draft, in_review, approved or changes_requested")
		}
		review := Review{ArtifactID: a.ID, VersionID: v.ID, Status: in.Status, UpdatedBy: p.user.ID, UpdatedByName: p.user.Name, UpdatedAt: now()}
		if e = s.mutate(func() error {
			if s.st.Reviews == nil {
				s.st.Reviews = map[string]Review{}
			}
			s.st.Reviews[v.ID] = review
			s.audit(p, "review.updated", a.ID, v.ID+":"+in.Status)
			return nil
		}); e != nil {
			return e
		}
		writeJSON(w, 200, map[string]any{"review": review})
		return nil
	}
	if parts[2] == "promote" && r.Method == "POST" {
		if e := s.require(r, p, false, "publish"); e != nil {
			return e
		}
		if !s.own(p, a) {
			return fail(403, "forbidden", "Only the owner or an administrator can publish a reviewed version")
		}
		var in struct {
			VersionID              string  `json:"versionId"`
			BasePublishedVersionID *string `json:"basePublishedVersionId"`
		}
		if e := decode(r, &in); e != nil {
			return e
		}
		if in.BasePublishedVersionID == nil {
			return fail(400, "base_version_required", "basePublishedVersionId is required")
		}
		v, e := s.reviewVersion(p, a, in.VersionID)
		if e != nil {
			return e
		}
		if *in.BasePublishedVersionID != a.PublishedVersionID {
			return fail(409, "version_conflict", "The published version changed; reload before publishing")
		}
		if s.reviewFor(a, v).Status != "approved" {
			return fail(409, "review_not_approved", "This exact version must be approved before publishing")
		}
		if e = s.mutate(func() error {
			versions := append([]Version(nil), s.st.Versions[a.ID]...)
			for i := range versions {
				if !a.ReviewEnabled || versions[i].ID == v.ID {
					versions[i].Published = true
				}
			}
			s.st.Versions[a.ID] = versions
			a.ReviewEnabled = true
			a.PublishedVersionID = v.ID
			a.UpdatedAt = now()
			s.st.Artifacts[a.ID] = a
			s.audit(p, "artifact.promoted", a.ID, v.ID)
			return nil
		}); e != nil {
			return e
		}
		writeJSON(w, 200, map[string]any{"artifact": s.publicArtifactFor(p, a), "review": s.reviewFor(a, v)})
		return nil
	}
	return fail(404, "not_found", "Not found")
}

func (s *Server) comments(w http.ResponseWriter, r *http.Request, p principal, a Artifact, parts []string) error {
	if len(parts) == 3 && r.Method == "GET" {
		v, e := s.reviewVersion(p, a, r.URL.Query().Get("versionId"))
		if e != nil {
			return e
		}
		out := []Comment{}
		for _, c := range s.st.Comments {
			if c.ArtifactID == a.ID && c.VersionID == v.ID {
				out = append(out, c)
			}
		}
		sort.Slice(out, func(i, j int) bool {
			if out[i].CreatedAt.Equal(out[j].CreatedAt) {
				return out[i].ID < out[j].ID
			}
			return out[i].CreatedAt.Before(out[j].CreatedAt)
		})
		writeJSON(w, 200, map[string]any{"comments": out})
		return nil
	}
	if len(parts) == 3 && r.Method == "POST" {
		var in struct {
			VersionID string `json:"versionId"`
			ParentID  string `json:"parentId"`
			Body      string `json:"body"`
			Path      string `json:"path"`
			Anchor    string `json:"anchor"`
		}
		if e := decode(r, &in); e != nil {
			return e
		}
		v, e := s.reviewVersion(p, a, in.VersionID)
		if e != nil {
			return e
		}
		if !validCommentBody(in.Body) {
			return fail(400, "invalid_comment", "Comment must contain 1 to 20000 bytes")
		}
		if in.ParentID != "" {
			parent, ok := s.st.Comments[in.ParentID]
			if !ok || parent.ArtifactID != a.ID || parent.VersionID != v.ID || parent.ParentID != "" {
				return fail(400, "invalid_parent", "Reply must belong to a root thread on this version")
			}
			if parent.Resolved {
				return fail(409, "thread_resolved", "Reopen the thread before replying")
			}
			if in.Path != "" || in.Anchor != "" {
				return fail(400, "invalid_anchor", "Replies inherit their thread location")
			}
		}
		if in.Path != "" {
			found := false
			for _, f := range v.Files {
				if f.Path == in.Path {
					found = true
					break
				}
			}
			if !found {
				return fail(400, "invalid_path", "Comment path must exist in this version")
			}
		}
		if len(in.Anchor) > 1000 || strings.IndexFunc(in.Anchor, unicode.IsControl) >= 0 || (in.Anchor != "" && in.Path == "") {
			return fail(400, "invalid_anchor", "Anchor requires a file path and at most 1000 bytes without control characters")
		}
		c := Comment{ID: random(), ArtifactID: a.ID, VersionID: v.ID, ParentID: in.ParentID, AuthorID: p.user.ID, AuthorName: p.user.Name, Body: strings.TrimSpace(in.Body), Path: in.Path, Anchor: in.Anchor, CreatedAt: now(), UpdatedAt: now()}
		if e = s.mutate(func() error {
			if s.st.Comments == nil {
				s.st.Comments = map[string]Comment{}
			}
			s.st.Comments[c.ID] = c
			s.audit(p, "comment.created", a.ID, c.ID+":"+v.ID)
			return nil
		}); e != nil {
			return e
		}
		writeJSON(w, 201, map[string]any{"comment": c})
		return nil
	}
	if len(parts) != 4 || (r.Method != "PATCH" && r.Method != "DELETE") {
		return fail(404, "not_found", "Not found")
	}
	c, ok := s.st.Comments[parts[3]]
	if !ok || c.ArtifactID != a.ID {
		return fail(404, "not_found", "Comment not found")
	}
	if _, e := s.reviewVersion(p, a, c.VersionID); e != nil {
		return e
	}
	if r.Method == "DELETE" {
		if p.user.ID != c.AuthorID && p.user.Role != "admin" {
			return fail(403, "forbidden", "Only the author or an administrator can delete this comment")
		}
		if e := s.mutate(func() error {
			c.Deleted = true
			c.Body = ""
			c.Path = ""
			c.Anchor = ""
			c.UpdatedAt = now()
			s.st.Comments[c.ID] = c
			s.audit(p, "comment.deleted", a.ID, c.ID)
			return nil
		}); e != nil {
			return e
		}
		w.WriteHeader(204)
		return nil
	}
	var in struct {
		Body     *string `json:"body"`
		Resolved *bool   `json:"resolved"`
	}
	if e := decode(r, &in); e != nil {
		return e
	}
	if in.Body == nil && in.Resolved == nil {
		return fail(400, "required_field", "body or resolved is required")
	}
	if in.Body != nil {
		if p.user.ID != c.AuthorID && p.user.Role != "admin" {
			return fail(403, "forbidden", "Only the author or an administrator can edit this comment")
		}
		if c.Deleted {
			return fail(409, "comment_deleted", "Deleted comments cannot be edited")
		}
		if !validCommentBody(*in.Body) {
			return fail(400, "invalid_comment", "Comment must contain 1 to 20000 bytes")
		}
		c.Body = strings.TrimSpace(*in.Body)
	}
	if in.Resolved != nil {
		if c.ParentID != "" {
			return fail(400, "invalid_thread", "Only root threads can be resolved or reopened")
		}
		if p.user.ID != c.AuthorID && !s.own(p, a) {
			return fail(403, "forbidden", "Only the thread author, artifact owner or an administrator can resolve a thread")
		}
		c.Resolved = *in.Resolved
	}
	c.UpdatedAt = now()
	if e := s.mutate(func() error { s.st.Comments[c.ID] = c; s.audit(p, "comment.updated", a.ID, c.ID); return nil }); e != nil {
		return e
	}
	writeJSON(w, 200, map[string]any{"comment": c})
	return nil
}

// setReviewers grants draft visibility only; readVersion still enforces artifact sharing.
func (s *Server) setReviewers(w http.ResponseWriter, r *http.Request, p principal, a Artifact) error {
	if e := s.require(r, p, false, "share"); e != nil {
		return e
	}
	if !s.own(p, a) {
		return fail(403, "forbidden", "Only the owner or an administrator can assign reviewers")
	}
	var in struct {
		ReviewerIDs []string `json:"reviewerIds"`
	}
	if e := decode(r, &in); e != nil {
		return e
	}
	if len(in.ReviewerIDs) > 500 {
		return fail(400, "invalid_reviewers", "Choose at most 500 active reviewers")
	}
	ids := []string{}
	seen := map[string]bool{}
	for _, id := range in.ReviewerIDs {
		u, ok := s.st.Users[id]
		if !ok || u.Disabled {
			return fail(400, "invalid_reviewer", "Reviewers must be active members")
		}
		if !seen[id] {
			ids = append(ids, id)
			seen[id] = true
		}
	}
	if e := s.mutate(func() error {
		a.ReviewerIDs = ids
		s.st.Artifacts[a.ID] = a
		s.audit(p, "artifact.reviewers", a.ID, strings.Join(ids, ","))
		return nil
	}); e != nil {
		return e
	}
	writeJSON(w, 200, map[string]any{"artifact": s.publicArtifactFor(p, a)})
	return nil
}
