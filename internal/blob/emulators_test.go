package blob

import (
	"bytes"
	"context"
	"encoding/base64"
	"io"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// Run only against explicitly selected loopback emulators. These tests create
// and remove their own private container/bucket; they refuse live endpoints.
// FOLIO_TEST_AZURITE=http://127.0.0.1:38471/folioaccount
//
//	AZURITE_ACCOUNTS=folioaccount:dGVzdC1vbmx5LWtleQ== azurite-blob ...
//
// FOLIO_TEST_GCS=http://127.0.0.1:4443
// FOLIO_TEST_S3=http://127.0.0.1:9000 (minioadmin/minioadmin)
func TestStandaloneEmulators(t *testing.T) {
	for _, provider := range []string{"azure", "gcs", "s3"} {
		t.Run(provider, func(t *testing.T) {
			envName := map[string]string{"azure": "FOLIO_TEST_AZURITE", "gcs": "FOLIO_TEST_GCS", "s3": "FOLIO_TEST_S3"}[provider]
			raw := os.Getenv(envName)
			if raw == "" {
				t.Skip("set " + envName + " for independent emulator integration")
			}
			u, err := endpoint(raw)
			if err != nil {
				t.Fatal(err)
			}
			ip := net.ParseIP(u.Hostname())
			if u.Hostname() != "localhost" && (ip == nil || !ip.IsLoopback()) {
				t.Fatal("standalone tests only permit loopback emulators")
			}
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			c := Config{Provider: provider, Endpoint: raw, Bucket: "folio-test-" + strings.ToLower(time.Now().UTC().Format("20060102t150405000000000")), Prefix: "nested/private"}
			switch provider {
			case "azure":
				c.AccountName = "folioaccount"
				c.AccountKey = base64.StdEncoding.EncodeToString([]byte("test-only-key"))
			case "s3":
				c.AccessKeyID = "minioadmin"
				c.SecretAccessKey = "minioadmin"
				c.Region = "us-east-1"
			}
			s, err := New(c)
			if err != nil {
				t.Fatal(err)
			}
			switch b := s.(*store).backend.(type) {
			case *azureBackend:
				if _, err := b.client.CreateContainer(ctx, c.Bucket, nil); err != nil {
					t.Fatalf("create test container: %v", err)
				}
				defer b.client.DeleteContainer(context.Background(), c.Bucket, nil)
			case *gcsBackend:
				if err := b.client.Bucket(c.Bucket).Create(ctx, "folio-emulator-project", nil); err != nil {
					t.Fatalf("create test bucket: %v", err)
				}
				defer b.client.Bucket(c.Bucket).Delete(context.Background())
			case *s3Backend:
				if _, err := b.client.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: aws.String(c.Bucket)}); err != nil {
					t.Fatalf("create test bucket: %v", err)
				}
				defer b.client.DeleteBucket(context.Background(), &s3.DeleteBucketInput{Bucket: aws.String(c.Bucket)})
			}
			if err := s.Test(ctx); err != nil {
				t.Fatal(err)
			}
			data := bytes.Repeat([]byte("independent emulator stream\n"), 350000)
			if err := s.Put(ctx, "artifact/index.html", struct{ io.Reader }{bytes.NewReader(data)}, int64(len(data)), "text/html"); err != nil {
				t.Fatal(err)
			}
			defer s.Delete(context.Background(), "artifact/index.html")
			r, err := s.Open(ctx, "artifact/index.html")
			if err != nil {
				t.Fatal(err)
			}
			got, readErr := io.ReadAll(r)
			closeErr := r.Close()
			if readErr != nil || closeErr != nil || !bytes.Equal(got, data) {
				t.Fatalf("stream mismatch: length %d read=%v close=%v", len(got), readErr, closeErr)
			}
			if err := s.Delete(ctx, "artifact/index.html"); err != nil {
				t.Fatal(err)
			}
		})
	}
}
