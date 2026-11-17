package server

import "testing"

func TestProfileOwnerPhotoPrivateProjection(t *testing.T) {
	c := fresh(t)
	c.setup()
	owner := c.member("owner-photo@test.invalid")
	if w := owner.photo("PUT", "/api/me/photo", "image/png", photoBytes(t, "png", 16, 16)); w.Code != 200 {
		t.Fatal(w.Code, w.Body)
	}
	var uid string
	for id, u := range c.s.st.Users {
		if u.Email == "owner-photo@test.invalid" {
			uid = id
		}
	}
	c.s.mu.Lock()
	defer c.s.mu.Unlock()
	u := c.s.st.Users[uid].User
	a := Artifact{OwnerID: uid, OwnerAvatarURL: "https://untrusted.invalid/backup-value"}
	member := c.s.publicArtifactFor(principal{user: &u}, a)
	if member.OwnerAvatarURL == "" || member.OwnerAvatarURL != u.AvatarURL {
		t.Fatal("member projection lost photo", member.OwnerAvatarURL)
	}
	if c.s.publicArtifactFor(principal{}, a).OwnerAvatarURL != "" {
		t.Fatal("anonymous photo metadata must stay private")
	}
	if c.s.publicArtifact(a).OwnerAvatarURL != "" {
		t.Fatal("untrusted stored photo URL retained")
	}
	record := c.s.st.Users[uid]
	record.Disabled = true
	c.s.st.Users[uid] = record
	if c.s.publicArtifactFor(principal{user: &u}, a).OwnerAvatarURL != "" {
		t.Fatal("disabled owner's photo remains visible")
	}
}
