package server

import "testing"

func TestResolveContentURLModes(t *testing.T) {
	cases := []struct {
		content  string
		wantErr  bool
		pathMode bool
		prefix   string
		template string
	}{
		{"", false, true, "/c", "http://app.test/c/{artifact}"},
		{"http://app.test/c/{artifact}", false, true, "/c", "http://app.test/c/{artifact}"},
		{"http://app.test/view/c/{artifact}", false, true, "/view/c", "http://app.test/view/c/{artifact}"},
		{"http://{artifact}.content.test", false, false, "", "http://{artifact}.content.test"},
		{"http://other.test/c/{artifact}", true, false, "", ""},
		{"https://app.test/c/{artifact}", true, false, "", ""},
		{"http://app.test/{artifact}", true, false, "", ""},
		{"http://app.test/api/{artifact}", true, false, "", ""},
		{"http://app.test/s/{artifact}", true, false, "", ""},
		{"http://app.test/g/{artifact}", true, false, "", ""},
		{"http://app.test/assets/{artifact}", true, false, "", ""},
		{"http://app.test/c/{artifact}/x", true, false, "", ""},
		{"http://app.test/c/x{artifact}", true, false, "", ""},
		{"http://app.test/c/{artifact}?q=1", true, false, "", ""},
		{"http://app.test/c/{artifact}/{artifact}", true, false, "", ""},
		{"http://app.test/View/{artifact}", true, false, "", ""},
	}
	for _, tc := range cases {
		got, e := ResolveContentURL("http://app.test", tc.content)
		if tc.wantErr {
			if e == nil {
				t.Errorf("%q: accepted %+v", tc.content, got)
			}
			continue
		}
		if e != nil {
			t.Errorf("%q: %v", tc.content, e)
			continue
		}
		if got.PathMode != tc.pathMode || got.Prefix != tc.prefix || got.Template != tc.template {
			t.Errorf("%q: got %+v", tc.content, got)
		}
	}
}

func TestResolveContentURLKeepsHostModeRules(t *testing.T) {
	if _, e := ResolveContentURL("http://app.test", "http://{artifact}.app.test"); e != nil {
		t.Fatal("distinct artifact host rejected", e)
	}
	if _, e := ResolveContentURL("http://app.test", "http://app.test"); e == nil {
		t.Fatal("template without placeholder accepted")
	}
	if _, e := ResolveContentURL("http://app.test", "http://{artifact}.content.test/x"); e == nil {
		t.Fatal("host template with path accepted")
	}
}
