package blob

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"io/fs"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

type storedObject struct {
	body        []byte
	contentType string
}
type cloudEmulator struct {
	t        *testing.T
	provider string
	server   *httptest.Server
	mu       sync.Mutex
	objects  map[string]storedObject
	blocks   map[string][]byte
	seen     []string
	auth     []string
	tokens   []string
	failRead bool
	deny     bool
	corrupt  bool
	stream   bool
}

func newCloudEmulator(t *testing.T, provider string) *cloudEmulator {
	e := &cloudEmulator{t: t, provider: provider, objects: map[string]storedObject{}, blocks: map[string][]byte{}}
	e.server = httptest.NewServer(http.HandlerFunc(e.serve))
	t.Cleanup(e.server.Close)
	return e
}
func (e *cloudEmulator) config() Config {
	c := Config{Provider: e.provider, Bucket: "private", Prefix: "/tenant/folio/", Endpoint: e.server.URL}
	switch e.provider {
	case "s3":
		c.AccessKeyID = "EXPLICIT_ID"
		c.SecretAccessKey = "EXPLICIT_SECRET"
		c.SessionToken = "EXPLICIT_SESSION"
		c.Region = "auto"
	case "azure":
		c.AccountName = "folioaccount"
		c.AccountKey = base64.StdEncoding.EncodeToString([]byte("test-only-key"))
	}
	return c
}
func (e *cloudEmulator) serve(w http.ResponseWriter, r *http.Request) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.seen = append(e.seen, r.Method+" "+r.URL.Path)
	e.auth = append(e.auth, r.Header.Get("Authorization"))
	e.tokens = append(e.tokens, r.Header.Get("X-Amz-Security-Token"))
	if e.deny {
		w.Header().Set("Content-Type", "application/xml")
		w.Header().Set("x-ms-error-code", "AuthorizationFailure")
		w.WriteHeader(403)
		io.WriteString(w, `<Error><Code>AccessDenied</Code><Message>EXPLICIT_SECRET</Message></Error>`)
		return
	}
	if e.provider == "gcs" {
		e.gcs(w, r)
		return
	}
	key := strings.TrimPrefix(r.URL.Path, "/private/")
	if !strings.HasPrefix(r.URL.Path, "/private/") {
		e.t.Errorf("unexpected bucket path %q", r.URL.Path)
		w.WriteHeader(400)
		return
	}
	switch r.Method {
	case "PUT":
		content, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(400)
			return
		}
		if e.provider == "azure" && r.URL.Query().Get("comp") == "block" {
			e.blocks[r.URL.Query().Get("blockid")] = content
			w.WriteHeader(201)
			return
		}
		if e.provider == "azure" && r.URL.Query().Get("comp") == "blocklist" {
			var list struct {
				Latest []string `xml:"Latest"`
			}
			if err := xml.Unmarshal(content, &list); err != nil {
				e.t.Error(err)
				w.WriteHeader(400)
				return
			}
			content = nil
			for _, id := range list.Latest {
				content = append(content, e.blocks[id]...)
			}
		}
		ct := r.Header.Get("Content-Type")
		if e.provider == "azure" {
			ct = r.Header.Get("x-ms-blob-content-type")
		}
		if r.Header.Get("x-amz-acl") != "" || r.Header.Get("x-ms-blob-public-access") != "" {
			e.t.Error("public/access policy header supplied")
		}
		e.objects[key] = storedObject{body: content, contentType: ct}
		w.Header().Set("ETag", `"test"`)
		if e.provider == "azure" {
			w.WriteHeader(201)
		}
	case "GET":
		e.read(w, r, key)
	case "DELETE":
		delete(e.objects, key)
		if e.provider == "azure" {
			w.WriteHeader(202)
		} else {
			w.WriteHeader(204)
		}
	default:
		e.t.Errorf("unexpected method %s", r.Method)
		w.WriteHeader(400)
	}
}
func (e *cloudEmulator) read(w http.ResponseWriter, r *http.Request, key string) {
	if e.failRead {
		w.WriteHeader(400)
		io.WriteString(w, "EXPLICIT_SECRET")
		return
	}
	o, ok := e.objects[key]
	if !ok {
		w.Header().Set("x-ms-error-code", "BlobNotFound")
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(404)
		io.WriteString(w, `<Error><Code>NoSuchKey</Code></Error>`)
		return
	}
	b := o.body
	if e.corrupt {
		b = []byte("corrupted")
	}
	w.Header().Set("Content-Type", o.contentType)
	w.Header().Set("Content-Length", strconv.Itoa(len(b)))
	w.Header().Set("ETag", `"test"`)
	w.WriteHeader(200)
	if e.stream {
		// Hold the body after sending headers. Releasing the emulator mutex here
		// permits cancellation and close to be observed by the request context.
		w.(http.Flusher).Flush()
		e.mu.Unlock()
		<-r.Context().Done()
		e.mu.Lock()
		return
	}
	w.Write(b)
}
func (e *cloudEmulator) gcs(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" && strings.Contains(r.URL.Path, "/upload/") {
		kind := r.URL.Query().Get("uploadType")
		if kind == "multipart" {
			_, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
			if err != nil {
				e.t.Error(err)
				w.WriteHeader(400)
				return
			}
			mr := multipart.NewReader(r.Body, params["boundary"])
			metaPart, err := mr.NextPart()
			if err != nil {
				e.t.Error(err)
				return
			}
			var meta struct {
				Name        string `json:"name"`
				ContentType string `json:"contentType"`
			}
			if err = json.NewDecoder(metaPart).Decode(&meta); err != nil {
				e.t.Error(err)
				return
			}
			dataPart, err := mr.NextPart()
			if err != nil {
				e.t.Error(err)
				return
			}
			data, err := io.ReadAll(dataPart)
			if err != nil {
				e.t.Error(err)
				return
			}
			e.objects[meta.Name] = storedObject{body: data, contentType: meta.ContentType}
			e.gcsObject(w, meta.Name, data)
			return
		}
		if kind == "resumable" {
			var meta struct {
				Name        string `json:"name"`
				ContentType string `json:"contentType"`
			}
			if err := json.NewDecoder(r.Body).Decode(&meta); err != nil {
				e.t.Error(err)
				w.WriteHeader(400)
				return
			}
			e.objects[meta.Name] = storedObject{contentType: meta.ContentType}
			w.Header().Set("Location", e.server.URL+"/resumable/"+meta.Name)
			w.WriteHeader(200)
			return
		}
		e.t.Errorf("unknown upload type %q", kind)
		w.WriteHeader(400)
		return
	}
	if r.Method == "POST" && strings.HasPrefix(r.URL.Path, "/resumable/") {
		key := strings.TrimPrefix(r.URL.Path, "/resumable/")
		data, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(400)
			return
		}
		obj := e.objects[key]
		obj.body = append(obj.body, data...)
		e.objects[key] = obj
		if strings.HasSuffix(r.Header.Get("Content-Range"), "/*") {
			w.Header().Set("Range", fmt.Sprintf("bytes=0-%d", len(obj.body)-1))
			w.Header().Set("X-Http-Status-Code-Override", "308")
			w.WriteHeader(200)
			return
		}
		e.gcsObject(w, key, obj.body)
		return
	}
	key := strings.TrimPrefix(r.URL.Path, "/storage/v1/b/private/o/")
	if key == r.URL.Path {
		e.t.Errorf("unexpected GCS path %s %s", r.Method, r.URL.Path)
		w.WriteHeader(400)
		return
	}
	switch r.Method {
	case "GET":
		if _, ok := e.objects[key]; !ok {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(404)
			io.WriteString(w, `{"error":{"code":404,"message":"Not Found"}}`)
			return
		}
		e.read(w, r, key)
	case "DELETE":
		delete(e.objects, key)
		w.WriteHeader(204)
	default:
		e.t.Errorf("unexpected GCS request %s %s", r.Method, r.URL)
		w.WriteHeader(400)
	}
}
func (e *cloudEmulator) gcsObject(w http.ResponseWriter, key string, data []byte) {
	sum := crc32.Checksum(data, crc32.MakeTable(crc32.Castagnoli))
	checksum := []byte{byte(sum >> 24), byte(sum >> 16), byte(sum >> 8), byte(sum)}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"kind": "storage#object", "bucket": "private", "name": key, "size": strconv.Itoa(len(data)), "generation": "1", "crc32c": base64.StdEncoding.EncodeToString(checksum)})
}

func TestCloudSDKRoundTrips(t *testing.T) {
	for _, provider := range []string{"s3", "gcs", "azure"} {
		t.Run(provider, func(t *testing.T) {
			e := newCloudEmulator(t, provider)
			s, err := New(e.config())
			if err != nil {
				t.Fatal(err)
			}
			// Exposing only Read prevents the adapters from depending on a seekable body.
			data := []byte("<!doctype html><p>private bytes</p>")
			if err := s.Put(context.Background(), "version/index.html", struct{ io.Reader }{bytes.NewReader(data)}, int64(len(data)), "text/html; charset=utf-8"); err != nil {
				t.Fatal(err)
			}
			e.mu.Lock()
			obj, ok := e.objects["tenant/folio/version/index.html"]
			e.mu.Unlock()
			if !ok || !bytes.Equal(obj.body, data) || obj.contentType != "text/html; charset=utf-8" {
				t.Fatalf("stored object = %#v, exists %v", obj, ok)
			}
			r, err := s.Open(context.Background(), "version/index.html")
			if err != nil {
				t.Fatal(err)
			}
			got, err := io.ReadAll(r)
			if closeErr := r.Close(); closeErr != nil {
				t.Fatal(closeErr)
			}
			if err != nil || !bytes.Equal(got, data) {
				t.Fatalf("read %q: %v", got, err)
			}
			if err := s.Test(context.Background()); err != nil {
				t.Fatal(err)
			}
			e.mu.Lock()
			count := len(e.objects)
			e.mu.Unlock()
			if count != 1 {
				t.Fatalf("probe leaked; %d objects", count)
			}
			if err := s.Delete(context.Background(), "version/index.html"); err != nil {
				t.Fatal(err)
			}
			if _, err := s.Open(context.Background(), "version/index.html"); !errors.Is(err, fs.ErrNotExist) {
				t.Fatalf("missing mapping: %v", err)
			}
			if err := s.Delete(context.Background(), "version/index.html"); err != nil {
				t.Fatal(err)
			}
			e.mu.Lock()
			defer e.mu.Unlock()
			for _, auth := range e.auth {
				switch provider {
				case "s3":
					if !strings.Contains(auth, "Credential=EXPLICIT_ID/") {
						t.Errorf("explicit S3 credential not selected")
					}
				case "azure":
					if !strings.HasPrefix(auth, "SharedKey folioaccount:") {
						t.Errorf("Azure shared key missing")
					}
				case "gcs":
					if auth != "" {
						t.Errorf("loopback emulator unexpectedly authenticated")
					}
				}
			}
			if provider == "s3" {
				for _, token := range e.tokens {
					if token != "EXPLICIT_SESSION" {
						t.Errorf("session token not passed")
					}
				}
			}
		})
	}
}
func TestCloudErrorsRedactAndProbeCleans(t *testing.T) {
	for _, provider := range []string{"s3", "gcs", "azure"} {
		t.Run(provider, func(t *testing.T) {
			e := newCloudEmulator(t, provider)
			s, err := New(e.config())
			if err != nil {
				t.Fatal(err)
			}
			e.failRead = true
			if err := s.Test(context.Background()); err == nil || strings.Contains(err.Error(), "EXPLICIT_SECRET") {
				t.Fatalf("read error = %v", err)
			}
			e.mu.Lock()
			count := len(e.objects)
			e.failRead = false
			e.corrupt = true
			e.mu.Unlock()
			if count != 0 {
				t.Fatal("failed probe leaked")
			}
			if err := s.Test(context.Background()); err == nil {
				t.Fatal("probe accepted different bytes")
			}
			e.mu.Lock()
			count = len(e.objects)
			e.corrupt = false
			e.deny = true
			e.mu.Unlock()
			if count != 0 {
				t.Fatal("corrupt probe leaked")
			}
			if err := s.Put(context.Background(), "file", strings.NewReader("x"), 1, "text/plain"); err == nil || strings.Contains(err.Error(), "EXPLICIT_SECRET") {
				t.Fatalf("denied error = %v", err)
			}
		})
	}
}
func TestCloudOpenRetainsContextUntilClose(t *testing.T) {
	for _, provider := range []string{"s3", "gcs", "azure"} {
		t.Run(provider, func(t *testing.T) {
			e := newCloudEmulator(t, provider)
			e.objects["tenant/folio/file"] = storedObject{body: []byte("hello"), contentType: "text/plain"}
			e.stream = true
			s, err := New(e.config())
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			r, err := s.Open(ctx, "file")
			if err != nil {
				t.Fatal(err)
			}
			done := make(chan error, 1)
			go func() { _, err := io.ReadAll(r); done <- err }()
			select {
			case err := <-done:
				t.Fatalf("body context canceled before close: %v", err)
			case <-time.After(25 * time.Millisecond):
			}
			cancel()
			select {
			case err := <-done:
				if err == nil {
					t.Fatal("canceled read succeeded")
				}
			case <-time.After(time.Second):
				t.Fatal("cancel did not interrupt body")
			}
			r.Close()
			r.Close()
		})
	}
}

func TestCloudRejectsBadKeysBeforeNetwork(t *testing.T) {
	for _, provider := range []string{"s3", "gcs", "azure"} {
		t.Run(provider, func(t *testing.T) {
			e := newCloudEmulator(t, provider)
			s, err := New(e.config())
			if err != nil {
				t.Fatal(err)
			}
			for _, key := range []string{"../file", "/absolute", "a/../file", "a%2fb", "a\\b"} {
				if err := s.Put(context.Background(), key, strings.NewReader("x"), 1, "text/plain"); err == nil {
					t.Errorf("accepted %q", key)
				}
				if _, err := s.Open(context.Background(), key); err == nil {
					t.Errorf("opened %q", key)
				}
				if err := s.Delete(context.Background(), key); err == nil {
					t.Errorf("deleted %q", key)
				}
			}
			e.mu.Lock()
			defer e.mu.Unlock()
			if len(e.seen) != 0 {
				t.Fatalf("invalid keys caused %d network requests", len(e.seen))
			}
		})
	}
}
func TestCloudSizeMismatch(t *testing.T) {
	for _, provider := range []string{"s3", "gcs", "azure"} {
		t.Run(provider, func(t *testing.T) {
			e := newCloudEmulator(t, provider)
			s, err := New(e.config())
			if err != nil {
				t.Fatal(err)
			}
			for _, body := range []string{"x", "too long"} {
				if err := s.Put(context.Background(), "file", strings.NewReader(body), 3, "text/plain"); err == nil {
					t.Errorf("accepted mismatch %q", body)
				}
			}
		})
	}
}
func TestCloudLargeStreamingUploads(t *testing.T) {
	for _, provider := range []string{"s3", "gcs", "azure"} {
		t.Run(provider, func(t *testing.T) {
			e := newCloudEmulator(t, provider)
			s, err := New(e.config())
			if err != nil {
				t.Fatal(err)
			}
			data := bytes.Repeat([]byte("stream\n"), 1600000)
			if err := s.Put(context.Background(), "large", struct{ io.Reader }{bytes.NewReader(data)}, int64(len(data)), "text/plain"); err != nil {
				t.Fatal(err)
			}
			r, err := s.Open(context.Background(), "large")
			if err != nil {
				t.Fatal(err)
			}
			defer r.Close()
			got, err := io.ReadAll(r)
			if err != nil || !bytes.Equal(got, data) {
				t.Fatalf("large stream mismatch: length=%d err=%v", len(got), err)
			}
		})
	}
}
func TestStreamEOFAfterReadAll(t *testing.T) {
	s, err := New(Config{Provider: "local", Root: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Put(context.Background(), "file", strings.NewReader("x"), 1, "text/plain"); err != nil {
		t.Fatal(err)
	}
	r, err := s.Open(context.Background(), "file")
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if _, err := io.ReadAll(r); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Read(make([]byte, 1)); err != io.EOF {
		t.Fatalf("repeated EOF = %v", err)
	}
}

func TestS3DefaultCredentialChainAndExplicitPrecedence(t *testing.T) {
	t.Setenv("AWS_ACCESS_KEY_ID", "ENV_ID")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "ENV_SECRET")
	t.Setenv("AWS_SESSION_TOKEN", "ENV_TOKEN")
	t.Setenv("AWS_EC2_METADATA_DISABLED", "true")
	for _, explicit := range []bool{false, true} {
		t.Run(strconv.FormatBool(explicit), func(t *testing.T) {
			e := newCloudEmulator(t, "s3")
			c := e.config()
			wantID, wantToken := "EXPLICIT_ID", "EXPLICIT_SESSION"
			if !explicit {
				c.AccessKeyID = ""
				c.SecretAccessKey = ""
				c.SessionToken = ""
				wantID = "ENV_ID"
				wantToken = "ENV_TOKEN"
			}
			s, err := New(c)
			if err != nil {
				t.Fatal(err)
			}
			if err := s.Put(context.Background(), "file", strings.NewReader("x"), 1, "text/plain"); err != nil {
				t.Fatal(err)
			}
			e.mu.Lock()
			defer e.mu.Unlock()
			if len(e.auth) != 1 || !strings.Contains(e.auth[0], "Credential="+wantID+"/") || e.tokens[0] != wantToken {
				t.Fatal("credential source or session token selection failed")
			}
		})
	}
}
func TestAzureConnectionStringSelection(t *testing.T) {
	e := newCloudEmulator(t, "azure")
	c := e.config()
	c.ConnectionString = "DefaultEndpointsProtocol=http;AccountName=folioaccount;AccountKey=" + c.AccountKey + ";BlobEndpoint=" + c.Endpoint + ";"
	c.Endpoint = ""
	c.AccountKey = ""
	s, err := New(c)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Test(context.Background()); err != nil {
		t.Fatal(err)
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, auth := range e.auth {
		if !strings.HasPrefix(auth, "SharedKey folioaccount:") {
			t.Fatal("connection string did not select shared key credentials")
		}
	}
}

func TestProbeCleanupSurvivesCancellation(t *testing.T) {
	for _, provider := range []string{"s3", "gcs", "azure"} {
		t.Run(provider, func(t *testing.T) {
			e := newCloudEmulator(t, provider)
			e.stream = true
			s, err := New(e.config())
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
			defer cancel()
			if err := s.Test(ctx); !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("probe deadline = %v", err)
			}
			e.mu.Lock()
			defer e.mu.Unlock()
			if len(e.objects) != 0 {
				t.Fatal("canceled probe leaked its object")
			}
		})
	}
}

func TestAlreadyCanceledProbeDoesNotContactProvider(t *testing.T) {
	for _, provider := range []string{"s3", "gcs", "azure"} {
		t.Run(provider, func(t *testing.T) {
			e := newCloudEmulator(t, provider)
			s, err := New(e.config())
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			if err := s.Test(ctx); !errors.Is(err, context.Canceled) {
				t.Fatalf("already canceled probe: %v", err)
			}
			e.mu.Lock()
			defer e.mu.Unlock()
			if len(e.seen) != 0 {
				t.Fatal("already canceled probe contacted provider")
			}
		})
	}
}
