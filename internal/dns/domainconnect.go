package dns

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"
)

type DomainConnect struct {
	Resolver                           Resolver
	HTTPClient                         *http.Client
	ProviderID, ServiceID, CallbackURL string
	// AllowedProviderHosts contains exact HTTPS authorities approved by the operator.
	// Discovery never fetches an arbitrary address supplied by DNS.
	AllowedProviderHosts []string
	StateKey             []byte
	StateTTL             time.Duration
	mu                   sync.Mutex
	pending              map[string]time.Time
}
type Discovery struct {
	Supported    bool   `json:"supported"`
	ProviderName string `json:"providerName"`
	Domain       string `json:"domain"`
	Host         string `json:"host"`
	URLSyncUX    string `json:"-"`
	Instructions string `json:"instructions,omitempty"`
}
type Authorization struct {
	AuthorizationURL string    `json:"authorizationUrl"`
	StateExpiresAt   time.Time `json:"stateExpiresAt"`
}
type StateClaims struct {
	AdminID, DomainID, Hostname string
	ExpiresAt                   time.Time
	Nonce                       string
}
type dcSettings struct{ ProviderID, ProviderName, URLSyncUX, URLAPI string }

var dcID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,252}$`)

func (d *DomainConnect) allowed(raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || strings.Contains(u.Path, "..") {
		return nil, errors.New("untrusted Domain Connect destination")
	}
	for _, host := range d.AllowedProviderHosts {
		if strings.EqualFold(u.Host, host) {
			return u, nil
		}
	}
	return nil, errors.New("Domain Connect provider host is not configured")
}
func (d *DomainConnect) get(ctx context.Context, raw string, out any) (int, error) {
	if _, err := d.allowed(raw); err != nil {
		return 0, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, raw, nil)
	if err != nil {
		return 0, errors.New("invalid Domain Connect request")
	}
	client := d.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	safe := *client
	safe.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	resp, err := safe.Do(req)
	if err != nil {
		return 0, errors.New("Domain Connect provider request failed")
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK && out != nil {
		if err = json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(out); err != nil {
			return resp.StatusCode, errors.New("invalid Domain Connect settings")
		}
	}
	return resp.StatusCode, nil
}
func (d *DomainConnect) Discover(ctx context.Context, hostname string) (Discovery, error) {
	fallback := Discovery{Instructions: "Configure the displayed ownership TXT and routing records manually. This provider/template is not configured for Domain Connect."}
	if err := ValidateHostname(hostname); err != nil {
		return fallback, err
	}
	if !dcID.MatchString(d.ProviderID) || !dcID.MatchString(d.ServiceID) || len(d.AllowedProviderHosts) == 0 {
		return fallback, nil
	}
	r := d.Resolver
	if r == nil {
		r = net.DefaultResolver
	}
	hostname = canonical(hostname)
	labels := strings.Split(hostname, ".")
	for i := 0; i < len(labels)-1; i++ {
		domain := strings.Join(labels[i:], ".")
		rows, err := r.LookupTXT(ctx, "_domainconnect."+domain)
		if err != nil {
			if ctx.Err() != nil {
				return fallback, ctx.Err()
			}
			continue
		}
		if len(rows) != 1 {
			return fallback, nil
		}
		discovery := rows[0]
		if strings.Contains(discovery, "://") {
			return fallback, nil
		}
		base, err := d.allowed("https://" + discovery)
		if err != nil {
			return fallback, nil
		}
		var settings dcSettings
		status, err := d.get(ctx, strings.TrimRight(base.String(), "/")+"/v2/"+url.PathEscape(domain)+"/settings", &settings)
		if err != nil || status != 200 {
			return fallback, nil
		}
		if settings.ProviderID == "" || settings.ProviderName == "" {
			return fallback, nil
		}
		api, err := d.allowed(settings.URLAPI)
		if err != nil {
			return fallback, nil
		}
		ux, err := d.allowed(settings.URLSyncUX)
		if err != nil {
			return fallback, nil
		}
		templatePath := "/v2/domainTemplates/providers/" + url.PathEscape(d.ProviderID) + "/services/" + url.PathEscape(d.ServiceID)
		status, err = d.get(ctx, strings.TrimRight(api.String(), "/")+templatePath, nil)
		if err != nil || status < 200 || status >= 300 {
			return fallback, nil
		}
		host := strings.TrimSuffix(strings.TrimSuffix(hostname, domain), ".")
		return Discovery{Supported: true, ProviderName: settings.ProviderName, Domain: domain, Host: host, URLSyncUX: ux.String()}, nil
	}
	return fallback, nil
}
func (d *DomainConnect) Authorize(ctx context.Context, hostname, adminID, domainID string, variables map[string]string) (Authorization, error) {
	result, err := d.Discover(ctx, hostname)
	if err != nil {
		return Authorization{}, err
	}
	if !result.Supported {
		return Authorization{}, ErrUnsupported
	}
	callback, err := url.Parse(d.CallbackURL)
	if err != nil || callback.Scheme != "https" || callback.Host == "" || callback.User != nil || callback.RawQuery != "" || callback.Fragment != "" {
		return Authorization{}, errors.New("a fixed HTTPS Domain Connect callback is required")
	}
	if len(d.StateKey) < 32 || adminID == "" || domainID == "" {
		return Authorization{}, errors.New("Domain Connect state key and identity binding are required")
	}
	ttl := d.StateTTL
	if ttl <= 0 || ttl > 15*time.Minute {
		ttl = 10 * time.Minute
	}
	expires := time.Now().UTC().Add(ttl)
	nonce := make([]byte, 24)
	if _, err = rand.Read(nonce); err != nil {
		return Authorization{}, err
	}
	claims := StateClaims{AdminID: adminID, DomainID: domainID, Hostname: canonical(hostname), ExpiresAt: expires, Nonce: base64.RawURLEncoding.EncodeToString(nonce)}
	raw, _ := json.Marshal(claims)
	payload := base64.RawURLEncoding.EncodeToString(raw)
	mac := hmac.New(sha256.New, d.StateKey)
	mac.Write([]byte(payload))
	state := payload + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	u, _ := url.Parse(strings.TrimRight(result.URLSyncUX, "/") + "/v2/domainTemplates/providers/" + url.PathEscape(d.ProviderID) + "/services/" + url.PathEscape(d.ServiceID) + "/apply")
	q := url.Values{}
	for k, v := range variables {
		switch k {
		case "domain", "host", "state", "redirect_uri", "sig", "key", "force", "groupId":
			return Authorization{}, errors.New("reserved Domain Connect variable")
		}
		if !dcID.MatchString(k) || len(v) > 4096 {
			return Authorization{}, errors.New("invalid Domain Connect variable")
		}
		q.Set(k, v)
	}
	q.Set("domain", result.Domain)
	if result.Host != "" {
		q.Set("host", result.Host)
	}
	q.Set("state", state)
	q.Set("redirect_uri", d.CallbackURL)
	u.RawQuery = q.Encode()
	d.mu.Lock()
	if d.pending == nil {
		d.pending = map[string]time.Time{}
	}
	now := time.Now()
	for key, expiry := range d.pending {
		if now.After(expiry) {
			delete(d.pending, key)
		}
	}
	d.pending[state] = expires
	d.mu.Unlock()
	return Authorization{u.String(), expires}, nil
}
func (d *DomainConnect) ConsumeState(state, adminID string) (StateClaims, error) {
	fail := errors.New("invalid, expired or already used Domain Connect state")
	if len(state) > 8192 || len(d.StateKey) < 32 || adminID == "" {
		return StateClaims{}, fail
	}
	parts := strings.Split(state, ".")
	if len(parts) != 2 {
		return StateClaims{}, fail
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return StateClaims{}, fail
	}
	mac := hmac.New(sha256.New, d.StateKey)
	mac.Write([]byte(parts[0]))
	if !hmac.Equal(sig, mac.Sum(nil)) {
		return StateClaims{}, fail
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return StateClaims{}, fail
	}
	var claims StateClaims
	if json.Unmarshal(raw, &claims) != nil || claims.AdminID != adminID || claims.DomainID == "" || time.Now().After(claims.ExpiresAt) {
		return StateClaims{}, fail
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	expiry, ok := d.pending[state]
	if !ok || time.Now().After(expiry) {
		return StateClaims{}, fail
	}
	delete(d.pending, state)
	return claims, nil
}
