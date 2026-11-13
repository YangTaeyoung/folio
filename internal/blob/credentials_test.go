package blob

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestGCSServiceAccountJSONUsesAuthenticatedTransport(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	var tokenCalls atomic.Int32
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("token method = %s", r.Method)
		}
		if err := r.ParseForm(); err != nil {
			t.Error(err)
		}
		if r.Form.Get("grant_type") != "urn:ietf:params:oauth:grant-type:jwt-bearer" || r.Form.Get("assertion") == "" {
			t.Error("missing service-account JWT grant")
		}
		tokenCalls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"access_token":"test-service-account-token","token_type":"Bearer","expires_in":3600}`))
	}))
	defer tokenServer.Close()
	encoded, err := json.Marshal(map[string]string{"type": "service_account", "project_id": "test-project", "private_key_id": "test-key", "private_key": string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: mustPKCS8(t, key)})), "client_email": "folio@test-project.iam.gserviceaccount.com", "client_id": "123", "token_uri": tokenServer.URL})
	if err != nil {
		t.Fatal(err)
	}
	e := newCloudEmulator(t, "gcs")
	c := e.config()
	c.ServiceAccountJSON = string(encoded)
	s, err := New(c)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Put(context.Background(), "file", strings.NewReader("x"), 1, "text/plain"); err != nil {
		t.Fatal(err)
	}
	if tokenCalls.Load() != 1 {
		t.Fatalf("token exchange calls = %d", tokenCalls.Load())
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, auth := range e.auth {
		if auth != "Bearer test-service-account-token" {
			t.Error("service account JSON did not select OAuth transport")
		}
	}
}
func mustPKCS8(t *testing.T, key *rsa.PrivateKey) []byte {
	t.Helper()
	b, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
