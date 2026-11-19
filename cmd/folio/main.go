package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/YangTaeyoung/folio/internal/agent"
	"github.com/YangTaeyoung/folio/internal/dns"
	"github.com/YangTaeyoung/folio/internal/server"
	"github.com/YangTaeyoung/folio/internal/webassets"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var version = "dev"

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if e := run(ctx, os.Args[1:]); e != nil && !errors.Is(e, context.Canceled) {
		fmt.Fprintln(os.Stderr, "folio:", e)
		os.Exit(1)
	}
}
func output(v any) error { e := json.NewEncoder(os.Stdout); e.SetIndent("", "  "); return e.Encode(v) }
func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
func flags(name string) *flag.FlagSet {
	f := flag.NewFlagSet(name, flag.ContinueOnError)
	f.SetOutput(os.Stderr)
	return f
}
func run(ctx context.Context, args []string) error {
	if len(args) == 0 {
		usage()
		return nil
	}
	cmd := args[0]
	args = args[1:]
	if cmd == "version" || cmd == "--version" {
		fmt.Println("Folio", version)
		return nil
	}
	if cmd == "help" || cmd == "--help" {
		usage()
		return nil
	}
	if cmd == "serve" {
		return serve(ctx, args)
	}
	if cmd == "skills" {
		return installSkill(args)
	}
	if cmd == "connect" {
		return connect(args)
	}
	cfg, e := agent.LoadConfig()
	if e != nil {
		return e
	}
	f := flags(cmd)
	base := f.String("url", cfg.URL, "Folio management origin")
	var tokenStdin *bool
	if cmd == "login" {
		tokenStdin = f.Bool("token-stdin", false, "Read API token from stdin (never a shell argument)")
	}
	title := f.String("title", "", "Artifact title")
	description := f.String("description", "", "Artifact description")
	slug := f.String("slug", "", "Stable URL slug")
	visibility := f.String("visibility", "", "private, organization, selected, public")
	members := f.String("members", "", "Comma-separated member IDs")
	expires := f.String("expires", "", "RFC3339 expiry (empty clears when sharing)")
	artifact := f.String("artifact", "", "Existing artifact ID to update")
	baseVersion := f.String("base-version", "", "Current version ID required for update")
	message := f.String("message", "", "Revision note")
	draft := f.Bool("draft", false, "Publish a review draft without changing the official link")
	tags := f.String("tags", "", "Comma-separated tags")
	view := f.String("view", "", "mine, shared, archive, all")
	query := f.String("query", "", "Search text")
	sort := f.String("sort", "updated", "updated or title")
	if e = f.Parse(args); e != nil {
		return e
	}
	cfg.URL = *base
	c, e := agent.NewClient(cfg)
	if e != nil {
		return e
	}
	if cmd == "login" {
		if tokenStdin == nil || !*tokenStdin {
			return errors.New("create a scoped API token in Agents & API keys, then run folio login --url URL --token-stdin")
		}
		raw, e := io.ReadAll(io.LimitReader(os.Stdin, 8193))
		if e != nil {
			return e
		}
		if len(raw) > 8192 {
			return errors.New("token too long")
		}
		cfg.Token = strings.TrimSpace(string(raw))
		if cfg.Token == "" {
			return errors.New("token is empty")
		}
		c.Token = cfg.Token
		if _, e = c.JSON(ctx, "GET", "/api/me", nil); e != nil {
			return e
		}
		if e = agent.SaveConfig(cfg); e != nil {
			return e
		}
		fmt.Fprintln(os.Stderr, "Folio credentials saved in private local config.")
		return nil
	}
	if cmd == "mcp" {
		return agent.NewMCP(c, version).Run(ctx, &mcp.StdioTransport{})
	}
	var result map[string]any
	id := func() (string, error) {
		if f.NArg() != 1 {
			return "", fmt.Errorf("usage: folio %s [flags] ARTIFACT_ID", cmd)
		}
		return url.PathEscape(f.Arg(0)), nil
	}
	switch cmd {
	case "doctor":
		result, e = c.JSON(ctx, "GET", "/api/capabilities", nil)
		if e == nil {
			_, e = c.JSON(ctx, "GET", "/api/me", nil)
			if e == nil {
				result["authenticated"] = true
			} else if cfg.Token == "" {
				result["authenticated"] = false
				e = nil
			}
		}
	case "publish":
		if f.NArg() != 1 {
			return errors.New("usage: folio publish [flags] PATH (flags before path)")
		}
		result, e = c.Publish(ctx, agent.PublishInput{Path: f.Arg(0), Title: *title, Description: *description, Slug: *slug, Visibility: *visibility, MemberIDs: split(*members), ExpiresAt: *expires, ArtifactID: *artifact, BaseVersionID: *baseVersion, Message: *message, Draft: *draft, Tags: split(*tags)})
	case "list":
		q := url.Values{"view": {*view}, "q": {*query}, "sort": {*sort}}
		result, e = c.JSON(ctx, "GET", "/api/artifacts?"+q.Encode(), nil)
	case "inspect", "archive", "restore", "share":
		var v string
		v, e = id()
		if e != nil {
			return e
		}
		endpoint := "/api/artifacts/" + v
		if cmd == "inspect" {
			result, e = c.JSON(ctx, "GET", endpoint, nil)
		} else if cmd == "share" {
			if *visibility == "" {
				return errors.New("share requires --visibility")
			}
			var expiry *string
			if *expires != "" {
				expiry = expires
			}
			result, e = c.JSON(ctx, "PATCH", endpoint+"/sharing", map[string]any{"visibility": *visibility, "memberIds": split(*members), "expiresAt": expiry})
		} else {
			result, e = c.JSON(ctx, "POST", endpoint+"/"+cmd, agent.Empty{})
		}
	default:
		return fmt.Errorf("unknown command %q; run folio help", cmd)
	}
	if e != nil {
		return e
	}
	return output(result)
}
func split(v string) []string {
	out := []string{}
	for _, p := range strings.Split(v, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
func usage() {
	fmt.Fprintln(os.Stderr, `Folio — self-hosted artifact publishing for people and agents

  folio serve [--data-dir ./data] [--listen 127.0.0.1:4310]
  folio login --url http://folio.localhost:4310 --token-stdin
  folio connect --client codex  # codex, claudecode, antigravity, vscode, cursor, opencode [--config-dir DIRECTORY]
  folio publish --title "My artifact" ./site
  folio publish --artifact ID --base-version VERSION --draft ./site
  folio list [--view mine|shared|archive] [--query text]
  folio inspect ID
  folio share --visibility private|organization|selected|public [--members ID,ID] ID
  folio archive ID | folio restore ID
  folio doctor | folio mcp | folio skills install [--client codex|claude|agents]

Client commands read FOLIO_URL/FOLIO_TOKEN or private local config.
Flags must precede positional arguments. Cloud services are opt-in.`)
}
func serve(ctx context.Context, args []string) error {
	f := flags("serve")
	data := f.String("data-dir", env("FOLIO_DATA_DIR", "./data"), "Private persistent data directory")
	listen := f.String("listen", env("FOLIO_LISTEN", "127.0.0.1:4310"), "Management bind address")
	contentListen := f.String("content-listen", env("FOLIO_CONTENT_LISTEN", "127.0.0.1:4311"), "Content bind address")
	app := f.String("app-url", env("FOLIO_APP_URL", "http://folio.localhost:4310"), "Advertised management origin")
	content := f.String("content-url", env("FOLIO_CONTENT_URL", ""), "Artifact content URL template: {artifact} in the path serves under the management origin (default <app-url>/c/{artifact}); in the host it gives each artifact its own hostname")
	dev := f.Bool("dev", false, "Allow source build without embedded UI; use Vite separately")
	dcProvider := f.String("domainconnect-provider", env("FOLIO_DOMAINCONNECT_PROVIDER", ""), "Provider ID for an approved Domain Connect template")
	dcService := f.String("domainconnect-service", env("FOLIO_DOMAINCONNECT_SERVICE", ""), "Service ID for that approved template")
	dcHosts := f.String("domainconnect-allowed-hosts", env("FOLIO_DOMAINCONNECT_ALLOWED_HOSTS", ""), "Comma-separated exact trusted HTTPS provider authorities")
	cert := f.String("cloudflare-cert", env("FOLIO_CLOUDFLARE_CERT", ""), "Explicit existing official cloudflared certificate to copy privately")
	cloudflared := f.String("cloudflared", env("FOLIO_CLOUDFLARED", ""), "Optional cloudflared executable path")
	if e := f.Parse(args); e != nil {
		return e
	}
	if !*dev && !webassets.Ready() {
		return errors.New("embedded UI missing; run make build or use a release binary (development: --dev)")
	}
	layout, e := server.ResolveContentURL(*app, *content)
	if e != nil {
		return e
	}
	*content = layout.Template
	contentListenSet := false
	f.Visit(func(fl *flag.Flag) {
		if fl.Name == "content-listen" {
			contentListenSet = true
		}
	})
	// Bind every listener before announcing readiness; no half-started management service.
	l, e := net.Listen("tcp", *listen)
	if e != nil {
		return e
	}
	defer l.Close()
	var cl net.Listener
	contentOrigin := listenerOrigin(l.Addr())
	if !layout.PathMode {
		cl, e = net.Listen("tcp", *contentListen)
		if e != nil {
			return e
		}
		defer cl.Close()
		contentOrigin = listenerOrigin(cl.Addr())
	} else if contentListenSet {
		fmt.Fprintln(os.Stderr, "Content listener disabled: content is served under the management origin")
	}
	manager := &dns.Manager{DataDir: *data, AppURL: *app, ContentURL: *content, Cloudflared: *cloudflared, AppOrigin: listenerOrigin(l.Addr()), ContentOrigin: contentOrigin}
	defer manager.Close()
	if *cert != "" {
		if e = manager.ImportCertificate(*cert); e != nil {
			return fmt.Errorf("cannot import Cloudflare certificate: %w", e)
		}
	}
	var dc *dns.DomainConnect
	if *dcProvider != "" || *dcService != "" || *dcHosts != "" {
		if *dcProvider == "" || *dcService == "" || len(split(*dcHosts)) == 0 {
			return errors.New("Domain Connect needs provider, service and allowed-hosts together")
		}
		for _, host := range split(*dcHosts) {
			u, e := url.Parse("https://" + host)
			if e != nil || u.Host != host || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
				return errors.New("Domain Connect allowed-hosts must be exact HTTPS authorities")
			}
		}
		dc = &dns.DomainConnect{ProviderID: *dcProvider, ServiceID: *dcService, AllowedProviderHosts: split(*dcHosts)}
	}
	s, e := server.New(server.Config{DataDir: *data, AppURL: *app, ContentURL: *content, BootstrapToken: os.Getenv("FOLIO_BOOTSTRAP_TOKEN"), Version: version, WebFS: webassets.FS(), DNS: manager, DomainConnect: dc})
	if e != nil {
		return e
	}
	defer s.Close()
	a := &http.Server{Handler: s.Handler(), ReadHeaderTimeout: 10 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 1 << 20}
	b := &http.Server{Handler: s.ContentHandler(), ReadHeaderTimeout: 10 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 1 << 20}
	errs := make(chan error, 2)
	go func() { errs <- a.Serve(l) }()
	if cl != nil {
		go func() { errs <- b.Serve(cl) }()
	}
	fmt.Fprintln(os.Stderr, "Folio", version, "ready:", *app, "(management)")
	fmt.Fprintln(os.Stderr, "Content:", *content)
	if token := s.BootstrapToken(); token != "" {
		fmt.Fprintln(os.Stderr, "First-run setup:", strings.TrimRight(*app, "/")+"/#setup="+url.QueryEscape(token))
		fmt.Fprintln(os.Stderr, "Setup token is stored privately in the data directory; keep this URL private.")
	}
	maintenanceCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-maintenanceCtx.Done():
				return
			case <-ticker.C:
				if _, e := s.Maintenance(maintenanceCtx, false); e != nil && maintenanceCtx.Err() == nil {
					fmt.Fprintln(os.Stderr, "Folio scheduled cleanup failed; inspect storage and audit log")
				}
			}
		}
	}()
	select {
	case <-ctx.Done():
		e = nil
	case e = <-errs:
		if errors.Is(e, http.ErrServerClosed) {
			e = nil
		}
	}
	cancel()
	shutdown, stop := context.WithTimeout(context.Background(), 10*time.Second)
	defer stop()
	if a.Shutdown(shutdown) != nil {
		_ = a.Close()
	}
	if b.Shutdown(shutdown) != nil {
		_ = b.Close()
	}
	<-done
	return e
}

func listenerOrigin(addr net.Addr) string {
	host, port, _ := net.SplitHostPort(addr.String())
	if ip := net.ParseIP(host); ip != nil && ip.IsUnspecified() {
		if ip.To4() != nil {
			host = "127.0.0.1"
		} else {
			host = "::1"
		}
	}
	return "http://" + net.JoinHostPort(host, port)
}

func installSkill(args []string) error {
	if len(args) == 0 || args[0] != "install" {
		return errors.New("usage: folio skills install --client codex|claude|agents")
	}
	f := flags("skills install")
	client := f.String("client", "agents", "codex, claude, or agents")
	dest := f.String("dir", "", "Explicit skill parent directory (offline or other clients)")
	if e := f.Parse(args[1:]); e != nil {
		return e
	}
	if *dest == "" {
		home, e := os.UserHomeDir()
		if e != nil {
			return e
		}
		sub := map[string]string{"codex": ".codex/skills", "claude": ".claude/skills", "agents": ".agents/skills"}[*client]
		if sub == "" {
			return errors.New("unknown client; use --dir for another skill directory")
		}
		*dest = filepath.Join(home, sub)
	}
	p := filepath.Join(*dest, "folio", "SKILL.md")
	if e := os.MkdirAll(filepath.Dir(p), 0755); e != nil {
		return e
	}
	file, e := os.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if errors.Is(e, os.ErrExist) {
		return errors.New("Folio skill already exists; inspect and update it manually (nothing overwritten)")
	}
	if e != nil {
		return e
	}
	_, e = file.WriteString(agent.Skill)
	ce := file.Close()
	if e != nil {
		return e
	}
	if ce != nil {
		return ce
	}
	fmt.Fprintln(os.Stderr, "Installed Folio skill:", p)
	return nil
}
