package dns

import (
	"context"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const fakeTunnelID = "00000000-1111-2222-3333-444444444444"

func certificate() []byte {
	return pem.EncodeToMemory(&pem.Block{Type: "ARGO TUNNEL TOKEN", Bytes: []byte(`{"accountID":"test-account","apiToken":"secret-certificate"}`)})
}

// The executable helper exercises real process creation, cancellation, output
// capture, credential files, and health probing without a Cloudflare account.
func TestCloudflaredHelper(t *testing.T) {
	if os.Getenv("FOLIO_DNS_HELPER") != "1" {
		return
	}
	args := os.Args
	for i, v := range args {
		if v == "--" {
			args = args[i+1:]
			break
		}
	}
	find := func(key string) string {
		for i, v := range args {
			if v == key && i+1 < len(args) {
				return args[i+1]
			}
		}
		return ""
	}
	has := func(key string) bool {
		for _, v := range args {
			if v == key {
				return true
			}
		}
		return false
	}
	if os.Getenv("TUNNEL_FORCE_PROVISIONING_DNS") != "" {
		fmt.Fprintln(os.Stderr, "unsafe ambient overwrite option")
		os.Exit(10)
	}
	switch {
	case has("login"):
		fmt.Fprintln(os.Stderr, "private-secret-log must never be returned")
		fmt.Fprintln(os.Stderr, "https://dash.cloudflare.com/argotunnel?aud=test&callback=https%3A%2F%2Flogin.cloudflareaccess.org%2Ftest")
		if os.Getenv("FOLIO_DNS_HELPER_MODE") == "timeout" {
			time.Sleep(10 * time.Second)
			os.Exit(1)
		}
		if os.Getenv("FOLIO_DNS_HELPER_MODE") == "error" {
			os.Exit(1)
		}
		time.Sleep(30 * time.Millisecond)
		os.WriteFile(os.Getenv("FOLIO_DNS_DEFAULT_CERT"), certificate(), 0600)
	case has("create"):
		os.WriteFile(find("--credentials-file"), []byte(`{"TunnelID":"`+fakeTunnelID+`","TunnelSecret":"secret-tunnel"}`), 0600)
		fmt.Printf(`{"id":"%s"}`, fakeTunnelID)
	case has("route"):
		if !has("--overwrite-dns=false") {
			os.Exit(12)
		}
		if has("collision.example.com") {
			fmt.Fprintln(os.Stderr, "private-secret-log collision")
			os.Exit(1)
		}
	case has("run"):
		listener, err := net.Listen("tcp", find("--metrics"))
		if err != nil {
			os.Exit(13)
		}
		server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/ready" {
				w.WriteHeader(404)
			}
		})}
		go server.Serve(listener)
		stop := make(chan os.Signal, 1)
		signal.Notify(stop, os.Interrupt)
		<-stop
		server.Close()
	default:
		fmt.Fprintln(os.Stderr, args)
		os.Exit(14)
	}
	os.Exit(0)
}
func fakeManager(t *testing.T) *Manager {
	t.Helper()
	dir := t.TempDir()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	wrapper := filepath.Join(dir, "cloudflared")
	if err = os.WriteFile(wrapper, []byte("#!/bin/sh\nexec \"$FOLIO_DNS_TEST_EXE\" -test.run=TestCloudflaredHelper -- \"$@\"\n"), 0700); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(dir, "official-cert.pem")
	t.Setenv("GORACE", "atexit_sleep_ms=0")
	t.Setenv("FOLIO_DNS_TEST_EXE", exe)
	t.Setenv("FOLIO_DNS_HELPER", "1")
	t.Setenv("FOLIO_DNS_DEFAULT_CERT", source)
	t.Setenv("TUNNEL_FORCE_PROVISIONING_DNS", "true")
	m := &Manager{DataDir: filepath.Join(dir, "data"), AppURL: "https://console.example.com", ContentURL: "https://{artifact}.content.example.com", Cloudflared: wrapper, defaultCert: source, LoginTimeout: 3 * time.Second}
	t.Cleanup(func() { m.Close() })
	return m
}
func waitLogin(t *testing.T, m *Manager, id, want string) LoginStatus {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		s, err := m.Login(id)
		if err != nil {
			t.Fatal(err)
		}
		if s.Status != "pending" {
			if s.Status != want {
				t.Fatal(s)
			}
			return s
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("login remained pending")
	return LoginStatus{}
}
func TestOfficialLoginLifecyclePrivateFilesAndExistingCertificate(t *testing.T) {
	m := fakeManager(t)
	home := os.Getenv("HOME")
	status, err := m.StartLogin(context.Background())
	if err != nil || status.ID == "" || !strings.HasPrefix(status.AuthorizationURL, "https://dash.cloudflare.com/argotunnel?") {
		t.Fatal(status, err)
	}
	waitLogin(t, m, status.ID, "complete")
	if os.Getenv("HOME") != home {
		t.Fatal("HOME changed")
	}
	dir := filepath.Join(m.DataDir, "dns")
	for _, path := range []string{dir, filepath.Join(dir, "cert.pem"), filepath.Join(dir, "command.json")} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		want := os.FileMode(0600)
		if info.IsDir() {
			want = 0700
		}
		if info.Mode().Perm() != want {
			t.Fatal(path, info.Mode())
		}
	}
	if _, err = os.Stat(m.defaultCert); err != nil {
		t.Fatal("official default certificate was removed", err)
	}
	copyManager := &Manager{DataDir: t.TempDir(), Cloudflared: m.Cloudflared, defaultCert: m.defaultCert}
	if _, err = copyManager.StartLogin(context.Background()); err == nil {
		t.Fatal("existing official certificate accepted without explicit import")
	}
	if err = copyManager.ImportCertificate(m.defaultCert); err != nil {
		t.Fatal(err)
	}
}
func TestLoginTimeoutAndSecretRedaction(t *testing.T) {
	m := fakeManager(t)
	m.LoginTimeout = 80 * time.Millisecond
	t.Setenv("FOLIO_DNS_HELPER_MODE", "timeout")
	s, err := m.StartLogin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	s = waitLogin(t, m, s.ID, "expired")
	if strings.Contains(s.Error, "private-secret") {
		t.Fatal("log leaked")
	}
}
func TestDedicatedTunnelConfigurationStartHealthStopAndCollision(t *testing.T) {
	m := fakeManager(t)
	if err := os.MkdirAll(filepath.Dir(m.defaultCert), 0700); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(m.defaultCert, certificate(), 0600)
	if err := m.ImportCertificate(m.defaultCert); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	status, err := m.ConfigureTunnel(ctx, "test", []string{"console.example.com", "demo.example.com"})
	if err != nil || !status.Configured || status.Running || status.Healthy || !strings.HasPrefix(status.Name, "folio-") {
		t.Fatal(status, err)
	}
	raw, err := os.ReadFile(filepath.Join(m.DataDir, "dns", "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	var config struct {
		Ingress []struct{ Hostname, Service string }
	}
	if json.Unmarshal(raw, &config) != nil || len(config.Ingress) != 3 || config.Ingress[0].Service != "http://127.0.0.1:4310" || config.Ingress[1].Service != "http://127.0.0.1:4311" || config.Ingress[2].Service != "http_status:404" {
		t.Fatal(string(raw))
	}
	if _, err = m.StartTunnel(ctx); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		status, err = m.Tunnel()
		if err != nil {
			t.Fatal(err)
		}
		if status.Healthy {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !status.Running || !status.Healthy {
		t.Fatal(status)
	}
	if _, err = m.ConfigureTunnel(ctx, "test", []string{"demo.example.com"}); err == nil {
		t.Fatal("running connector reconfigured")
	}
	status, err = m.StopTunnel(ctx)
	if err != nil || status.Running || status.Healthy {
		t.Fatal(status, err)
	}
	if err = m.RouteDNS(ctx, "unapproved.example.com"); err == nil {
		t.Fatal("unapproved host routed")
	}
	_, err = m.ConfigureTunnel(ctx, "test", []string{"console.example.com", "collision.example.com"})
	if err == nil || strings.Contains(err.Error(), "private-secret") {
		t.Fatal("collision not safely refused", err)
	}
	reloaded := &Manager{DataDir: m.DataDir}
	status, err = reloaded.Tunnel()
	if err != nil || status.ID != fakeTunnelID || status.Running || status.Healthy {
		t.Fatal(status, err)
	}
	for _, file := range []string{"credentials.json", "config.json", "tunnel.json"} {
		info, err := os.Stat(filepath.Join(m.DataDir, "dns", file))
		if err != nil || info.Mode().Perm() != 0600 {
			t.Fatal(file, info, err)
		}
	}
}

func TestLoginURLCaptureRequiresCompleteOfficialDestination(t *testing.T) {
	capture := &authCapture{found: make(chan string, 1)}
	capture.Write([]byte("https://evil.example/argotunnel?secret=x\nhttps://dash.cloudflare.com/argotunnel?aud="))
	select {
	case value := <-capture.found:
		t.Fatal("partial or untrusted URL accepted", value)
	default:
	}
	capture.Write([]byte("test&callback=https%3A%2F%2Flogin.cloudflareaccess.org%2Ftest\n"))
	select {
	case value := <-capture.found:
		if !strings.Contains(value, "aud=test&callback=") {
			t.Fatal(value)
		}
	default:
		t.Fatal("complete official URL was not captured")
	}
}
