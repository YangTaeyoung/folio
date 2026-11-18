package main

import (
	"context"
	"io"
	"net"
	"os"
	"strings"
	"testing"
	"time"
)

func TestServePathModeSkipsContentListener(t *testing.T) {
	t.Setenv("FOLIO_CONTENT_URL", "")
	held, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	defer held.Close()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- serve(ctx, []string{"--data-dir", t.TempDir(), "--listen", "127.0.0.1:0", "--content-listen", held.Addr().String(), "--dev"})
	}()
	select {
	case e := <-done:
		t.Fatal("serve stopped before cancellation:", e)
	case <-time.After(500 * time.Millisecond):
	}
	cancel()
	if e := <-done; e != nil {
		t.Fatal(e)
	}
}

func TestServeHostModeStillBindsContentListener(t *testing.T) {
	held, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	defer held.Close()
	e = serve(context.Background(), []string{"--data-dir", t.TempDir(), "--listen", "127.0.0.1:0", "--content-listen", held.Addr().String(), "--content-url", "http://{artifact}.content.folio.localhost:4311", "--dev"})
	if e == nil {
		t.Fatal("host mode started without its content listener")
	}
}

func TestServePathModeIgnoresContentListenEnvQuietly(t *testing.T) {
	t.Setenv("FOLIO_CONTENT_URL", "")
	t.Setenv("FOLIO_CONTENT_LISTEN", "127.0.0.1:0")
	r, w, e := os.Pipe()
	if e != nil {
		t.Fatal(e)
	}
	stderr := os.Stderr
	os.Stderr = w
	defer func() { os.Stderr = stderr }()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- serve(ctx, []string{"--data-dir", t.TempDir(), "--listen", "127.0.0.1:0", "--dev"})
	}()
	time.Sleep(300 * time.Millisecond)
	cancel()
	if e := <-done; e != nil {
		t.Fatal(e)
	}
	w.Close()
	os.Stderr = stderr
	out, _ := io.ReadAll(r)
	if strings.Contains(string(out), "Content listener disabled") {
		t.Fatal("environment default reported as an explicit setting:", string(out))
	}
	if !strings.Contains(string(out), "Content: http://folio.localhost:4310/c/{artifact}") {
		t.Fatal(string(out))
	}
}
