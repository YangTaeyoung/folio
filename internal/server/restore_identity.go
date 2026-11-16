package server

import (
	"errors"
	"strings"
)

// A restore into another installation may contain the same email with a different
// opaque ID. Reuse the current identity and credentials, remapping ownership and
// ACL references; never introduce an ambiguous duplicate sign-in identity.
func mergeRestoreIdentities(imported *state, current map[string]userRecord) error {
	byEmail := map[string]string{}
	for id, u := range current {
		byEmail[strings.ToLower(u.Email)] = id
	}
	remap := map[string]string{}
	for id, u := range imported.Users {
		if existing, ok := current[id]; ok && !strings.EqualFold(existing.Email, u.Email) {
			return errors.New("backup identity conflicts with a current user")
		}
		if currentID, ok := byEmail[strings.ToLower(u.Email)]; ok && currentID != id {
			remap[id] = currentID
			delete(imported.Users, id)
		}
	}
	for id, u := range current {
		imported.Users[id] = u
	}
	mapped := func(id string) string {
		if replacement, ok := remap[id]; ok {
			return replacement
		}
		return id
	}
	ids := func(in []string) []string {
		out := []string{}
		for _, id := range in {
			id = mapped(id)
			if !contains(out, id) {
				out = append(out, id)
			}
		}
		return out
	}
	for id, a := range imported.Artifacts {
		a.OwnerID = mapped(a.OwnerID)
		a.DeletedBy = mapped(a.DeletedBy)
		a.MemberIDs = ids(a.MemberIDs)
		a.ReviewerIDs = ids(a.ReviewerIDs)
		imported.Artifacts[id] = a
	}
	for id, c := range imported.Collections {
		c.OwnerID = mapped(c.OwnerID)
		imported.Collections[id] = c
	}
	for id, c := range imported.Comments {
		c.AuthorID = mapped(c.AuthorID)
		imported.Comments[id] = c
	}
	for id, r := range imported.Reviews {
		r.UpdatedBy = mapped(r.UpdatedBy)
		imported.Reviews[id] = r
	}
	fav := map[string]map[string]bool{}
	for uid, items := range imported.Favorites {
		uid = mapped(uid)
		if fav[uid] == nil {
			fav[uid] = map[string]bool{}
		}
		for aid, yes := range items {
			if yes {
				fav[uid][aid] = true
			}
		}
	}
	imported.Favorites = fav
	for i := range imported.Audit {
		imported.Audit[i].ActorID = mapped(imported.Audit[i].ActorID)
	}
	return nil
}
