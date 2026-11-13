package blob

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"net"
	"strings"

	"cloud.google.com/go/storage"
	"google.golang.org/api/option"
)

type gcsBackend struct {
	client *storage.Client
	bucket string
}

func newGCS(c Config) (backend, error) {
	if c.Bucket == "" {
		return nil, errors.New("GCS bucket is required")
	}
	opts := []option.ClientOption{storage.WithJSONReads(), storage.WithDisabledClientMetrics()}
	if c.Endpoint != "" {
		u, err := endpoint(c.Endpoint)
		if err != nil {
			return nil, err
		}
		opts = append(opts, option.WithEndpoint(strings.TrimRight(c.Endpoint, "/")+"/storage/v1/"))
		// Unauthenticated custom endpoints are restricted to explicit loopback
		// emulators. Hosted endpoints use service-account credentials or ADC.
		ip := net.ParseIP(u.Hostname())
		if c.ServiceAccountJSON == "" && (u.Hostname() == "localhost" || (ip != nil && ip.IsLoopback())) {
			opts = append(opts, option.WithoutAuthentication())
		}
	}
	if c.ServiceAccountJSON != "" {
		opts = append(opts, option.WithCredentialsJSON([]byte(c.ServiceAccountJSON)))
	}
	ctx, cancel := context.WithTimeout(context.Background(), operationTimeout)
	defer cancel()
	client, err := storage.NewClient(ctx, opts...)
	if err != nil {
		return nil, errors.New("cannot initialize GCS credentials")
	}
	return &gcsBackend{client: client, bucket: c.Bucket}, nil
}
func gcsError(err error) error {
	if errors.Is(err, storage.ErrObjectNotExist) {
		return fs.ErrNotExist
	}
	return err
}
func (b *gcsBackend) put(ctx context.Context, key string, r io.Reader, size int64, contentType string) error {
	w := b.client.Bucket(b.bucket).Object(key).NewWriter(ctx)
	w.ContentType = contentType
	// A bounded resumable-upload buffer, rather than buffering complete artifacts.
	w.ChunkSize = 8 * 1024 * 1024
	if _, err := io.Copy(w, &exactReader{r: r, remaining: size}); err != nil {
		w.CloseWithError(err)
		return err
	}
	return gcsError(w.Close())
}
func (b *gcsBackend) open(ctx context.Context, key string) (io.ReadCloser, error) {
	r, err := b.client.Bucket(b.bucket).Object(key).NewReader(ctx)
	return r, gcsError(err)
}
func (b *gcsBackend) delete(ctx context.Context, key string) error {
	return gcsError(b.client.Bucket(b.bucket).Object(key).Delete(ctx))
}

func (b *gcsBackend) Close() error { return b.client.Close() }
