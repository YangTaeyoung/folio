package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestCursorConnectPreservesOtherServersAndSettings(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mcp.json")
	original := `{"mcpServers":{"existing":{"command":"other","args":["--keep"],"env":{"SECRET":"fixture-only"}}},"otherSetting":{"keep":true}}`
	if err := os.WriteFile(path, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}
	changed, err := mergeCursorConfig(path)
	if err != nil || !changed {
		t.Fatalf("changed=%v err=%v", changed, err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err = json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	servers := got["mcpServers"].(map[string]any)
	if servers["existing"].(map[string]any)["env"].(map[string]any)["SECRET"] != "fixture-only" {
		t.Fatal("existing server data lost")
	}
	if got["otherSetting"].(map[string]any)["keep"] != true {
		t.Fatal("unrelated settings lost")
	}
	folio := servers["folio"].(map[string]any)
	if len(folio) != 2 || folio["command"] != "folio" || folio["args"].([]any)[0] != "mcp" {
		t.Fatalf("launcher = %#v", folio)
	}
	changed, err = mergeCursorConfig(path)
	if err != nil || changed {
		t.Fatalf("repeat changed=%v err=%v", changed, err)
	}
	after, _ := os.ReadFile(path)
	if string(after) != string(raw) {
		t.Fatal("idempotent registration rewrote config")
	}
}

func TestCursorConnectRefusesUnsafeExistingConfiguration(t *testing.T) {
	for _, input := range []string{``, `not json`, `null`, `[]`, `{"mcpServers":null}`, `{"mcpServers":[]}`, `{"mcpServers":{"folio":{"command":"different"}}}`, `{"mcpServers":{"folio":{"command":"folio","args":["mcp"],"env":{"FOLIO_TOKEN":"fixture"}}}}`} {
		t.Run(input, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "mcp.json")
			if err := os.WriteFile(path, []byte(input), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := mergeCursorConfig(path); err == nil {
				t.Fatal("expected refusal")
			}
			after, _ := os.ReadFile(path)
			if string(after) != input {
				t.Fatal("refusal changed existing file")
			}
		})
	}
}

func TestCursorConnectCreatesIsolatedConfigAndRejectsSymlink(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("FOLIO_CONFIG_DIR", filepath.Join(dir, "isolated-cursor"))
	if err := connect([]string{"--client", "cursor"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "isolated-cursor", "mcp.json")); err != nil {
		t.Fatal(err)
	}
	if err := connect([]string{"--client", "unknown"}); err == nil {
		t.Fatal("unknown client accepted")
	}
	path := filepath.Join(dir, "link.json")
	if err := os.Symlink(filepath.Join(dir, "isolated-cursor", "mcp.json"), path); err != nil {
		t.Skip(err)
	}
	if _, err := mergeCursorConfig(path); err == nil {
		t.Fatal("symlink accepted")
	}
}
