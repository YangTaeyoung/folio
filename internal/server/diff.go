package server

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"sort"
	"strings"
	"unicode/utf8"
)

type DiffLine struct {
	Kind string `json:"kind"`
	Text string `json:"text"`
}
type FileDiff struct {
	Path        string     `json:"path"`
	Status      string     `json:"status"`
	BeforeBytes int64      `json:"beforeBytes"`
	AfterBytes  int64      `json:"afterBytes"`
	BeforeHash  string     `json:"beforeHash,omitempty"`
	AfterHash   string     `json:"afterHash,omitempty"`
	Lines       []DiffLine `json:"lines,omitempty"`
	TextOmitted bool       `json:"textOmitted,omitempty"`
}

func (s *Server) fileDigest(ctx context.Context, v Version, f File) (string, error) {
	store, e := s.store(v.BackendID)
	if e != nil {
		return "", e
	}
	rd, e := store.Open(ctx, f.Key)
	if e != nil {
		return "", e
	}
	defer rd.Close()
	h := sha256.New()
	if _, e = io.Copy(h, rd); e != nil {
		return "", e
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
func (s *Server) diffText(ctx context.Context, v Version, f File) (string, bool) {
	if f.Size > 256<<10 {
		return "", false
	}
	store, e := s.store(v.BackendID)
	if e != nil {
		return "", false
	}
	rd, e := store.Open(ctx, f.Key)
	if e != nil {
		return "", false
	}
	defer rd.Close()
	raw, e := io.ReadAll(io.LimitReader(rd, (256<<10)+1))
	if e != nil || len(raw) > 256<<10 || !utf8.Valid(raw) || strings.ContainsRune(string(raw), 0) {
		return "", false
	}
	return strings.ReplaceAll(string(raw), "\r\n", "\n"), true
}

// LCS is bounded to 1,000 lines on each side; larger files retain full hashes.
func lineDiff(a, b string) ([]DiffLine, bool) {
	aa := strings.Split(a, "\n")
	bb := strings.Split(b, "\n")
	if len(aa) > 1000 || len(bb) > 1000 {
		return nil, false
	}
	cols := len(bb) + 1
	grid := make([]uint16, (len(aa)+1)*cols)
	for i := len(aa) - 1; i >= 0; i-- {
		for j := len(bb) - 1; j >= 0; j-- {
			if aa[i] == bb[j] {
				grid[i*cols+j] = 1 + grid[(i+1)*cols+j+1]
			} else {
				x, y := grid[(i+1)*cols+j], grid[i*cols+j+1]
				if x > y {
					grid[i*cols+j] = x
				} else {
					grid[i*cols+j] = y
				}
			}
		}
	}
	lines := []DiffLine{}
	i, j := 0, 0
	for i < len(aa) || j < len(bb) {
		if i < len(aa) && j < len(bb) && aa[i] == bb[j] {
			lines = append(lines, DiffLine{"context", aa[i]})
			i++
			j++
		} else if j < len(bb) && (i == len(aa) || grid[i*cols+j+1] >= grid[(i+1)*cols+j]) {
			lines = append(lines, DiffLine{"added", bb[j]})
			j++
		} else {
			lines = append(lines, DiffLine{"removed", aa[i]})
			i++
		}
	}
	return lines, true
}
func (s *Server) diff(w http.ResponseWriter, r *http.Request, p principal, a Artifact) error {
	if r.Method != "GET" {
		return fail(405, "method_not_allowed", "Use GET for version comparisons")
	}
	if e := s.require(r, p, false, "read"); e != nil {
		return e
	}
	from, ok := s.version(a, r.URL.Query().Get("from"))
	if !ok || !s.readVersion(p, a, from) {
		return fail(404, "not_found", "Version not found")
	}
	to, ok := s.version(a, r.URL.Query().Get("to"))
	if !ok || !s.readVersion(p, a, to) {
		return fail(404, "not_found", "Version not found")
	}
	left, right := map[string]File{}, map[string]File{}
	names := []string{}
	for _, f := range from.Files {
		left[f.Path] = f
		names = append(names, f.Path)
	}
	for _, f := range to.Files {
		right[f.Path] = f
		if _, ok := left[f.Path]; !ok {
			names = append(names, f.Path)
		}
	}
	sort.Strings(names)
	files := []FileDiff{}
	summary := map[string]int{"added": 0, "removed": 0, "modified": 0, "unchanged": 0}
	selected := r.URL.Query().Get("path")
	for _, name := range names {
		l, lok := left[name]
		rr, rok := right[name]
		item := FileDiff{Path: name, BeforeBytes: l.Size, AfterBytes: rr.Size}
		var e error
		if lok {
			item.BeforeHash, e = s.fileDigest(r.Context(), from, l)
			if e != nil {
				return fail(502, "storage_failed", "Cannot compare stored content")
			}
		}
		if rok {
			item.AfterHash, e = s.fileDigest(r.Context(), to, rr)
			if e != nil {
				return fail(502, "storage_failed", "Cannot compare stored content")
			}
		}
		switch {
		case !lok:
			item.Status = "added"
		case !rok:
			item.Status = "removed"
		case item.BeforeHash == item.AfterHash:
			item.Status = "unchanged"
		default:
			item.Status = "modified"
		}
		summary[item.Status]++
		if selected == name && item.Status != "unchanged" {
			before, after := "", ""
			lb, rb := true, true
			if lok {
				before, lb = s.diffText(r.Context(), from, l)
			}
			if rok {
				after, rb = s.diffText(r.Context(), to, rr)
			}
			if lb && rb {
				item.Lines, lb = lineDiff(before, after)
			}
			item.TextOmitted = !lb || !rb
		}
		files = append(files, item)
	}
	if selected != "" {
		if _, l := left[selected]; !l {
			if _, rr := right[selected]; !rr {
				return fail(404, "not_found", "File not found")
			}
		}
	}
	writeJSON(w, 200, map[string]any{"from": s.publicVersion(from), "to": s.publicVersion(to), "files": files, "summary": summary, "textLimitBytes": 256 << 10, "textLimitLines": 1000, "previewHosts": s.previewHosts(a, from.ID, to.ID)})
	return nil
}

// previewHosts lists the content hostnames that version previews need routed; path mode serves
// previews on the management host and needs none.
func (s *Server) previewHosts(a Artifact, versionIDs ...string) []string {
	hosts := []string{}
	if s.pathMode {
		return hosts
	}
	for _, id := range versionIDs {
		hosts = append(hosts, s.canonicalVersion(a, id).Host)
	}
	return hosts
}
