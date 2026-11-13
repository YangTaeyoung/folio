package server

import (
	"io"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/YangTaeyoung/folio/internal/blob"
)

type observedStore struct {
	blob.Store
	mu     sync.Mutex
	closed int
}

func (s *observedStore) Close() error {
	s.mu.Lock()
	s.closed++
	s.mu.Unlock()
	if closer, ok := s.Store.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}
func (s *observedStore) closeCount() int { s.mu.Lock(); defer s.mu.Unlock(); return s.closed }
func TestStorageClientsReuseAcrossAssetsAndCloseOnDeletionAndShutdown(t *testing.T) {
	c := fresh(t)
	c.setup()
	var mu sync.Mutex
	created := map[string][]*observedStore{}
	c.s.factory = func(cfg blob.Config) (blob.Store, error) {
		base, e := blob.New(cfg)
		if e != nil {
			return nil, e
		}
		store := &observedStore{Store: base}
		mu.Lock()
		created[cfg.ID] = append(created[cfg.ID], store)
		mu.Unlock()
		return store, nil
	}
	code, out := c.publishFiles("", "", []string{"index.html", "app.js"}, []string{`<script src="./app.js"></script>`, `document.body.dataset.ready="yes"`})
	if code != 201 {
		t.Fatal(code, out)
	}
	a := out["artifact"].(map[string]any)
	localID := out["version"].(map[string]any)["backendId"].(string)
	code, _, _ = c.call("PATCH", "/api/artifacts/"+a["id"].(string)+"/sharing", map[string]any{"visibility": "public", "memberIds": []string{}, "expiresAt": nil})
	if code != 200 {
		t.Fatal(code)
	}
	for range 4 {
		for _, path := range []string{"/index.html", "/app.js"} {
			req := httptest.NewRequest("GET", "http://"+a["slug"].(string)+".content.test"+path, nil)
			w := httptest.NewRecorder()
			c.s.ContentHandler().ServeHTTP(w, req)
			if w.Code != 200 {
				t.Fatal(w.Code, w.Body)
			}
		}
	}
	code, out, _ = c.call("POST", "/api/storage/"+localID+"/test", map[string]any{})
	if code != 200 || out["ok"] != true {
		t.Fatal(code, out)
	}
	if len(created[localID]) != 1 || created[localID][0].closeCount() != 0 {
		t.Fatal("asset/probe calls rebuilt or closed active storage client", created[localID])
	}
	code, out, _ = c.call("POST", "/api/storage", map[string]any{"name": "Ephemeral then cached", "provider": "local"})
	if code != 201 {
		t.Fatal(code, out)
	}
	otherID := out["backend"].(map[string]any)["id"].(string)
	if len(created[otherID]) != 1 || created[otherID][0].closeCount() != 1 {
		t.Fatal("validation client was not released")
	}
	code, out, _ = c.call("POST", "/api/storage/"+otherID+"/test", map[string]any{})
	if code != 200 || out["ok"] != true {
		t.Fatal(code, out)
	}
	code, out, _ = c.call("POST", "/api/storage/"+otherID+"/test", map[string]any{})
	if code != 200 || len(created[otherID]) != 2 || created[otherID][1].closeCount() != 0 {
		t.Fatal("backend probes did not reuse the cached client", code, out)
	}
	code, _, _ = c.call("DELETE", "/api/storage/"+otherID, nil)
	if code != 204 || created[otherID][1].closeCount() != 1 {
		t.Fatal("backend deletion did not release cached client", code)
	}
	if e := c.s.Close(); e != nil {
		t.Fatal(e)
	}
	if created[localID][0].closeCount() != 1 || created[otherID][1].closeCount() != 1 {
		t.Fatal("shutdown did not release exactly the remaining owned clients")
	}
	if e := c.s.Close(); e != nil {
		t.Fatal(e)
	}
	if created[localID][0].closeCount() != 1 {
		t.Fatal("idempotent server close repeated client close")
	}
}

func TestClosingDuringUploadDoesNotReopenStorage(t *testing.T) {
	c, f := freshFault(t)
	calls := 0
	var observed *observedStore
	c.s.factory = func(blob.Config) (blob.Store, error) {
		calls++
		observed = &observedStore{Store: f}
		return observed, nil
	}
	code, out := c.upload("", "", "index.html", "old")
	if code != 201 {
		t.Fatal(code, out)
	}
	a := out["artifact"].(map[string]any)
	f.mu.Lock()
	f.putStarted = make(chan struct{}, 1)
	f.putContinue = make(chan struct{})
	f.mu.Unlock()
	result := make(chan int, 1)
	go func() {
		code, _ := c.upload(a["id"].(string), a["currentVersionId"].(string), "index.html", "new")
		result <- code
	}()
	<-f.putStarted
	if e := c.s.Close(); e != nil {
		t.Fatal(e)
	}
	close(f.putContinue)
	if code := <-result; code != 503 {
		t.Fatal("publication continued after shutdown", code)
	}
	if calls != 1 || len(c.s.stores) != 0 || observed.closeCount() != 1 {
		t.Fatal("closed server reopened storage resources", calls, len(c.s.stores), observed.closeCount())
	}
}
