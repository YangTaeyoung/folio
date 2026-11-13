package dns

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type Cloudflare struct {
	APIToken, APIBase string
	HTTPClient        *http.Client
}
type cfRecord struct{ ID, Type, Name, Content, Comment string }
type cfEnvelope struct {
	Success    bool
	Result     json.RawMessage
	ResultInfo struct {
		TotalPages int `json:"total_pages"`
	} `json:"result_info"`
	Errors []struct{ Code int }
}

const ownedComment = "Managed by Folio"

func (c *Cloudflare) request(ctx context.Context, method, path string, body, out any) (int, error) {
	if c.APIToken == "" {
		return 0, errors.New("Cloudflare API token is not configured")
	}
	base := c.APIBase
	if base == "" {
		base = "https://api.cloudflare.com/client/v4"
	}
	u, err := url.Parse(base)
	if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") {
		return 0, errors.New("invalid Cloudflare API base")
	}
	var b io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return 0, err
		}
		b = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(base, "/")+path, b)
	if err != nil {
		return 0, errors.New("invalid Cloudflare request")
	}
	req.Header.Set("Authorization", "Bearer "+c.APIToken)
	req.Header.Set("Content-Type", "application/json")
	client := c.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	safeClient := *client
	safeClient.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	resp, err := safeClient.Do(req)
	if err != nil {
		return 0, errors.New("Cloudflare request failed")
	}
	defer resp.Body.Close()
	var env cfEnvelope
	if err = json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(&env); err != nil {
		return 0, errors.New("invalid Cloudflare response")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 || !env.Success {
		code := 0
		if len(env.Errors) > 0 {
			code = env.Errors[0].Code
		}
		return 0, fmt.Errorf("Cloudflare request rejected (HTTP %d, code %d)", resp.StatusCode, code)
	}
	if out != nil {
		if err = json.Unmarshal(env.Result, out); err != nil {
			return 0, errors.New("invalid Cloudflare result")
		}
	}
	return env.ResultInfo.TotalPages, nil
}
func (c *Cloudflare) Zones(ctx context.Context) ([]Zone, error) {
	zones := []Zone{}
	for page := 1; page <= 1000; page++ {
		var rows []Zone
		pages, err := c.request(ctx, http.MethodGet, "/zones?per_page=50&page="+strconv.Itoa(page), nil, &rows)
		if err != nil {
			return nil, err
		}
		zones = append(zones, rows...)
		if page >= pages {
			return zones, nil
		}
	}
	return nil, errors.New("Cloudflare pagination limit exceeded")
}
func zonePath(zoneID string) (string, error) {
	if zoneID == "" || strings.ContainsAny(zoneID, "/\\?#") {
		return "", errors.New("invalid Cloudflare zone ID")
	}
	return "/zones/" + url.PathEscape(zoneID) + "/dns_records", nil
}
func (c *Cloudflare) Provision(ctx context.Context, zoneID, hostname, target string) (string, error) {
	if err := ValidateHostname(hostname); err != nil {
		return "", err
	}
	if err := ValidateHostname(target); err != nil {
		return "", err
	}
	return c.create(ctx, zoneID, "CNAME", canonical(hostname), canonical(target))
}
func (c *Cloudflare) ProvisionOwnership(ctx context.Context, zoneID, hostname, challenge string) (string, error) {
	if err := ValidateHostname(hostname); err != nil {
		return "", err
	}
	if len(challenge) == 0 || len(challenge) > 255 {
		return "", errors.New("invalid ownership challenge")
	}
	return c.create(ctx, zoneID, "TXT", "_folio."+canonical(hostname), challenge)
}
func (c *Cloudflare) create(ctx context.Context, zoneID, typ, name, value string) (string, error) {
	path, err := zonePath(zoneID)
	if err != nil {
		return "", err
	}
	var existing []cfRecord
	_, err = c.request(ctx, http.MethodGet, path+"?name="+url.QueryEscape(name)+"&per_page=100", nil, &existing)
	if err != nil {
		return "", err
	}
	if len(existing) > 0 {
		return "", ErrCollision
	}
	var created cfRecord
	_, err = c.request(ctx, http.MethodPost, path, map[string]any{"type": typ, "name": name, "content": value, "ttl": 1, "proxied": false, "comment": ownedComment}, &created)
	if err != nil {
		return "", err
	}
	if created.ID == "" {
		return "", errors.New("Cloudflare did not return a record ID")
	}
	return created.ID, nil
}
func (c *Cloudflare) DeleteOwned(ctx context.Context, zoneID, recordID, hostname, target string) error {
	path, err := zonePath(zoneID)
	if err != nil {
		return err
	}
	if recordID == "" || strings.ContainsAny(recordID, "/\\?#") {
		return errors.New("invalid Cloudflare record ID")
	}
	path += "/" + url.PathEscape(recordID)
	var record cfRecord
	if _, err = c.request(ctx, http.MethodGet, path, nil, &record); err != nil {
		return err
	}
	contentMatches := record.Content == target
	if record.Type == "CNAME" {
		contentMatches = canonical(record.Content) == canonical(target)
	}
	if (record.Type != "CNAME" && record.Type != "TXT") || canonical(record.Name) != canonical(hostname) || !contentMatches || record.Comment != ownedComment {
		return ErrNotOwned
	}
	_, err = c.request(ctx, http.MethodDelete, path, nil, nil)
	return err
}

// VerifyRoute reads the exact authenticated DNS record, including proxied
// CNAMEs whose target is intentionally absent from public DNS answers. The
// caller must independently verify ownership before activating the hostname.
func (c *Cloudflare) VerifyRoute(ctx context.Context, zoneID, hostname, target string) (bool, error) {
	if err := ValidateHostname(hostname); err != nil {
		return false, err
	}
	if err := ValidateHostname(target); err != nil {
		return false, err
	}
	path, err := zonePath(zoneID)
	if err != nil {
		return false, err
	}
	var rows []cfRecord
	if _, err = c.request(ctx, http.MethodGet, path+"?name="+url.QueryEscape(canonical(hostname))+"&per_page=100", nil, &rows); err != nil {
		return false, err
	}
	for _, row := range rows {
		if row.Type == "CNAME" && canonical(row.Name) == canonical(hostname) && canonical(row.Content) == canonical(target) {
			return true, nil
		}
	}
	return false, nil
}
