package server

import (
	"errors"
	"net/url"
	"strings"
)

// ContentLayout is the effective artifact content URL template and how it addresses artifacts.
// PathMode serves artifacts under Prefix on the management origin; otherwise each artifact has
// its own hostname.
type ContentLayout struct {
	Template string
	PathMode bool
	Prefix   string
}

var reservedContentPrefixes = map[string]bool{"api": true, "s": true, "g": true, "assets": true}

// ResolveContentURL validates contentURL against the management origin appURL. An empty
// contentURL selects path mode at <appURL>/c/{artifact}.
func ResolveContentURL(appURL, contentURL string) (ContentLayout, error) {
	appURL = strings.TrimRight(appURL, "/")
	a, e := url.Parse(appURL)
	if e != nil || a.Host == "" {
		return ContentLayout{}, errors.New("AppURL must be an HTTP(S) origin")
	}
	if contentURL == "" {
		contentURL = appURL + "/c/{artifact}"
	}
	if strings.Count(contentURL, "{artifact}") != 1 {
		return ContentLayout{}, errors.New("ContentURL must contain one {artifact} placeholder")
	}
	cu, e := url.Parse(strings.ReplaceAll(contentURL, "{artifact}", "sample"))
	if e != nil || cu.Host == "" || (cu.Scheme != "http" && cu.Scheme != "https") || cu.RawQuery != "" || cu.Fragment != "" || cu.User != nil {
		return ContentLayout{}, errors.New("ContentURL must be an HTTP(S) URL")
	}
	rest := strings.SplitN(contentURL, "://", 2)[1]
	authority := strings.SplitN(rest, "/", 2)[0]
	if strings.Contains(authority, "{artifact}") {
		if cu.Path != "" || cu.Hostname() == a.Hostname() {
			return ContentLayout{}, errors.New("ContentURL must be a distinct artifact-isolated HTTP(S) origin")
		}
		return ContentLayout{Template: contentURL}, nil
	}
	if cu.Scheme != a.Scheme || !strings.EqualFold(cu.Host, a.Host) {
		return ContentLayout{}, errors.New("ContentURL path mode must use the management origin")
	}
	prefix := strings.TrimSuffix(cu.Path, "/sample")
	if !strings.HasSuffix(contentURL, "/{artifact}") || prefix == "" || prefix == cu.Path || strings.Contains(prefix+"/", "//") {
		return ContentLayout{}, errors.New("ContentURL path placeholder must be the last whole segment after a non-empty prefix")
	}
	if reservedContentPrefixes[strings.SplitN(prefix[1:], "/", 2)[0]] {
		return ContentLayout{}, errors.New("ContentURL path prefix is reserved by the management app")
	}
	return ContentLayout{Template: contentURL, PathMode: true, Prefix: prefix}, nil
}
