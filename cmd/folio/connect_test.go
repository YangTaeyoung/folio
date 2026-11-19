package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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
	launcher, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if len(folio) != 2 || folio["command"] != launcher || !filepath.IsAbs(launcher) || folio["args"].([]any)[0] != "mcp" {
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

func TestClientCLIsReceiveAbsoluteCredentialFreeArguments(t *testing.T) {
	for _, client := range []string{"codex", "claudecode", "antigravity", "vscode"} {
		t.Run(client, func(t *testing.T) {
			dir := t.TempDir()
			name := map[string]string{"codex": "codex", "claudecode": "claude", "antigravity": "agy", "vscode": "code"}[client]
			capture := filepath.Join(dir, "args")
			stub := filepath.Join(dir, name)
			if err := os.WriteFile(stub, []byte("#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$FOLIO_CONNECT_TEST_CAPTURE\"\n"), 0700); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", dir)
			t.Setenv("FOLIO_CONFIG_DIR", "")
			t.Setenv("FOLIO_CONNECT_TEST_CAPTURE", capture)
			if err := connect([]string{"--client", client}); err != nil {
				t.Fatal(err)
			}
			raw, err := os.ReadFile(capture)
			if err != nil {
				t.Fatal(err)
			}
			launcher, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(raw), launcher) || strings.Contains(string(raw), "FOLIO_TOKEN") {
				t.Fatalf("invalid launcher args: %s", raw)
			}
			if client == "vscode" {
				var configuration struct {
					Command string   `json:"command"`
					Args    []string `json:"args"`
				}
				lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
				if len(lines) != 2 || lines[0] != "--add-mcp" {
					t.Fatalf("args: %q", lines)
				}
				if err := json.Unmarshal([]byte(lines[1]), &configuration); err != nil {
					t.Fatal(err)
				}
				if configuration.Command != launcher || len(configuration.Args) != 1 || configuration.Args[0] != "mcp" {
					t.Fatal("invalid VS Code JSON")
				}
			}
		})
	}
}

func TestAntigravityDoesNotSilentlyIgnoreConfigDirectory(t *testing.T) {
	if err := connect([]string{"--client", "antigravity", "--config-dir", t.TempDir()}); err == nil {
		t.Fatal("unsupported isolation must fail before calling a client")
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

func TestOpenCodeConnectCreatesNativeConfig(t *testing.T) {
	dir := t.TempDir()
	if err := connect([]string{"--client", "opencode", "--config-dir", dir}); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "opencode.json"))
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err = json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	server := got["mcp"].(map[string]any)["folio"].(map[string]any)
	command := server["command"].([]any)
	if !filepath.IsAbs(command[0].(string)) || command[1] != "mcp" || server["type"] != "local" || server["enabled"] != true {
		t.Fatalf("unexpected launcher: %#v", server)
	}
	if err := connect([]string{"--client", "opencode", "--config-dir", dir}); err != nil {
		t.Fatal(err)
	}
}
