package server

import "time"

// Provenance is deliberately explicit; Folio never captures a chat or environment.
type Provenance struct {
	Agent      string   `json:"agent,omitempty"`
	Model      string   `json:"model,omitempty"`
	Summary    string   `json:"summary,omitempty"`
	Repository string   `json:"repository,omitempty"`
	Commit     string   `json:"commit,omitempty"`
	TaskURL    string   `json:"taskUrl,omitempty"`
	Sources    []string `json:"sources,omitempty"`
}
type Collection struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	OwnerID     string    `json:"ownerId"`
	Visibility  string    `json:"visibility"`
	ArtifactIDs []string  `json:"artifactIds"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

func (s *Server) initFeatureState() {
	for aid, versions := range s.st.Versions {
		a, ok := s.st.Artifacts[aid]
		if !ok {
			continue
		}
		if !a.ReviewEnabled {
			if a.PublishedVersionID == "" {
				a.PublishedVersionID = a.CurrentVersionID
				s.st.Artifacts[aid] = a
			}
		}
		for i := range versions {
			if !a.ReviewEnabled {
				versions[i].Published = true
			}
			if !versions[i].MetadataSnapshot {
				versions[i].Title = a.Title
				versions[i].Description = a.Description
				versions[i].Tags = append([]string{}, a.Tags...)
				versions[i].MetadataSnapshot = true
			}
		}
		s.st.Versions[aid] = versions
	}
	if s.st.Collections == nil {
		s.st.Collections = map[string]Collection{}
	}
	if s.st.Favorites == nil {
		s.st.Favorites = map[string]map[string]bool{}
	}
	if s.st.Reviews == nil {
		s.st.Reviews = map[string]Review{}
	}
	if s.st.Comments == nil {
		s.st.Comments = map[string]Comment{}
	}
	if s.st.GuestLinks == nil {
		s.st.GuestLinks = map[string]GuestLink{}
	}
	if s.st.GuestSessions == nil {
		s.st.GuestSessions = map[string]GuestSession{}
	}
}
func (s *Server) readVersion(p principal, a Artifact, v Version) bool {
	if a.DeletedAt != nil || v.ArtifactID != a.ID {
		return false
	}
	if p.kind == "guest" {
		return s.guestCanRead(p, a, v.ID)
	}
	return s.read(p, a) && (s.own(p, a) || s.draftReviewer(p, a) || !a.ReviewEnabled || v.Published)
}
func (s *Server) effectiveVersionID(p principal, a Artifact) string {
	if !s.own(p, a) && !s.draftReviewer(p, a) && a.ReviewEnabled {
		return a.PublishedVersionID
	}
	return a.CurrentVersionID
}
func (s *Server) publicArtifactFor(p principal, a Artifact) Artifact {
	a = s.publicArtifact(a)
	if p.user != nil {
		if owner, ok := s.st.Users[a.OwnerID]; ok && !owner.Disabled {
			a.OwnerAvatarURL = owner.AvatarURL
		}
	}
	if !s.own(p, a) && !s.draftReviewer(p, a) && a.ReviewEnabled {
		a.CurrentVersionID = a.PublishedVersionID
		if v, ok := s.version(a, a.PublishedVersionID); ok {
			a.SizeBytes = v.SizeBytes
			a.UpdatedAt = v.CreatedAt
			if v.MetadataSnapshot {
				a.Title = v.Title
				a.Description = v.Description
				a.Tags = append([]string{}, v.Tags...)
			}
			a.Provenance = v.Provenance
		}
	}
	a.Tags = append([]string{}, a.Tags...)
	if p.user != nil {
		a.Favorite = s.st.Favorites[p.user.ID][a.ID]
	}
	return a
}

func (s *Server) draftReviewer(p principal, a Artifact) bool {
	return p.user != nil && contains(a.ReviewerIDs, p.user.ID)
}
