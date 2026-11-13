package blob

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func TestLocalLifecycle(t *testing.T) {
	root := t.TempDir()
	s, err := New(Config{Provider: "local", Root: root, Prefix: "/tenant/artifacts/"})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := s.Put(ctx, "v1/index.html", strings.NewReader("hello"), 5, "text/html"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, "tenant/artifacts/v1/index.html"))
	if err != nil || string(data) != "hello" {
		t.Fatalf("data=%q err=%v", data, err)
	}
	r, err := s.Open(ctx, "v1/index.html")
	if err != nil {
		t.Fatal(err)
	}
	got, err := io.ReadAll(r)
	r.Close()
	if err != nil || string(got) != "hello" {
		t.Fatalf("read %q: %v", got, err)
	}
	if err := s.Test(ctx); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete(ctx, "v1/index.html"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Open(ctx, "v1/index.html"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("missing: %v", err)
	}
	if err := s.Delete(ctx, "v1/index.html"); err != nil {
		t.Fatalf("idempotent delete: %v", err)
	}
}

func TestLocalRejectsTraversalAndSymlinks(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	s, err := New(Config{Provider: "local", Root: root})
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"", "/abs", "../outside", "a/../b", "a//b", "a/./b", `a\b`, "a\x00b", "a/%2e%2e/b", "a/b?x", "a/b#x", "a/b:", "a/"} {
		if err := s.Put(context.Background(), key, strings.NewReader("x"), 1, "text/plain"); err == nil {
			t.Errorf("accepted %q", key)
		}
		if r, err := s.Open(context.Background(), key); err == nil {
			r.Close()
			t.Errorf("opened %q", key)
		}
		if err := s.Delete(context.Background(), key); err == nil {
			t.Errorf("deleted %q", key)
		}
	}
	if err := os.Symlink(outside, filepath.Join(root, "escape")); err != nil {
		t.Fatal(err)
	}
	if err := s.Put(context.Background(), "escape/leak", strings.NewReader("x"), 1, "text/plain"); err == nil {
		t.Fatal("symlink escaped root")
	}
	if _, err := os.Stat(filepath.Join(outside, "leak")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("outside write: %v", err)
	}
}

func TestPutSizeAndCancellationDoNotReplaceLocalObject(t *testing.T) {
	s, err := New(Config{Provider: "local", Root: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := s.Put(ctx, "file", strings.NewReader("old"), 3, "text/plain"); err != nil {
		t.Fatal(err)
	}
	for _, body := range []string{"x", "too long"} {
		if err := s.Put(ctx, "file", strings.NewReader(body), 3, "text/plain"); err == nil {
			t.Fatal("size mismatch accepted")
		}
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if err := s.Put(canceled, "file", bytes.NewReader([]byte("new")), 3, "text/plain"); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel: %v", err)
	}
	r, err := s.Open(ctx, "file")
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	b, _ := io.ReadAll(r)
	if string(b) != "old" {
		t.Fatalf("replaced: %q", b)
	}
}

func TestInvalidConfigNeverLeaksSecrets(t *testing.T) {
	for _, c := range []Config{
		{Provider: "unknown", SecretAccessKey: "SENTINEL_SECRET"},
		{Provider: "local"},
		{Provider: "local", Root: t.TempDir(), Prefix: "../escape"},
		{Provider: "s3", Bucket: "b", AccessKeyID: "id", SecretAccessKey: ""},
		{Provider: "s3", Bucket: "b", Endpoint: "http://user:SENTINEL_SECRET@localhost"},
		{Provider: "gcs", Bucket: "b", ServiceAccountJSON: "SENTINEL_SECRET"},
		{Provider: "azure", Bucket: "b", AccountName: "account", AccountKey: "SENTINEL_SECRET"},
	} {
		_, err := New(c)
		if err == nil {
			t.Errorf("accepted invalid provider %q", c.Provider)
			continue
		}
		if strings.Contains(err.Error(), "SENTINEL_SECRET") {
			t.Fatalf("secret leak: %v", err)
		}
	}
}

func TestLocalDoesNotConsultCloudServices(t *testing.T) {
	// Cloud settings and credential-provider environment must be irrelevant to
	// offline storage. An explicit endpoint traps accidental SDK requests.
	var calls atomic.Int32
	endpoint := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1); w.WriteHeader(500) }))
	defer endpoint.Close()
	t.Setenv("AWS_EC2_METADATA_SERVICE_ENDPOINT", endpoint.URL)
	t.Setenv("STORAGE_EMULATOR_HOST", endpoint.URL)
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", "/nonexistent/folio-credential.json")
	s, err := New(Config{Provider: "local", Root: t.TempDir(), Endpoint: endpoint.URL, AccessKeyID: "incomplete", ServiceAccountJSON: "malformed", ConnectionString: "malformed"})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Test(context.Background()); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 0 {
		t.Fatal("local storage contacted a cloud service")
	}
}

func TestCloudBucketCannotInjectAPath(t *testing.T) {
	for _, provider := range []string{"s3", "gcs", "azure"} {
		for _, bucket := range []string{"../other", "a/b", `a\b`, "a?x", "a#x", "a%2fb", "a\n"} {
			c := Config{Provider: provider, Bucket: bucket, Endpoint: "http://127.0.0.1:1", AccessKeyID: "id", SecretAccessKey: "secret", AccountName: "folioaccount", AccountKey: "dGVzdA=="}
			if _, err := New(c); err == nil {
				t.Errorf("%s accepted bucket %q", provider, bucket)
			}
		}
	}
}
