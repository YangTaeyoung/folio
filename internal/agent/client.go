// Package agent contains the shared authenticated CLI and MCP client.
package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Client struct {
	URL, Token string
	HTTP       *http.Client
}
type Config struct {
	URL   string `json:"url"`
	Token string `json:"token"`
}

func ConfigPath() (string, error) {
	if p := os.Getenv("FOLIO_CONFIG"); p != "" {
		return p, nil
	}
	d, e := os.UserConfigDir()
	return filepath.Join(d, "folio", "config.json"), e
}
func LoadConfig() (Config, error) {
	var c Config
	p, e := ConfigPath()
	if e != nil {
		return c, e
	}
	b, e := os.ReadFile(p)
	if e != nil && !errors.Is(e, os.ErrNotExist) {
		return c, e
	}
	if len(b) > 0 {
		if e = json.Unmarshal(b, &c); e != nil {
			return c, fmt.Errorf("invalid Folio config: %w", e)
		}
	}
	if v := os.Getenv("FOLIO_URL"); v != "" {
		c.URL = v
	}
	if v := os.Getenv("FOLIO_TOKEN"); v != "" {
		c.Token = v
	}
	if c.URL == "" {
		c.URL = "http://folio.localhost:4310"
	}
	return c, nil
}
func SaveConfig(c Config) error {
	p, e := ConfigPath()
	if e != nil {
		return e
	}
	if e = os.MkdirAll(filepath.Dir(p), 0700); e != nil {
		return e
	}
	f, e := os.CreateTemp(filepath.Dir(p), ".config-*")
	if e != nil {
		return e
	}
	defer os.Remove(f.Name())
	if e = f.Chmod(0600); e != nil {
		f.Close()
		return e
	}
	e = json.NewEncoder(f).Encode(c)
	if ce := f.Close(); e == nil {
		e = ce
	}
	if e != nil {
		return e
	}
	return os.Rename(f.Name(), p)
}
func NewClient(c Config) (*Client, error) {
	u, e := url.Parse(c.URL)
	if e != nil || u.Host == "" || u.User != nil || (u.Scheme != "http" && u.Scheme != "https") || u.RawQuery != "" || u.Fragment != "" || strings.Trim(u.Path, "/") != "" {
		return nil, errors.New("Folio URL must be an HTTP(S) origin without credentials or a path")
	}
	return &Client{URL: strings.TrimRight(c.URL, "/"), Token: c.Token, HTTP: &http.Client{Timeout: 5 * time.Minute, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}, nil
}
func (c *Client) request(ctx context.Context, method, path, contentType string, body io.Reader) (map[string]any, error) {
	req, e := http.NewRequestWithContext(ctx, method, c.URL+path, body)
	if e != nil {
		return nil, e
	}
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	hc := c.HTTP
	if hc == nil {
		hc = http.DefaultClient
	}
	res, e := hc.Do(req)
	if e != nil {
		return nil, errors.New("cannot reach Folio; verify FOLIO_URL, network and server status")
	}
	defer res.Body.Close()
	raw, e := io.ReadAll(io.LimitReader(res.Body, 16<<20))
	if e != nil {
		return nil, e
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		var v struct {
			Error struct{ Code, Message string }
		}
		_ = json.Unmarshal(raw, &v)
		if v.Error.Message == "" {
			v.Error.Message = http.StatusText(res.StatusCode)
		}
		return nil, fmt.Errorf("Folio HTTP %d (%s): %s", res.StatusCode, v.Error.Code, v.Error.Message)
	}
	out := map[string]any{}
	if len(raw) > 0 {
		if e = json.Unmarshal(raw, &out); e != nil {
			return nil, errors.New("Folio returned invalid JSON")
		}
	}
	return out, nil
}
func (c *Client) JSON(ctx context.Context, method, path string, in any) (map[string]any, error) {
	var b io.Reader
	if in != nil {
		raw, e := json.Marshal(in)
		if e != nil {
			return nil, e
		}
		b = bytes.NewReader(raw)
	}
	return c.request(ctx, method, path, "application/json", b)
}

type PublishInput struct {
	Path          string   `json:"path" jsonschema:"Absolute path to a file or directory on this MCP server machine. Hidden files and symlinks are refused or excluded."`
	Title         string   `json:"title,omitempty" jsonschema:"Human-readable artifact title. If omitted on update, retain the current title; on creation use the local file or directory name."`
	Description   string   `json:"description,omitempty"`
	Slug          string   `json:"slug,omitempty"`
	ArtifactID    string   `json:"artifactId,omitempty" jsonschema:"Existing artifact ID for publishing a new revision; omit to create"`
	BaseVersionID string   `json:"baseVersionId,omitempty" jsonschema:"Required for updates: currentVersionId from folio_inspect. Stale updates return conflict."`
	Message       string   `json:"message,omitempty"`
	Visibility    string   `json:"visibility,omitempty" jsonschema:"private (default), organization, selected, or public. Change only as requested by the user."`
	MemberIDs     []string `json:"memberIds,omitempty"`
	ExpiresAt     string   `json:"expiresAt,omitempty"`
}
type uploadFile struct {
	path, source string
	size         int64
}

func collectFiles(root string) ([]uploadFile, error) {
	abs, e := filepath.Abs(root)
	if e != nil {
		return nil, e
	}
	fi, e := os.Lstat(abs)
	if e != nil {
		return nil, e
	}
	if fi.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New("symlinks are not published")
	}
	var files []uploadFile
	add := func(p, rel string, info os.FileInfo) error {
		if !info.Mode().IsRegular() {
			return errors.New("only regular files can be published")
		}
		if len(files) >= 2500 {
			return errors.New("at most 2500 files are allowed")
		}
		files = append(files, uploadFile{filepath.ToSlash(rel), p, info.Size()})
		return nil
	}
	if !fi.IsDir() {
		if strings.HasPrefix(fi.Name(), ".") {
			return nil, errors.New("hidden files are not published")
		}
		e = add(abs, fi.Name(), fi)
	} else {
		e = filepath.WalkDir(abs, func(p string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if p == abs {
				return nil
			}
			if strings.HasPrefix(d.Name(), ".") {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			if d.Type()&os.ModeSymlink != 0 {
				return errors.New("directory contains a symlink; remove it before publishing")
			}
			if d.IsDir() {
				return nil
			}
			rel, e := filepath.Rel(abs, p)
			if e != nil {
				return e
			}
			info, e := d.Info()
			if e != nil {
				return e
			}
			return add(p, rel, info)
		})
	}
	if e != nil {
		return nil, e
	}
	if len(files) == 0 {
		return nil, errors.New("no publishable files found")
	}
	return files, nil
}
func (c *Client) Publish(ctx context.Context, in PublishInput) (map[string]any, error) {
	if in.Path == "" {
		return nil, errors.New("path is required")
	}
	if in.ArtifactID != "" && in.BaseVersionID == "" {
		return nil, errors.New("updates require baseVersionId from folio_inspect")
	}
	files, e := collectFiles(in.Path)
	if e != nil {
		return nil, e
	}
	capabilities, e := c.JSON(ctx, "GET", "/api/capabilities", nil)
	if e != nil {
		return nil, e
	}
	limit, _ := capabilities["maxUploadBytes"].(float64)
	var total int64
	for _, item := range files {
		if item.size < 0 || item.size > int64(limit)-total {
			return nil, errors.New("files exceed this installation's upload limit; inspect folio doctor")
		}
		total += item.size
	}
	abs, e := filepath.Abs(in.Path)
	if e != nil {
		return nil, e
	}
	info, e := os.Lstat(abs)
	if e != nil {
		return nil, e
	}
	rootPath := abs
	if !info.IsDir() {
		rootPath = filepath.Dir(abs)
	}
	root, e := os.OpenRoot(rootPath)
	if e != nil {
		return nil, e
	}
	defer root.Close()
	if in.Title == "" {
		if in.ArtifactID != "" {
			existing, e := c.JSON(ctx, "GET", "/api/artifacts/"+url.PathEscape(in.ArtifactID), nil)
			if e != nil {
				return nil, e
			}
			a, _ := existing["artifact"].(map[string]any)
			in.Title, _ = a["title"].(string)
			if in.Description == "" {
				in.Description, _ = a["description"].(string)
			}
		} else {
			in.Title = filepath.Base(abs)
		}
	}
	meta := map[string]any{"title": in.Title, "description": in.Description, "message": in.Message}
	if in.Slug != "" {
		meta["slug"] = in.Slug
	}
	if in.Visibility != "" {
		meta["visibility"] = in.Visibility
	}
	if len(in.MemberIDs) > 0 {
		meta["memberIds"] = in.MemberIDs
	}
	if in.ExpiresAt != "" {
		meta["expiresAt"] = in.ExpiresAt
	}
	if in.BaseVersionID != "" {
		meta["baseVersionId"] = in.BaseVersionID
	}
	mf := []map[string]string{}
	for _, f := range files {
		mf = append(mf, map[string]string{"path": f.path, "mimeType": mime.TypeByExtension(filepath.Ext(f.path))})
	}
	meta["files"] = mf
	// A private temporary multipart file bounds memory. Remove it on every
	// outcome, including cancellation. OpenRoot confines race-time path changes.
	f, e := os.CreateTemp("", "folio-upload-*")
	if e != nil {
		return nil, e
	}
	defer os.Remove(f.Name())
	defer f.Close()
	w := multipart.NewWriter(f)
	raw, _ := json.Marshal(meta)
	if e = w.WriteField("metadata", string(raw)); e != nil {
		return nil, e
	}
	for i, item := range files {
		if e = ctx.Err(); e != nil {
			return nil, e
		}
		part, e := w.CreateFormFile(fmt.Sprintf("file%d", i), filepath.Base(item.path))
		if e != nil {
			return nil, e
		}
		source, e := root.Open(filepath.FromSlash(item.path))
		if e != nil {
			return nil, e
		}
		stat, e := source.Stat()
		if e != nil || !stat.Mode().IsRegular() || stat.Size() != item.size {
			source.Close()
			return nil, errors.New("file changed while publishing; retry")
		}
		var copied int64
		copied, e = io.Copy(part, io.LimitReader(source, item.size+1))
		ce := source.Close()
		if e != nil {
			return nil, e
		}
		if ce != nil {
			return nil, ce
		}
		if copied != item.size {
			return nil, errors.New("file changed while publishing; retry")
		}
	}
	if e = w.Close(); e != nil {
		return nil, e
	}
	if _, e = f.Seek(0, 0); e != nil {
		return nil, e
	}
	endpoint := "/api/artifacts"
	if in.ArtifactID != "" {
		endpoint += "/" + url.PathEscape(in.ArtifactID) + "/versions"
	}
	return c.request(ctx, "POST", endpoint, w.FormDataContentType(), f)
}
