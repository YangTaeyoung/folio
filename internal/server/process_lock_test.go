package server

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func lockTestConfig(dir string) Config {
	return Config{DataDir: dir, AppURL: "http://app.test", ContentURL: "http://{artifact}.content.test", BootstrapToken: "test-setup"}
}
func TestDataDirExclusiveUntilClose(t *testing.T) {
	dir := t.TempDir()
	cfg := lockTestConfig(dir)
	first, e := New(cfg)
	if e != nil {
		t.Fatal(e)
	}
	defer first.Close()
	key, e := os.ReadFile(filepath.Join(dir, "data.key"))
	if e != nil {
		t.Fatal(e)
	}
	orgID := first.st.Organization.ID
	secondCfg := cfg
	secondCfg.AppURL = "http://other-app.test:9999"
	second, e := New(secondCfg)
	if second != nil {
		second.Close()
		t.Fatal("second instance opened the live state")
	}
	if !errors.Is(e, ErrDataDirInUse) {
		t.Fatalf("duplicate error: %v", e)
	}
	// A rejected second connection must not accidentally release the first lock.
	second, e = New(secondCfg)
	if second != nil {
		second.Close()
		t.Fatal("failed duplicate released the active lock")
	}
	if !errors.Is(e, ErrDataDirInUse) {
		t.Fatalf("second duplicate error: %v", e)
	}
	keyAfter, e := os.ReadFile(filepath.Join(dir, "data.key"))
	if e != nil || !bytes.Equal(key, keyAfter) || first.st.Organization.ID != orgID {
		t.Fatal("duplicate startup changed existing state")
	}
	if e = first.Close(); e != nil {
		t.Fatal(e)
	}
	if e = first.Close(); e != nil {
		t.Fatal("idempotent close", e)
	}
	reopened, e := New(secondCfg)
	if e != nil {
		t.Fatalf("reopen after close: %v", e)
	}
	defer reopened.Close()
	if reopened.st.Organization.ID != orgID {
		t.Fatal("reopen discarded persisted state")
	}
}

func TestDataDirLockReleasedAfterStartupFailure(t *testing.T) {
	dir := t.TempDir()
	if e := os.WriteFile(filepath.Join(dir, "data.key"), []byte("invalid"), 0600); e != nil {
		t.Fatal(e)
	}
	s, e := New(lockTestConfig(dir))
	if s != nil {
		s.Close()
		t.Fatal("invalid key unexpectedly accepted")
	}
	if e == nil || errors.Is(e, ErrDataDirInUse) {
		t.Fatalf("expected key failure, got %v", e)
	}
	if e = os.Remove(filepath.Join(dir, "data.key")); e != nil {
		t.Fatal(e)
	}
	s, e = New(lockTestConfig(dir))
	if e != nil {
		t.Fatalf("failed startup leaked process lock: %v", e)
	}
	s.Close()
}

func TestDataDirExclusiveAcrossProcessesAndCrashRecovery(t *testing.T) {
	dir := t.TempDir()
	exe, e := os.Executable()
	if e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, exe, "-test.run=^TestDataDirLockSubprocess$", "-test.count=1")
	cmd.Env = append(os.Environ(), "FOLIO_TEST_PROCESS_LOCK_DIR="+dir)
	output, e := cmd.StdoutPipe()
	if e != nil {
		t.Fatal(e)
	}
	input, e := cmd.StdinPipe()
	if e != nil {
		t.Fatal(e)
	}
	defer input.Close()
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if e = cmd.Start(); e != nil {
		t.Fatal(e)
	}
	defer func() {
		if cmd.Process != nil {
			cmd.Process.Kill()
		}
	}()
	ready := make(chan string, 1)
	go func() { line, _ := bufio.NewReader(output).ReadString('\n'); ready <- line }()
	select {
	case line := <-ready:
		if line != "LOCKED\n" {
			cmd.Wait()
			t.Fatalf("child failed to acquire lock: %q %s", line, stderr.String())
		}
	case <-ctx.Done():
		t.Fatal("child readiness timeout")
	}
	second, e := New(lockTestConfig(dir))
	if second != nil {
		second.Close()
		t.Fatal("another process opened the live data directory")
	}
	if !errors.Is(e, ErrDataDirInUse) {
		t.Fatalf("cross-process error: %v", e)
	}
	// Kill without Close: kernel/SQLite file locks, rather than a stale PID file,
	// must let a new process recover immediately without deleting lock files.
	if e = cmd.Process.Kill(); e != nil {
		t.Fatal(e)
	}
	cmd.Wait()
	recovered, e := New(lockTestConfig(dir))
	if e != nil {
		t.Fatalf("reopen after process crash: %v", e)
	}
	recovered.Close()
}

func TestDataDirLockSubprocess(t *testing.T) {
	dir := os.Getenv("FOLIO_TEST_PROCESS_LOCK_DIR")
	if dir == "" {
		t.Skip("subprocess helper")
	}
	s, e := New(lockTestConfig(dir))
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	fmt.Fprintln(os.Stdout, "LOCKED")
	io.Copy(io.Discard, os.Stdin)
}
