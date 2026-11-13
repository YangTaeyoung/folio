package dns

import (
	"context"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
)

// ErrBrowserDNSUnavailable means the selected browser certificate could not
// authorize the generic DNS/zone API required for safe automated ownership.
// Official tunnel-route permission alone is not proof of generic DNS permission.
var ErrBrowserDNSUnavailable = errors.New("browser certificate DNS access unavailable; use a DNS API token or configure ownership TXT manually")

type accountCertificate struct{ ZoneID, AccountID, APIToken, Endpoint string }

func (m *Manager) certificateDNS() (*Cloudflare, string, error) {
	dir, err := m.directory()
	if err != nil {
		return nil, "", err
	}
	raw, err := regularRead(filepath.Join(dir, "cert.pem"))
	if err != nil {
		return nil, "", errors.New("Cloudflare browser login or explicit certificate import is required")
	}
	block, _ := pem.Decode(raw)
	var cert accountCertificate
	if block == nil || block.Type != "ARGO TUNNEL TOKEN" || json.Unmarshal(block.Bytes, &cert) != nil || cert.AccountID == "" || cert.ZoneID == "" || cert.APIToken == "" {
		return nil, "", errors.New("browser certificate has no selected Cloudflare zone; repeat official login")
	}
	base := m.DNSAPIBase
	if base == "" && strings.EqualFold(cert.Endpoint, "fed") {
		base = "https://api.fed.cloudflare.com/client/v4"
	}
	// Token exists only in this private operation; never return credentials from
	// an exported method or put them in configuration/status/error objects.
	return &Cloudflare{APIToken: cert.APIToken, APIBase: base, HTTPClient: m.HTTPClient}, cert.ZoneID, nil
}
func browserDNSError(err error) error {
	var provider *ProviderError
	if errors.As(err, &provider) && (provider.HTTPStatus == http.StatusUnauthorized || provider.HTTPStatus == http.StatusForbidden) {
		return fmt.Errorf("%w (HTTP %d, code %d)", ErrBrowserDNSUnavailable, provider.HTTPStatus, provider.Code)
	}
	return err
}
func withinZone(hostname, zone string) bool {
	hostname = canonical(hostname)
	zone = canonical(zone)
	return hostname == zone || strings.HasSuffix(hostname, "."+zone)
}
func selectedHostname(zone Zone, hostname string) error {
	if err := ValidateHostname(hostname); err != nil {
		return err
	}
	if !withinZone(hostname, zone.Name) {
		return errors.New("hostname is outside the browser-login selected Cloudflare zone")
	}
	return nil
}

// SelectedZone returns only the certificate's explicitly selected zone. Success
// proves current zone-read access; it does not promise TXT-write permission.
func (m *Manager) SelectedZone(ctx context.Context) (Zone, error) {
	cf, id, err := m.certificateDNS()
	if err != nil {
		return Zone{}, err
	}
	zone, err := cf.Zone(ctx, id)
	return zone, browserDNSError(err)
}

// ProvisionOwnership tries the generic DNS API using the official browser
// certificate. A record ID is returned only after collision-safe TXT creation
// succeeds. An API denial is explicit; it never reports partial success as done.
func (m *Manager) ProvisionOwnership(ctx context.Context, hostname, challenge string) (string, error) {
	cf, id, err := m.certificateDNS()
	if err != nil {
		return "", err
	}
	zone, err := cf.Zone(ctx, id)
	if err != nil {
		return "", browserDNSError(err)
	}
	if err = selectedHostname(zone, hostname); err != nil {
		return "", err
	}
	recordID, err := cf.ProvisionOwnership(ctx, id, hostname, challenge)
	return recordID, browserDNSError(err)
}

// VerifyRoute reads an exact provider record without exposing certificate data.
// Callers must also verify the public ownership TXT and/or exact-host origin.
func (m *Manager) VerifyRoute(ctx context.Context, hostname, target string) (bool, error) {
	cf, id, err := m.certificateDNS()
	if err != nil {
		return false, err
	}
	zone, err := cf.Zone(ctx, id)
	if err != nil {
		return false, browserDNSError(err)
	}
	if err = selectedHostname(zone, hostname); err != nil {
		return false, err
	}
	ok, err := cf.VerifyRoute(ctx, id, hostname, target)
	return ok, browserDNSError(err)
}

// DeleteOwned deletes only an exact Folio-commented record in the selected zone.
// Official cloudflared tunnel CNAME routes have no Folio comment and are refused.
func (m *Manager) DeleteOwned(ctx context.Context, recordID, hostname, target string) error {
	cf, id, err := m.certificateDNS()
	if err != nil {
		return err
	}
	zone, err := cf.Zone(ctx, id)
	if err != nil {
		return browserDNSError(err)
	}
	baseHostname := strings.TrimPrefix(canonical(hostname), "_folio.")
	if err = selectedHostname(zone, baseHostname); err != nil {
		return err
	}
	return browserDNSError(cf.DeleteOwned(ctx, id, recordID, hostname, target))
}
