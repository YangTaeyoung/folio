package server

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"testing/fstest"
	"time"
)

func TestVersionOriginsBindRootAssetsAndConsumedGrantReceipts(t *testing.T) {
	c := fresh(t)
	c.setup()
	a, v1 := workspaceUpload(t, c, "", map[string]any{"title": "Absolute assets"}, map[string]string{"index.html": "<script src='/app.js'></script>", "app.js": "VERSION_ONE"})
	_, v2 := workspaceUpload(t, c, a.ID, map[string]any{"baseVersionId": v1.ID}, map[string]string{"index.html": "<script src='/app.js'></script>", "app.js": "VERSION_TWO"})
	cookies := map[string][]*http.Cookie{}
	grants := map[string]string{}
	for _, v := range []Version{v1, v2} {
		out := reviewHTTP(t, c, "POST", "/api/artifacts/"+a.ID+"/launch", map[string]any{"versionId": v.ID, "isolatedVersion": true}, 200)
		grant := out["url"].(string)
		grants[v.ID] = grant
		r := httptest.NewRequest("GET", grant, nil)
		w := httptest.NewRecorder()
		c.s.ContentHandler().ServeHTTP(w, r)
		if w.Code != 303 {
			t.Fatal("grant", w.Code, w.Body.String())
		}
		cookies[v.ID] = w.Result().Cookies()
	}
	if strings.Split(grants[v1.ID], "/grant")[0] == strings.Split(grants[v2.ID], "/grant")[0] {
		t.Fatal("versions share origin")
	}
	for i, v := range []Version{v1, v2} {
		u, _ := url.Parse(grants[v.ID])
		r := httptest.NewRequest("GET", u.Scheme+"://"+u.Host+"/app.js", nil)
		for _, co := range cookies[v.ID] {
			r.AddCookie(co)
		}
		w := httptest.NewRecorder()
		c.s.ContentHandler().ServeHTTP(w, r)
		want := "VERSION_ONE"
		if i == 1 {
			want = "VERSION_TWO"
		}
		if w.Code != 200 || w.Body.String() != want {
			t.Fatal("mixed absolute asset", w.Code, w.Body.String())
		}
		wrong := v2
		if v.ID == v2.ID {
			wrong = v1
		}
		rr := httptest.NewRequest("GET", u.Scheme+"://"+u.Host+"/render/"+a.ID+"/"+wrong.ID+"/app.js", nil)
		for _, co := range cookies[v.ID] {
			rr.AddCookie(co)
		}
		ww := httptest.NewRecorder()
		c.s.ContentHandler().ServeHTTP(ww, rr)
		if ww.Code != 404 {
			t.Fatal("crossversionhost", ww.Code)
		}
		for _, authenticated := range []bool{false, true} {
			rr := httptest.NewRequest("GET", grants[v.ID], nil)
			if authenticated {
				for _, co := range cookies[v.ID] {
					rr.AddCookie(co)
				}
			}
			ww := httptest.NewRecorder()
			c.s.ContentHandler().ServeHTTP(ww, rr)
			expected := 404
			if authenticated {
				expected = 303
			}
			if ww.Code != expected {
				t.Fatal("receipt", authenticated, ww.Code)
			}
			if len(ww.Result().Cookies()) != 0 {
				t.Fatal("receipt extended session")
			}
		}
	}
	reviewHTTP(t, c, "POST", "/api/artifacts/"+a.ID+"/launch", map[string]string{"hostname": "unverified.invalid"}, 400)
	reviewHTTP(t, c, "POST", "/api/auth/logout", map[string]string{}, 204)
	rr := httptest.NewRequest("GET", grants[v1.ID], nil)
	for _, co := range cookies[v1.ID] {
		rr.AddCookie(co)
	}
	ww := httptest.NewRecorder()
	c.s.ContentHandler().ServeHTTP(ww, rr)
	if ww.Code != 404 {
		t.Fatal("revoked receipt", ww.Code)
	}
}
func TestGuestSuccessfulUnlockBoundedAndExpiredSessionsCollected(t *testing.T) {
	c := fresh(t)
	c.setup()
	a := opsArtifact(t, c)
	p := opsAdmin(c)
	g := GuestLink{ID: random(), ArtifactID: a.ID, VersionID: a.CurrentVersionID, CreatedBy: p.user.ID, CreatedAt: now(), ExpiresAt: now().Add(time.Hour)}
	c.s.st.GuestLinks[g.ID] = g
	unlock := func(cookie *http.Cookie) *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", "http://app.test/api/guest/"+g.ID+"/unlock", strings.NewReader(`{"password":""}`))
		r.Header.Set("Origin", "http://app.test")
		if cookie != nil {
			r.AddCookie(cookie)
		}
		w := httptest.NewRecorder()
		c.s.Handler().ServeHTTP(w, r)
		return w
	}
	first := unlock(nil)
	if first.Code != 200 {
		t.Fatal(first.Code)
	}
	cookie := first.Result().Cookies()[0]
	for i := 0; i < 25; i++ {
		if w := unlock(cookie); w.Code != 200 {
			t.Fatal("session reuse", w.Code)
		}
	}
	if len(c.s.st.GuestSessions) != 1 {
		t.Fatal("reuse created rows")
	}
	for i := 0; i < 9; i++ {
		if w := unlock(nil); w.Code != 200 {
			t.Fatal("early limit", i, w.Code)
		}
	}
	if w := unlock(nil); w.Code != 429 {
		t.Fatal("successful creates unlimited", w.Code)
	}
	for k, se := range c.s.st.GuestSessions {
		se.ExpiresAt = now().Add(-time.Hour)
		c.s.st.GuestSessions[k] = se
	}
	if _, e := c.s.Maintenance(t.Context(), false); e != nil {
		t.Fatal(e)
	}
	if len(c.s.st.GuestSessions) != 0 {
		t.Fatal("expired guest sessions retained")
	}
}
func TestGuestSPAFallbackIsHTML(t *testing.T) {
	c := fresh(t)
	c.s.cfg.WebFS = fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte("<!doctype html><title>Folio</title>")}}
	w := httptest.NewRecorder()
	c.s.Handler().ServeHTTP(w, httptest.NewRequest("GET", "http://app.test/g/example", nil))
	if w.Code != 200 || !strings.HasPrefix(w.Header().Get("Content-Type"), "text/html") {
		t.Fatal(w.Code, w.Header())
	}
}
