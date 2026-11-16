package server

import "time"

// GuestLink is persisted privately. guestLinkPublic strips its password verifier.
type GuestLink struct {
	ID           string     `json:"id"`
	ArtifactID   string     `json:"artifactId"`
	VersionID    string     `json:"versionId"`
	CreatedBy    string     `json:"createdBy"`
	CreatedAt    time.Time  `json:"createdAt"`
	ExpiresAt    time.Time  `json:"expiresAt"`
	RevokedAt    *time.Time `json:"revokedAt,omitempty"`
	PasswordHash string     `json:"passwordHash,omitempty"`
}
type GuestSession struct {
	LinkID    string
	CSRF      string
	ExpiresAt time.Time
}
type BackupInfo struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"createdAt"`
	SizeBytes int64     `json:"sizeBytes"`
	Scope     string    `json:"scope"`
}
