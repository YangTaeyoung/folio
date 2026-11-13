package dns

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

type LoginStatus struct {
	ID               string `json:"id"`
	AuthorizationURL string `json:"authorizationUrl,omitempty"`
	Status           string `json:"status"`
	Error            string `json:"error,omitempty"`
}
type TunnelStatus struct {
	Configured bool     `json:"configured"`
	Running    bool     `json:"running"`
	Healthy    bool     `json:"healthy"`
	ID         string   `json:"id,omitempty"`
	Name       string   `json:"name,omitempty"`
	Hostnames  []string `json:"hostnames"`
}
type loginProcess struct {
	status LoginStatus
	cancel context.CancelFunc
}
type tunnelProcess struct {
	cmd     *exec.Cmd
	done    chan struct{}
	metrics string
}

// Manager invokes only official cloudflared commands. Construct one Manager per
// installation and call Close at shutdown. ConfigureTunnel is an opt-in mutation.
type Manager struct {
	DataDir, AppURL, ContentURL, Cloudflared string
	AppOrigin, ContentOrigin                 string
	LoginTimeout                             time.Duration
	DNSAPIBase                               string
	HTTPClient                               *http.Client
	mu                                       sync.Mutex
	opMu                                     sync.Mutex
	logins                                   map[string]*loginProcess
	process                                  *tunnelProcess
	defaultCert                              string // tests override the official default without changing HOME
}

func (m *Manager) directory() (string, error) {
	if m.DataDir == "" {
		return "", errors.New("DNS data directory is required")
	}
	dir := filepath.Join(m.DataDir, "dns")
	if info, err := os.Lstat(dir); err == nil && (!info.IsDir() || info.Mode()&os.ModeSymlink != 0) {
		return "", errors.New("DNS data directory must be a regular directory")
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	if err := os.Chmod(dir, 0700); err != nil {
		return "", err
	}
	if _, err := os.Lstat(filepath.Join(dir, "command.json")); os.IsNotExist(err) {
		if err = privateWrite(filepath.Join(dir, "command.json"), []byte("{}")); err != nil {
			return "", err
		}
	}
	return dir, nil
}
func privateWrite(path string, data []byte) error {
	file, err := os.CreateTemp(filepath.Dir(path), ".folio-*")
	if err != nil {
		return err
	}
	tmp := file.Name()
	defer os.Remove(tmp)
	if err = file.Chmod(0600); err == nil {
		_, err = file.Write(data)
	}
	closeErr := file.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
func regularRead(path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > 1<<20 {
		return nil, errors.New("credential must be a bounded regular file")
	}
	return os.ReadFile(path)
}
func (m *Manager) ImportCertificate(path string) error {
	dir, err := m.directory()
	if err != nil {
		return err
	}
	data, err := regularRead(path)
	if err != nil {
		return errors.New("cannot read Cloudflare certificate")
	}
	block, _ := pem.Decode(data)
	if block == nil || block.Type != "ARGO TUNNEL TOKEN" {
		return errors.New("invalid Cloudflare account certificate")
	}
	var cert struct{ AccountID, APIToken string }
	if json.Unmarshal(block.Bytes, &cert) != nil || cert.AccountID == "" || cert.APIToken == "" {
		return errors.New("invalid Cloudflare account certificate")
	}
	return privateWrite(filepath.Join(dir, "cert.pem"), data)
}
func (m *Manager) binary() string {
	if m.Cloudflared != "" {
		return m.Cloudflared
	}
	return "cloudflared"
}

// Keep the system HOME intact. Strip ambient cloudflared options so environment
// variables cannot enable overwrite-dns, external config files, or debug logs.
func commandEnv() []string {
	env := []string{}
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		if strings.HasPrefix(key, "TUNNEL_") || strings.HasPrefix(key, "CLOUDFLARED_") || key == "NO_AUTOUPDATE" {
			continue
		}
		env = append(env, entry)
	}
	return env
}
func (m *Manager) defaultCertificate() (string, error) {
	if m.defaultCert != "" {
		return m.defaultCert, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".cloudflared", "cert.pem"), nil
}

var authURLPattern = regexp.MustCompile(`https://dash\.cloudflare\.com/argotunnel[^\s<>"\x1b]*`)

type authCapture struct {
	mu    sync.Mutex
	tail  string
	found chan string
	sent  bool
}

func (w *authCapture) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.tail += string(p)
	if len(w.tail) > 32768 {
		w.tail = w.tail[len(w.tail)-32768:]
	}
	if !w.sent {
		for _, match := range authURLPattern.FindAllStringIndex(w.tail, -1) {
			if match[1] == len(w.tail) {
				continue
			}
			raw := w.tail[match[0]:match[1]]
			u, err := url.Parse(raw)
			if err == nil && u.Scheme == "https" && u.Host == "dash.cloudflare.com" && u.Path == "/argotunnel" && u.RawQuery != "" {
				w.found <- u.String()
				w.sent = true
				break
			}
		}
	}
	return len(p), nil
}
func (m *Manager) StartLogin(ctx context.Context) (LoginStatus, error) {
	m.opMu.Lock()
	defer m.opMu.Unlock()
	if err := ctx.Err(); err != nil {
		return LoginStatus{}, err
	}
	dir, err := m.directory()
	if err != nil {
		return LoginStatus{}, err
	}
	m.mu.Lock()
	for _, p := range m.logins {
		if p.status.Status == "pending" {
			s := p.status
			m.mu.Unlock()
			return s, nil
		}
	}
	m.mu.Unlock()
	if _, err := regularRead(filepath.Join(dir, "cert.pem")); err == nil {
		return LoginStatus{Status: "complete"}, nil
	}
	source, err := m.defaultCertificate()
	if err != nil {
		return LoginStatus{}, err
	}
	if _, err = os.Lstat(source); !os.IsNotExist(err) {
		return LoginStatus{}, errors.New("official default Cloudflare certificate already exists; explicitly import it instead of overwriting it")
	}
	nonce := make([]byte, 16)
	if _, err = rand.Read(nonce); err != nil {
		return LoginStatus{}, err
	}
	id := hex.EncodeToString(nonce)
	timeout := m.LoginTimeout
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}
	runCtx, cancel := context.WithTimeout(context.Background(), timeout)
	cmd := exec.CommandContext(runCtx, m.binary(), "tunnel", "--config", filepath.Join(dir, "command.json"), "login")
	cmd.Env = commandEnv()
	capture := &authCapture{found: make(chan string, 1)}
	cmd.Stdout = capture
	cmd.Stderr = capture
	if err = cmd.Start(); err != nil {
		cancel()
		return LoginStatus{}, errors.New("cannot start cloudflared login; install cloudflared first")
	}
	p := &loginProcess{status: LoginStatus{ID: id, Status: "pending"}, cancel: cancel}
	m.mu.Lock()
	if m.logins == nil {
		m.logins = map[string]*loginProcess{}
	}
	m.logins[id] = p
	m.mu.Unlock()
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer cancel()
		err := cmd.Wait()
		status := "complete"
		message := ""
		if runCtx.Err() != nil {
			status = "expired"
			message = "Cloudflare login timed out or was cancelled"
		} else if err != nil {
			status = "error"
			message = "Cloudflare login failed"
		} else if err = m.ImportCertificate(source); err != nil {
			status = "error"
			message = "Cloudflare login did not produce a valid account certificate"
		}
		m.mu.Lock()
		p.status.Status = status
		p.status.Error = message
		m.mu.Unlock()
	}()
	timer := time.NewTimer(10 * time.Second)
	defer timer.Stop()
	select {
	case raw := <-capture.found:
		m.mu.Lock()
		p.status.AuthorizationURL = raw
		s := p.status
		m.mu.Unlock()
		return s, nil
	case <-done:
		return m.Login(id)
	case <-ctx.Done():
		cancel()
		return LoginStatus{}, ctx.Err()
	case <-timer.C:
		cancel()
		return LoginStatus{}, errors.New("cloudflared did not emit an official authorization URL")
	}
}
func (m *Manager) Login(id string) (LoginStatus, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	p, ok := m.logins[id]
	if !ok {
		return LoginStatus{}, errors.New("login attempt not found")
	}
	return p.status, nil
}
func (m *Manager) load() (TunnelStatus, error) {
	dir, err := m.directory()
	if err != nil {
		return TunnelStatus{}, err
	}
	data, err := regularRead(filepath.Join(dir, "tunnel.json"))
	if os.IsNotExist(err) {
		return TunnelStatus{Hostnames: []string{}}, nil
	}
	if err != nil {
		return TunnelStatus{}, err
	}
	var s TunnelStatus
	if json.Unmarshal(data, &s) != nil || !tunnelIDPattern.MatchString(s.ID) || !strings.HasPrefix(s.Name, "folio-") {
		return TunnelStatus{}, errors.New("invalid owned tunnel configuration")
	}
	s.Running = false
	s.Healthy = false
	return s, nil
}
func (m *Manager) Tunnel() (TunnelStatus, error) {
	s, err := m.load()
	if err != nil {
		return s, err
	}
	m.mu.Lock()
	p := m.process
	if p != nil {
		select {
		case <-p.done:
		default:
			s.Running = true
		}
	}
	m.mu.Unlock()
	if s.Running {
		client := http.Client{Timeout: time.Second}
		resp, err := client.Get("http://" + p.metrics + "/ready")
		if err == nil {
			s.Healthy = resp.StatusCode == 200
			resp.Body.Close()
		}
	}
	return s, nil
}

var tunnelIDPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
var tunnelNamePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9-]{0,47}$`)

func (m *Manager) run(ctx context.Context, args ...string) ([]byte, error) {
	bounded, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	cmd := exec.CommandContext(bounded, m.binary(), args...)
	cmd.Env = commandEnv()
	// Provider diagnostics may contain credentials. Never return stdout/stderr in errors.
	out := &boundedOutput{}
	cmd.Stdout = out
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		return nil, errors.New("cloudflared command failed; check login, DNS collisions, and network connectivity")
	}
	return out.data, nil
}

type boundedOutput struct{ data []byte }

func (b *boundedOutput) Write(p []byte) (int, error) {
	if len(b.data)+len(p) > 1<<20 {
		return 0, errors.New("cloudflared response too large")
	}
	b.data = append(b.data, p...)
	return len(p), nil
}
func (m *Manager) baseArgs(dir string) []string {
	return []string{"tunnel", "--no-autoupdate", "--config", filepath.Join(dir, "command.json"), "--origincert", filepath.Join(dir, "cert.pem")}
}
func (m *Manager) ConfigureTunnel(ctx context.Context, name string, hostnames []string) (TunnelStatus, error) {
	m.opMu.Lock()
	defer m.opMu.Unlock()
	if _, err := origin(m.AppOrigin, "http://127.0.0.1:4310"); err != nil {
		return TunnelStatus{}, err
	}
	if _, err := origin(m.ContentOrigin, "http://127.0.0.1:4311"); err != nil {
		return TunnelStatus{}, err
	}
	if u, err := url.Parse(m.AppURL); err != nil || u.Hostname() == "" {
		return TunnelStatus{}, errors.New("advertised AppURL is required")
	}
	if !tunnelNamePattern.MatchString(name) || len(hostnames) == 0 || len(hostnames) > 100 {
		return TunnelStatus{}, errors.New("valid tunnel name and 1–100 exact hostnames required")
	}
	seen := map[string]bool{}
	hosts := []string{}
	for _, host := range hostnames {
		if err := ValidateHostname(host); err != nil {
			return TunnelStatus{}, err
		}
		host = canonical(host)
		if !seen[host] {
			seen[host] = true
			hosts = append(hosts, host)
		}
	}
	dir, err := m.directory()
	if err != nil {
		return TunnelStatus{}, err
	}
	if _, err = regularRead(filepath.Join(dir, "cert.pem")); err != nil {
		return TunnelStatus{}, errors.New("Cloudflare login is required")
	}
	zone, err := m.SelectedZone(ctx)
	if err != nil {
		return TunnelStatus{}, err
	}
	for _, host := range hosts {
		if !withinZone(host, zone.Name) {
			return TunnelStatus{}, errors.New("hostname is outside the browser-login selected Cloudflare zone")
		}
	}
	s, err := m.Tunnel()
	if err != nil {
		return s, err
	}
	if s.Running {
		return s, errors.New("stop the Folio connector before changing ingress")
	}
	if !s.Configured {
		installation := make([]byte, 8)
		if _, err = rand.Read(installation); err != nil {
			return s, err
		}
		s.Name = "folio-" + hex.EncodeToString(installation) + "-" + name
		args := append(m.baseArgs(dir), "create", "--output", "json", "--credentials-file", filepath.Join(dir, "credentials.json"), s.Name)
		out, err := m.run(ctx, args...)
		if err != nil {
			return s, err
		}
		var created struct{ ID string }
		if json.Unmarshal(out, &created) != nil || !tunnelIDPattern.MatchString(created.ID) {
			return s, errors.New("cloudflared did not return a valid tunnel ID")
		}
		s.ID = created.ID
		s.Configured = true
		s.Hostnames = []string{}
		credentialData, credentialErr := regularRead(filepath.Join(dir, "credentials.json"))
		var credential struct{ TunnelID, TunnelSecret string }
		if credentialErr != nil || json.Unmarshal(credentialData, &credential) != nil || credential.TunnelID != s.ID || credential.TunnelSecret == "" {
			return s, errors.New("cloudflared did not write matching tunnel credentials")
		}
		if err = os.Chmod(filepath.Join(dir, "credentials.json"), 0600); err != nil {
			return s, errors.New("cannot protect tunnel credentials")
		}
		// Persist the owned ID before routing. A collision can leave a stopped,
		// recoverable dedicated tunnel; retry never selects an unrelated tunnel.
		data, _ := json.Marshal(s)
		if err = privateWrite(filepath.Join(dir, "tunnel.json"), data); err != nil {
			return s, err
		}
	}
	for _, host := range hosts {
		if contains(s.Hostnames, host) {
			continue
		}
		if err = m.route(ctx, dir, s.ID, host); err != nil {
			return s, err
		}
		s.Hostnames = append(s.Hostnames, host)
		data, _ := json.Marshal(s)
		if err = privateWrite(filepath.Join(dir, "tunnel.json"), data); err != nil {
			return s, err
		}
	}
	s.Hostnames = hosts
	if err = m.writeIngress(dir, s); err != nil {
		return s, err
	}
	data, _ := json.Marshal(s)
	if err = privateWrite(filepath.Join(dir, "tunnel.json"), data); err != nil {
		return s, err
	}
	return s, nil
}
func contains(rows []string, value string) bool {
	for _, row := range rows {
		if row == value {
			return true
		}
	}
	return false
}
func (m *Manager) route(ctx context.Context, dir, id, hostname string) error {
	_, err := m.run(ctx, append(m.baseArgs(dir), "route", "dns", "--overwrite-dns=false", id, hostname)...)
	return err
}
func (m *Manager) RouteDNS(ctx context.Context, hostname string) error {
	m.opMu.Lock()
	defer m.opMu.Unlock()
	if err := ValidateHostname(hostname); err != nil {
		return err
	}
	s, err := m.Tunnel()
	if err != nil {
		return err
	}
	if !s.Configured {
		return errors.New("dedicated Folio tunnel is not configured")
	}
	if !contains(s.Hostnames, canonical(hostname)) {
		return errors.New("hostname must first be included in the explicit Folio tunnel ingress")
	}
	zone, err := m.SelectedZone(ctx)
	if err != nil {
		return err
	}
	if !withinZone(hostname, zone.Name) {
		return errors.New("hostname is outside the browser-login selected Cloudflare zone")
	}
	dir, err := m.directory()
	if err != nil {
		return err
	}
	return m.route(ctx, dir, s.ID, canonical(hostname))
}
func origin(raw, fallback string) (string, error) {
	if raw == "" {
		raw = fallback
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return "", errors.New("tunnel upstream must be an HTTP(S) origin")
	}
	return u.Scheme + "://" + u.Host, nil
}
func (m *Manager) writeIngress(dir string, s TunnelStatus) error {
	app, err := origin(m.AppOrigin, "http://127.0.0.1:4310")
	if err != nil {
		return err
	}
	content, err := origin(m.ContentOrigin, "http://127.0.0.1:4311")
	if err != nil {
		return err
	}
	appURL, err := url.Parse(m.AppURL)
	if err != nil || appURL.Hostname() == "" {
		return errors.New("advertised AppURL is required")
	}
	ingress := []map[string]string{}
	for _, host := range s.Hostnames {
		service := content
		if canonical(appURL.Hostname()) == host {
			service = app
		}
		ingress = append(ingress, map[string]string{"hostname": host, "service": service})
	}
	ingress = append(ingress, map[string]string{"service": "http_status:404"})
	config := map[string]any{"tunnel": s.ID, "credentials-file": filepath.Join(dir, "credentials.json"), "ingress": ingress}
	data, _ := json.MarshalIndent(config, "", "  ")
	return privateWrite(filepath.Join(dir, "config.json"), data)
}
func (m *Manager) StartTunnel(ctx context.Context) (TunnelStatus, error) {
	m.opMu.Lock()
	defer m.opMu.Unlock()
	if err := ctx.Err(); err != nil {
		return TunnelStatus{}, err
	}
	s, err := m.Tunnel()
	if err != nil || s.Running {
		return s, err
	}
	if !s.Configured {
		return s, errors.New("dedicated Folio tunnel is not configured")
	}
	dir, err := m.directory()
	if err != nil {
		return s, err
	}
	if err = m.writeIngress(dir, s); err != nil {
		return s, err
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return s, err
	}
	metrics := listener.Addr().String()
	listener.Close()
	args := []string{"tunnel", "--no-autoupdate", "--origincert", filepath.Join(dir, "cert.pem"), "--config", filepath.Join(dir, "config.json"), "--metrics", metrics, "run", s.ID}
	cmd := exec.Command(m.binary(), args...)
	cmd.Env = commandEnv()
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err = cmd.Start(); err != nil {
		return s, errors.New("cannot start Folio cloudflared connector")
	}
	p := &tunnelProcess{cmd: cmd, done: make(chan struct{}), metrics: metrics}
	m.mu.Lock()
	m.process = p
	m.mu.Unlock()
	go func() { _ = cmd.Wait(); close(p.done) }()
	return m.Tunnel()
}
func (m *Manager) StopTunnel(ctx context.Context) (TunnelStatus, error) {
	m.opMu.Lock()
	defer m.opMu.Unlock()
	m.mu.Lock()
	p := m.process
	m.mu.Unlock()
	if p != nil {
		select {
		case <-p.done:
		default:
			if err := p.cmd.Process.Signal(os.Interrupt); err != nil {
				_ = p.cmd.Process.Kill()
			}
			timer := time.NewTimer(3 * time.Second)
			defer timer.Stop()
			select {
			case <-p.done:
			case <-ctx.Done():
				_ = p.cmd.Process.Kill()
				<-p.done
			case <-timer.C:
				_ = p.cmd.Process.Kill()
				<-p.done
			}
		}
	}
	return m.Tunnel()
}
func (m *Manager) Close() error {
	m.mu.Lock()
	for _, p := range m.logins {
		p.cancel()
	}
	m.mu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := m.StopTunnel(ctx)
	return err
}

// CertificatePath returns a local path, never certificate contents.
func (m *Manager) CertificatePath() (string, error) {
	dir, err := m.directory()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "cert.pem"), nil
}
