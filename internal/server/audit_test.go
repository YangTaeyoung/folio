package server

import (
	"encoding/json"
	"errors"
	"net/url"
	"strings"
	"testing"
	"time"
)

func auditGet(t *testing.T, c *client, query string) auditPage {
	t.Helper()
	code, _, w := c.call("GET", "/api/admin/audit"+query, nil)
	if code != 200 {
		t.Fatalf("audit: %d %s", code, w.Body)
	}
	var out auditPage
	if e := json.Unmarshal(w.Body.Bytes(), &out); e != nil {
		t.Fatal(e)
	}
	return out
}
func auditFind(t *testing.T, events []Audit, action string) Audit {
	t.Helper()
	for _, a := range events {
		if a.Action == action {
			return a
		}
	}
	t.Fatalf("missing %s in %v", action, events)
	return Audit{}
}
func auditChange(t *testing.T, a Audit, field string) AuditChange {
	t.Helper()
	for _, v := range a.Changes {
		if v.Field == field {
			return v
		}
	}
	t.Fatalf("missing %s in %+v", field, a)
	return AuditChange{}
}

func TestAuditSemanticSnapshotsSurviveRenameDeletionAndRestart(t *testing.T) {
	c := fresh(t)
	c.setup()
	code, out := c.upload("", "", "index.html", "<h1>private document body</h1>")
	if code != 201 {
		t.Fatal(code, out)
	}
	id := out["artifact"].(map[string]any)["id"].(string)
	if code, _, _ := c.call("PATCH", "/api/artifacts/"+id, map[string]any{"title": "Quarterly review", "description": "private description"}); code != 200 {
		t.Fatal(code)
	}
	if code, _, _ := c.call("PATCH", "/api/artifacts/"+id+"/sharing", map[string]any{"visibility": "organization", "memberIds": []string{}}); code != 200 {
		t.Fatal(code)
	}
	page := auditGet(t, c, "?category=sharing")
	shared := auditFind(t, page.Events, "artifact.sharing")
	change := auditChange(t, shared, "visibility")
	if change.Before != "private" || change.After != "organization" || shared.TargetName != "Quarterly review" || !strings.HasPrefix(shared.TargetURL, "/s/") {
		t.Fatalf("bad sharing snapshot %+v", shared)
	}
	c.call("PATCH", "/api/artifacts/"+id, map[string]any{"title": "Renamed later"})
	c.call("DELETE", "/api/artifacts/"+id, nil)
	if code, _, _ := c.call("DELETE", "/api/trash/"+id, nil); code != 204 {
		t.Fatal(code)
	}
	page = auditGet(t, c, "?category=workspace")
	shared = auditFind(t, page.Events, "artifact.sharing")
	if shared.TargetName != "Quarterly review" || shared.TargetURL != "" {
		t.Fatalf("historical target drift/dead link %+v", shared)
	}
	purged := auditFind(t, page.Events, "artifact.purged")
	if purged.TargetName != "Renamed later" || !purged.Snapshot {
		t.Fatalf("deleted name lost %+v", purged)
	}
	raw, _ := json.Marshal(page)
	if strings.Contains(string(raw), "private document body") || strings.Contains(string(raw), "private description") {
		t.Fatal("content leaked into audit")
	}
	cfg := c.s.cfg
	if e := c.s.Close(); e != nil {
		t.Fatal(e)
	}
	s, e := New(cfg)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { s.Close() })
	c.s = s
	shared = auditFind(t, auditGet(t, c, "?category=sharing").Events, "artifact.sharing")
	if shared.TargetName != "Quarterly review" || auditChange(t, shared, "visibility").Before != "private" {
		t.Fatalf("snapshot not persisted %+v", shared)
	}
}

func TestAuditFiltersApplyBeforeLimitAndCursor(t *testing.T) {
	c := fresh(t)
	c.setup()
	base := time.Date(2026, 1, 2, 10, 0, 0, 0, time.UTC)
	c.s.mu.Lock()
	c.s.st.Audit = []Audit{}
	for i := 0; i < 7; i++ {
		name := "Alice"
		actor := "alice"
		if i%2 != 0 {
			name = "Bob"
			actor = "bob"
		}
		c.s.st.Audit = append(c.s.st.Audit, Audit{ID: string(rune('a' + i)), Action: "artifact.updated", TargetName: "Plan", Snapshot: true, ActorID: actor, ActorName: name, CreatedAt: base.Add(time.Duration(i) * time.Hour)})
	}
	for i := 0; i < 130; i++ {
		c.s.st.Audit = append(c.s.st.Audit, Audit{ID: random(), Action: "login.google", CreatedAt: base.Add(24 * time.Hour)})
	}
	c.s.mu.Unlock()
	first := auditGet(t, c, "?category=workspace&limit=2")
	if first.Total != 7 || len(first.Events) != 2 || first.Events[0].ID != "g" || first.NextCursor == "" {
		t.Fatalf("filter applied after limit %+v", first)
	}
	second := auditGet(t, c, "?category=workspace&limit=2&cursor="+url.QueryEscape(first.NextCursor))
	if second.Total != 7 || second.Events[0].ID != "e" || second.Events[1].ID != "d" {
		t.Fatalf("cursor failed %+v", second)
	}
	filtered := auditGet(t, c, "?category=workspace&limit=1&actor=alice&q=plan&from=2026-01-02T12:00:00Z&to=2026-01-02T16:00:00Z")
	if filtered.Total != 2 || filtered.Events[0].ID != "e" {
		t.Fatalf("combined filters %+v", filtered)
	}
	legacy := auditGet(t, c, "?limit=2")
	if len(legacy.Events) != 2 || legacy.Events[0].Action != "login.google" {
		t.Fatal("legacy endpoint changed")
	}
	security := auditGet(t, c, "?category=security&limit=1000")
	if security.Total != 130 || len(security.Events) != 130 {
		t.Fatal("security events lost")
	}
	for _, query := range []string{"?category=bogus", "?from=bad", "?to=bad", "?cursor=%3F", "?cursor=dW5rbm93bg", "?from=2026-01-03T00:00:00Z&to=2026-01-02T00:00:00Z"} {
		if code, _, _ := c.call("GET", "/api/admin/audit"+query, nil); code != 400 {
			t.Fatalf("invalid filter %s accepted: %d", query, code)
		}
	}
}

func TestAuditAdminOnlyAndDeniedMutationsProduceNoSuccess(t *testing.T) {
	c := fresh(t)
	c.setup()
	member := c.member("member@test.invalid")
	code, _, _ := member.call("GET", "/api/admin/audit?category=workspace", nil)
	if code != 403 {
		t.Fatal("member read audit", code)
	}
	anonymous := &client{t: t, s: c.s}
	code, _, _ = anonymous.call("GET", "/api/admin/audit", nil)
	if code != 401 {
		t.Fatal("anonymous read audit", code)
	}
	_, out := c.upload("", "", "index.html", "<h1>test</h1>")
	id := out["artifact"].(map[string]any)["id"].(string)
	c.call("PATCH", "/api/artifacts/"+id+"/sharing", map[string]any{"visibility": "organization", "memberIds": []string{}})
	before := len(c.s.st.Audit)
	code, _, _ = member.call("PATCH", "/api/artifacts/"+id+"/sharing", map[string]any{"visibility": "public", "memberIds": []string{}})
	if code != 403 {
		t.Fatal("unauthorized change", code)
	}
	if len(c.s.st.Audit) != before {
		t.Fatal("denied mutation emitted success event")
	}
}

func TestAuditAllowlistAndFailedCommitRollback(t *testing.T) {
	c := fresh(t)
	c.setup()
	s := c.s
	s.mu.Lock()
	defer s.mu.Unlock()
	before := len(s.st.Audit)
	if err := s.mutate(func() error {
		s.st.Tokens["token-test"] = tokenRecord{Token: Token{ID: "token-test", Name: "Automation", Hash: "token-secret", Scopes: []string{"read"}}, Digest: "digest-secret"}
		s.audit(principal{}, "token.created", "token-test", "")
		s.st.GoogleAuth.Secret = "oauth-secret"
		s.st.GoogleAuth.Enabled = true
		s.audit(principal{}, "auth.google.updated", s.st.Organization.ID, "")
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(s.st.Audit[before:])
	for _, secret := range []string{"token-secret", "digest-secret", "oauth-secret", "passwordHash", "clientSecret"} {
		if strings.Contains(string(raw), secret) {
			t.Fatalf("audit secret leak %s", secret)
		}
	}
	count := len(s.st.Audit)
	name := s.st.Organization.Name
	err := s.mutate(func() error {
		s.st.Organization.Name = "Failed name"
		s.audit(principal{}, "settings.updated", s.st.Organization.ID, "")
		return errors.New("rejected")
	})
	if err == nil || len(s.st.Audit) != count || s.st.Organization.Name != name {
		t.Fatal("failed mutation audit not rolled back")
	}
	s.db.Close()
	err = s.mutate(func() error {
		s.st.Organization.Name = "Failed commit"
		s.audit(principal{}, "settings.updated", s.st.Organization.ID, "")
		return nil
	})
	if err == nil || len(s.st.Audit) != count || s.st.Organization.Name != name {
		t.Fatal("failed commit persisted success event")
	}
}

func TestAuditLegacyEventsDoNotInventHistoricalValues(t *testing.T) {
	c := fresh(t)
	c.setup()
	id := c.s.st.Sessions[digest(c.cookie.Value)].UserID
	c.s.mu.Lock()
	c.s.st.Audit = append(c.s.st.Audit, Audit{ID: "legacy", Action: "member.updated", TargetID: id, ActorName: "Historical actor", CreatedAt: now()})
	c.s.mu.Unlock()
	a := auditFind(t, auditGet(t, c, "?category=members").Events, "member.updated")
	if a.Snapshot || len(a.Changes) > 0 || !a.TargetNameCurrent || a.ActorName != "Historical actor" {
		t.Fatalf("legacy history fabricated %+v", a)
	}
}

func TestAuditMemberAndPolicyChangesAreAttributable(t *testing.T) {
	c := fresh(t)
	c.setup()
	c.member("policy-member@test.invalid")
	memberID := ""
	for id, u := range c.s.st.Users {
		if u.Email == "policy-member@test.invalid" {
			memberID = id
		}
	}
	if code, _, _ := c.call("PATCH", "/api/admin/members/"+memberID, map[string]any{"role": "admin", "name": "Reviewer", "disabled": true}); code != 200 {
		t.Fatal(code)
	}
	changed := auditFind(t, auditGet(t, c, "?category=members").Events, "member.updated")
	if auditChange(t, changed, "role").Before != "member" || auditChange(t, changed, "role").After != "admin" || auditChange(t, changed, "disabled").After != true || changed.ActorName != "Admin" || changed.TargetName != "Reviewer" {
		t.Fatalf("unattributable member change %+v", changed)
	}
	oldQuota := c.s.st.Policy.QuotaBytes
	policy := c.s.st.Policy
	policy.QuotaBytes = oldQuota + 1024
	if code, _, _ := c.call("PATCH", "/api/admin/settings", map[string]any{"organizationName": "Research team", "policy": policy}); code != 200 {
		t.Fatal(code)
	}
	changed = auditFind(t, auditGet(t, c, "?category=settings").Events, "settings.updated")
	change := auditChange(t, changed, "quotaBytes")
	if change.Before != float64(oldQuota) || change.After != float64(oldQuota+1024) || auditChange(t, changed, "name").Before != "Team" || changed.TargetName != "Research team" {
		t.Fatalf("policy snapshot %+v", changed)
	}
}

func TestAuditRestoredHistoryNeverReceivesInventedSnapshots(t *testing.T) {
	c := fresh(t)
	c.setup()
	s := c.s
	s.mu.Lock()
	defer s.mu.Unlock()
	// Recovery can replace the audit slice with a longer or shorter history.
	for _, count := range []int{8, 1} {
		if e := s.mutate(func() error {
			history := []Audit{}
			for i := 0; i < count; i++ {
				history = append(history, Audit{ID: random(), Action: "member.updated", TargetID: "legacy-member"})
			}
			s.st.Audit = history
			s.audit(principal{}, "backup.restored", "backup", "")
			return nil
		}); e != nil {
			t.Fatal(e)
		}
		for i, a := range s.st.Audit {
			if i == count {
				if !a.Snapshot {
					t.Fatal("recovery event not enriched")
				}
			} else if a.Snapshot || len(a.Changes) > 0 {
				t.Fatal("imported historical event was enriched")
			}
		}
	}
}

func TestAuditCollectionReorderPreservesBeforeAfterOrder(t *testing.T) {
	c := fresh(t)
	c.setup()
	_, first := c.upload("", "", "index.html", "<h1>First fixture</h1>")
	_, second := c.upload("", "", "index.html", "<h1>Second fixture</h1>")
	firstID := first["artifact"].(map[string]any)["id"].(string)
	secondID := second["artifact"].(map[string]any)["id"].(string)
	c.call("PATCH", "/api/artifacts/"+firstID, map[string]any{"title": "First fixture"})
	c.call("PATCH", "/api/artifacts/"+secondID, map[string]any{"title": "Second fixture"})
	code, out, _ := c.call("POST", "/api/collections", map[string]any{"name": "Ordered collection", "artifactIds": []string{firstID, secondID}})
	if code != 201 {
		t.Fatal(code, out)
	}
	id := out["collection"].(map[string]any)["id"].(string)
	if code, _, _ := c.call("PATCH", "/api/collections/"+id, map[string]any{"artifactIds": []string{secondID, firstID}}); code != 200 {
		t.Fatal(code)
	}
	changed := auditFind(t, auditGet(t, c, "?category=content").Events, "collection.saved")
	order := auditChange(t, changed, "artifacts")
	before := order.Before.([]any)
	after := order.After.([]any)
	if len(before) != 2 || len(after) != 2 || before[0] != "First fixture" || before[1] != "Second fixture" || after[0] != "Second fixture" || after[1] != "First fixture" {
		t.Fatalf("collection order erased %+v", order)
	}
}
