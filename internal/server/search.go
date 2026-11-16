package server

import (
	"context"
	"io"
	"path"
	"strings"
	"unicode/utf8"

	"golang.org/x/net/html"
)

const maxSearchBytes = 2 << 20

// The bounded index stores rendered HTML text, never scripts, styles or attributes.
func searchableFile(f File) bool {
	switch strings.ToLower(path.Ext(f.Path)) {
	case ".html", ".htm", ".txt", ".md", ".csv", ".json", ".xml":
		return true
	}
	return strings.HasPrefix(f.MIMEType, "text/") && !strings.HasPrefix(f.MIMEType, "text/javascript") && !strings.HasSuffix(f.Path, ".css")
}
func visibleText(raw []byte, isHTML bool) string {
	if !utf8.Valid(raw) {
		// A byte-limited source may end inside a multi-byte character. Keep the
		// valid source prefix instead of discarding every preceding word.
		raw = []byte(strings.ToValidUTF8(string(raw), ""))
	}
	if !isHTML {
		return strings.Join(strings.Fields(string(raw)), " ")
	}
	z := html.NewTokenizer(strings.NewReader(string(raw)))
	var b strings.Builder
	skip := ""
	for {
		tt := z.Next()
		if tt == html.ErrorToken {
			break
		}
		switch tt {
		case html.StartTagToken:
			name, _ := z.TagName()
			tag := string(name)
			if tag == "script" || tag == "style" || tag == "template" {
				skip = tag
			}
		case html.EndTagToken:
			name, _ := z.TagName()
			if string(name) == skip {
				skip = ""
			}
		case html.TextToken:
			if skip == "" {
				b.Write(z.Text())
				b.WriteByte(' ')
			}
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}
func (s *Server) extractVersionText(ctx context.Context, v Version) string {
	store, e := s.store(v.BackendID)
	if e != nil {
		return ""
	}
	var b strings.Builder
	remaining := int64(maxSearchBytes)
	for _, f := range v.Files {
		if remaining <= 0 || ctx.Err() != nil {
			break
		}
		if !searchableFile(f) {
			continue
		}
		rd, e := store.Open(ctx, f.Key)
		if e != nil {
			continue
		}
		raw, e := io.ReadAll(io.LimitReader(rd, remaining))
		rd.Close()
		remaining -= int64(len(raw))
		if e != nil {
			continue
		}
		ext := strings.ToLower(path.Ext(f.Path))
		text := visibleText(raw, ext == ".html" || ext == ".htm" || strings.HasPrefix(f.MIMEType, "text/html"))
		b.WriteString(text)
		b.WriteByte(' ')
	}
	return strings.TrimSpace(b.String())
}
func (s *Server) matchArtifact(ctx context.Context, p principal, a Artifact, q string) (string, bool) {
	v, ok := s.version(a, s.effectiveVersionID(p, a))
	if !ok || !s.readVersion(p, a, v) {
		return "", false
	}
	text := v.SearchText
	if !v.SearchIndexed {
		text = s.extractVersionText(ctx, v)
		for i := range s.st.Versions[a.ID] {
			if s.st.Versions[a.ID][i].ID == v.ID {
				s.st.Versions[a.ID][i].SearchText = text
				s.st.Versions[a.ID][i].SearchIndexed = true
			}
		}
		s.save()
	}
	visible := s.publicArtifactFor(p, a)
	hay := strings.ToLower(visible.Title + " " + visible.Description + " " + strings.Join(visible.Tags, " ") + " " + text)
	terms := strings.Fields(q)
	for _, term := range terms {
		if !strings.Contains(hay, term) {
			return "", false
		}
	}
	runes := []rune(text)
	low := []rune(strings.ToLower(text))
	start := 0
	if len(terms) > 0 {
		needle := []rune(terms[0])
		for i := 0; i+len(needle) <= len(low); i++ {
			if string(low[i:i+len(needle)]) == string(needle) {
				start = i - 70
				if start < 0 {
					start = 0
				}
				break
			}
		}
	}
	end := start + 220
	if end > len(runes) {
		end = len(runes)
	}
	snippet := string(runes[start:end])
	if start > 0 {
		snippet = "…" + snippet
	}
	if end < len(runes) {
		snippet += "…"
	}
	return snippet, true
}
