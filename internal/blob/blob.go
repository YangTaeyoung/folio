// Package blob stores private artifact bytes. It never changes bucket access policies.
package blob

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"strings"
	"sync"
	"time"
	"unicode"
)

type Config struct {
	ID, Name, Provider, Root, Bucket, Region, Endpoint, Prefix, AccountName, AccessKeyID, SecretAccessKey, SessionToken, ServiceAccountJSON, ConnectionString, AccountKey string
}

type Store interface {
	Put(context.Context, string, io.Reader, int64, string) error
	Open(context.Context, string) (io.ReadCloser, error)
	Delete(context.Context, string) error
	Test(context.Context) error
}

type backend interface {
	put(context.Context, string, io.Reader, int64, string) error
	open(context.Context, string) (io.ReadCloser, error)
	delete(context.Context, string) error
}

type store struct {
	backend   backend
	prefix    string
	lifecycle sync.RWMutex
	closed    bool
	closeErr  error
}

const operationTimeout = 5 * time.Minute

func New(c Config) (Store, error) {
	prefix := strings.Trim(c.Prefix, "/")
	if prefix != "" {
		if err := validateKey(prefix); err != nil {
			return nil, errors.New("invalid storage prefix")
		}
	}
	if c.Provider == "s3" || c.Provider == "gcs" || c.Provider == "azure" {
		if err := validateBucket(c.Bucket); err != nil {
			return nil, err
		}
	}
	var b backend
	var err error
	switch c.Provider {
	case "", "local":
		b, err = newLocal(c)
	case "s3":
		b, err = newS3(c)
	case "gcs":
		b, err = newGCS(c)
	case "azure":
		b, err = newAzure(c)
	default:
		return nil, errors.New("unsupported storage provider")
	}
	if err != nil {
		return nil, err
	}
	return &store{backend: b, prefix: prefix}, nil
}

// Close releases resources owned by this Store. It is optional so the public
// Store contract remains compatible with callers and injected implementations.
// Active operations and Open readers finish before the provider client closes.
func (s *store) Close() error {
	s.lifecycle.Lock()
	defer s.lifecycle.Unlock()
	if s.closed {
		return s.closeErr
	}
	s.closed = true
	if closer, ok := s.backend.(io.Closer); ok {
		s.closeErr = safeError("client close", closer.Close())
	}
	return s.closeErr
}
func (s *store) acquire() error {
	s.lifecycle.RLock()
	if s.closed {
		s.lifecycle.RUnlock()
		return errors.New("storage is closed")
	}
	return nil
}

func validateBucket(bucket string) error {
	if bucket == "" || len(bucket) > 222 {
		return errors.New("invalid storage bucket or container")
	}
	for _, r := range bucket {
		if !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '.' || r == '-' || r == '_') {
			return errors.New("invalid storage bucket or container")
		}
	}
	if bucket == "." || bucket == ".." {
		return errors.New("invalid storage bucket or container")
	}
	return nil
}

func validateKey(key string) error {
	if key == "" || len(key) > 1024 || strings.ContainsAny(key, `\%:?#`) {
		return errors.New("invalid object key")
	}
	for _, r := range key {
		if unicode.IsControl(r) {
			return errors.New("invalid object key")
		}
	}
	for _, part := range strings.Split(key, "/") {
		if part == "" || part == "." || part == ".." {
			return errors.New("invalid object key")
		}
	}
	return nil
}
func (s *store) key(k string) (string, error) {
	if err := validateKey(k); err != nil {
		return "", err
	}
	if s.prefix != "" {
		k = s.prefix + "/" + k
	}
	return k, validateKey(k)
}
func endpoint(raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return nil, errors.New("invalid storage endpoint")
	}
	return u, nil
}

// Provider responses can include URLs, credentials or object data; return bounded,
// non-secret errors while retaining cancellation and missing-object semantics.
func safeError(op string, err error) error {
	if err == nil {
		return nil
	}
	for _, known := range []error{context.Canceled, context.DeadlineExceeded, fs.ErrNotExist} {
		if errors.Is(err, known) {
			return fmt.Errorf("storage %s: %w", op, known)
		}
	}
	return fmt.Errorf("storage %s failed", op)
}
func (s *store) Put(ctx context.Context, k string, r io.Reader, size int64, contentType string) error {
	if err := s.acquire(); err != nil {
		return err
	}
	defer s.lifecycle.RUnlock()
	return s.put(ctx, k, r, size, contentType)
}
func (s *store) put(ctx context.Context, k string, r io.Reader, size int64, contentType string) error {
	k, err := s.key(k)
	if err != nil {
		return err
	}
	if r == nil || size < 0 {
		return errors.New("invalid object body or size")
	}
	if strings.ContainsAny(contentType, "\r\n\x00") {
		return errors.New("invalid content type")
	}
	ctx, cancel := context.WithTimeout(ctx, operationTimeout)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return err
	}
	return safeError("write", s.backend.put(ctx, k, &contextReader{ctx: ctx, r: r}, size, contentType))
}
func (s *store) Open(ctx context.Context, k string) (io.ReadCloser, error) {
	if err := s.acquire(); err != nil {
		return nil, err
	}
	r, err := s.open(ctx, k, s.lifecycle.RUnlock)
	if err != nil {
		s.lifecycle.RUnlock()
	}
	return r, err
}
func (s *store) open(ctx context.Context, k string, release func()) (io.ReadCloser, error) {
	k, err := s.key(k)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, operationTimeout)
	if err := ctx.Err(); err != nil {
		cancel()
		return nil, err
	}
	r, err := s.backend.open(ctx, k)
	if err != nil {
		cancel()
		return nil, safeError("read", err)
	}
	result := &stream{ctx: ctx, r: r, cancel: cancel, release: release}
	// A canceled request also releases an unread stream, so Store.Close cannot
	// retain the provider indefinitely if the caller abandons its reader.
	context.AfterFunc(ctx, func() { result.Close() })
	return result, nil
}
func (s *store) Delete(ctx context.Context, k string) error {
	if err := s.acquire(); err != nil {
		return err
	}
	defer s.lifecycle.RUnlock()
	return s.delete(ctx, k)
}
func (s *store) delete(ctx context.Context, k string) error {
	k, err := s.key(k)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, operationTimeout)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return err
	}
	err = s.backend.delete(ctx, k)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return safeError("delete", err)
}
func (s *store) Test(ctx context.Context) (err error) {
	if err := s.acquire(); err != nil {
		return err
	}
	defer s.lifecycle.RUnlock()
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return err
	}
	token := make([]byte, 24)
	if _, err = rand.Read(token); err != nil {
		return errors.New("cannot create storage probe")
	}
	key := ".folio-probes/" + hex.EncodeToString(token)
	// Cleanup even after a failed or canceled upload; an object may have committed
	// while its response was lost. The separate deadline cannot hang the caller.
	defer func() {
		cleanupCtx, stop := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer stop()
		if cleanupErr := s.delete(cleanupCtx, key); cleanupErr != nil {
			err = errors.Join(err, errors.New("storage probe cleanup failed"))
		}
	}()
	if err = s.put(ctx, key, bytes.NewReader(token), int64(len(token)), "application/octet-stream"); err != nil {
		return err
	}
	r, err := s.open(ctx, key, nil)
	if err != nil {
		return err
	}
	got, readErr := io.ReadAll(io.LimitReader(r, int64(len(token)+1)))
	closeErr := r.Close()
	if readErr != nil {
		return safeError("probe read", readErr)
	}
	if closeErr != nil {
		return safeError("probe close", closeErr)
	}
	if !bytes.Equal(token, got) {
		return errors.New("storage probe returned different bytes")
	}
	return nil
}

type contextReader struct {
	ctx context.Context
	r   io.Reader
}

func (r *contextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.r.Read(p)
}

type stream struct {
	ctx     context.Context
	r       io.ReadCloser
	cancel  context.CancelFunc
	once    sync.Once
	err     error
	eof     bool
	release func()
}

func (r *stream) Read(p []byte) (int, error) {
	if r.eof {
		return 0, io.EOF
	}
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	n, err := r.r.Read(p)
	if err == io.EOF {
		r.eof = true
		r.Close()
	} else if err != nil {
		err = safeError("stream", err)
		r.Close()
	}
	return n, err
}
func (r *stream) Close() error {
	r.once.Do(func() {
		r.cancel()
		r.err = safeError("close", r.r.Close())
		if r.release != nil {
			r.release()
		}
	})
	return r.err
}

// exactReader prevents committing truncated or overlong streams for providers
// whose SDK consumes until EOF. It does not buffer the complete artifact.
type exactReader struct {
	r         io.Reader
	remaining int64
	done      bool
}

func (r *exactReader) Read(p []byte) (int, error) {
	if r.done {
		return 0, io.EOF
	}
	if r.remaining == 0 {
		var b [1]byte
		n, err := r.r.Read(b[:])
		if n > 0 {
			return 0, errors.New("object size mismatch")
		}
		if err == io.EOF {
			r.done = true
		}
		return 0, err
	}
	if int64(len(p)) > r.remaining {
		p = p[:r.remaining]
	}
	n, err := r.r.Read(p)
	r.remaining -= int64(n)
	if err == io.EOF && r.remaining != 0 {
		return n, io.ErrUnexpectedEOF
	}
	return n, err
}
