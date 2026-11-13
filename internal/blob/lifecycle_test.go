package blob

import (
	"context"
	"errors"
	"io"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestStoreCloseStopsFurtherOperations(t *testing.T) {
	for _, provider := range []string{"local", "s3", "gcs", "azure"} {
		t.Run(provider, func(t *testing.T) {
			c := Config{Provider: "local", Root: t.TempDir()}
			var e *cloudEmulator
			if provider != "local" {
				e = newCloudEmulator(t, provider)
				c = e.config()
			}
			s, err := New(c)
			if err != nil {
				t.Fatal(err)
			}
			closer, ok := s.(io.Closer)
			if !ok {
				t.Fatal("store does not provide optional Close")
			}
			if err := closer.Close(); err != nil {
				t.Fatal(err)
			}
			if err := closer.Close(); err != nil {
				t.Fatal(err)
			}
			if err := s.Put(context.Background(), "file", strings.NewReader("x"), 1, "text/plain"); err == nil {
				t.Fatal("closed store accepted Put")
			}
			if r, err := s.Open(context.Background(), "file"); err == nil {
				r.Close()
				t.Fatal("closed store accepted Open")
			}
			if err := s.Delete(context.Background(), "file"); err == nil {
				t.Fatal("closed store accepted Delete")
			}
			if err := s.Test(context.Background()); err == nil {
				t.Fatal("closed store accepted probe")
			}
			if e != nil {
				e.mu.Lock()
				defer e.mu.Unlock()
				if len(e.seen) != 0 {
					t.Fatal("closed store made provider requests")
				}
			}
		})
	}
}

func TestCloseWaitsForActiveReader(t *testing.T) {
	s, err := New(Config{Provider: "local", Root: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Put(context.Background(), "file", strings.NewReader("hello"), 5, "text/plain"); err != nil {
		t.Fatal(err)
	}
	r, err := s.Open(context.Background(), "file")
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- s.(io.Closer).Close() }()
	select {
	case err := <-done:
		t.Fatalf("Close returned while reader active: %v", err)
	case <-time.After(20 * time.Millisecond):
	}
	got, err := io.ReadAll(r)
	if err != nil || string(got) != "hello" {
		t.Fatalf("active reader failed: %q %v", got, err)
	}
	r.Close()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("Close did not finish after EOF")
	}
}

func TestCanceledReaderReleasesStoreWithoutConsumerRead(t *testing.T) {
	s, err := New(Config{Provider: "local", Root: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Put(context.Background(), "file", strings.NewReader("hello"), 5, "text/plain"); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	r, err := s.Open(ctx, "file")
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	cancel()
	done := make(chan error, 1)
	go func() { done <- s.(io.Closer).Close() }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("canceled reader retained store resources")
	}
}

type closeCountingBackend struct{ calls atomic.Int32 }

func (*closeCountingBackend) put(context.Context, string, io.Reader, int64, string) error { return nil }
func (*closeCountingBackend) open(context.Context, string) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("")), nil
}
func (*closeCountingBackend) delete(context.Context, string) error { return nil }
func (b *closeCountingBackend) Close() error {
	b.calls.Add(1)
	return errors.New("test provider close failure")
}
func TestStoreCloseDelegatesOnceAndPreservesCloseFailure(t *testing.T) {
	b := &closeCountingBackend{}
	s := &store{backend: b}
	for range 2 {
		if err := s.Close(); err == nil {
			t.Fatal("provider close failure lost")
		}
	}
	if b.calls.Load() != 1 {
		t.Fatalf("provider closed %d times", b.calls.Load())
	}
}
