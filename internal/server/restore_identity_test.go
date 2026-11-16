package server

import "testing"

func TestRestoreReusesCurrentEmailIdentityAcrossInstallations(t *testing.T) {
	current := map[string]userRecord{"current": {User: User{ID: "current", Email: "admin@example.test", Role: "admin"}, Hash: "current-credential"}}
	st := state{Users: map[string]userRecord{"old": {User: User{ID: "old", Email: "admin@example.test", Role: "member"}, Hash: "old-credential"}}, Artifacts: map[string]Artifact{"a": {OwnerID: "old", MemberIDs: []string{"old"}, ReviewerIDs: []string{"old"}}}, Collections: map[string]Collection{"c": {OwnerID: "old"}}, Comments: map[string]Comment{"comment": {AuthorID: "old"}}, Reviews: map[string]Review{"v": {UpdatedBy: "old"}}, Favorites: map[string]map[string]bool{"old": {"a": true}}}
	if e := mergeRestoreIdentities(&st, current); e != nil {
		t.Fatal(e)
	}
	if len(st.Users) != 1 || st.Users["current"].Hash != "current-credential" || st.Artifacts["a"].OwnerID != "current" || st.Artifacts["a"].ReviewerIDs[0] != "current" || st.Comments["comment"].AuthorID != "current" || !st.Favorites["current"]["a"] {
		t.Fatal("identity/permissions not preserved", st)
	}
}
