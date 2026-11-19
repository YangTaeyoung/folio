package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// connect registers only a token-free launcher. Authentication stays in the
// Folio CLI config written by login, never in a third-party MCP configuration.
func connect(args []string) error {
	f := flags("connect")
	client := f.String("client", "", "Client: codex, claudecode, antigravity, vscode, cursor, opencode")
	directory := f.String("config-dir", os.Getenv("FOLIO_CONFIG_DIR"), "Optional client configuration directory")
	if err := f.Parse(args); err != nil {
		return err
	}
	if f.NArg() != 0 {
		return errors.New("usage: folio connect --client CLIENT [--config-dir DIRECTORY]")
	}
	launcher, err := os.Executable()
	if err != nil {
		return err
	}
	launcher, err = filepath.Abs(launcher)
	if err != nil {
		return err
	}
	if *client != "cursor" && *client != "opencode" {
		var name string
		var arguments, environment []string
		switch *client {
		case "codex":
			if *directory != "" {
				return errors.New("Codex configuration directory overrides are not supported")
			}
			name, arguments = "codex", []string{"mcp", "add", "folio", "--", launcher, "mcp"}
		case "claudecode":
			name, arguments = "claude", []string{"mcp", "add", "--scope", "user", "--transport", "stdio", "folio", "--", launcher, "mcp"}
			if *directory != "" {
				environment = append(environment, "CLAUDE_CONFIG_DIR="+*directory)
			}
		case "antigravity":
			if *directory != "" {
				return errors.New("Antigravity CLI does not support a verified isolated configuration directory; no settings were changed")
			}
			name, arguments = "agy", []string{"mcp", "add", "--type", "stdio", "folio", launcher, "mcp"}
		case "vscode":
			configuration, _ := json.Marshal(map[string]any{"name": "folio", "type": "stdio", "command": launcher, "args": []string{"mcp"}})
			name, arguments = "code", []string{"--add-mcp", string(configuration)}
			if *directory != "" {
				arguments = append([]string{"--user-data-dir", *directory}, arguments...)
			}
		default:
			return errors.New("choose codex, claudecode, antigravity, vscode, cursor, or opencode")
		}
		path, err := exec.LookPath(name)
		if err != nil {
			return fmt.Errorf("%s CLI was not found; install it and run setup again", name)
		}
		cmd := exec.Command(path, arguments...)
		cmd.Env = append(os.Environ(), environment...)
		cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("%s MCP registration failed: %w", name, err)
		}
		return nil
	}
	if *directory == "" && *client == "opencode" {
		if path := os.Getenv("OPENCODE_CONFIG"); path != "" {
			return configureJSONClient(*client, path, launcher)
		}
		base := os.Getenv("XDG_CONFIG_HOME")
		if base == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return err
			}
			base = filepath.Join(home, ".config")
		}
		*directory = filepath.Join(base, "opencode")
	}
	if *directory == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		*directory = filepath.Join(home, ".cursor")
	}
	filename := "mcp.json"
	if *client == "opencode" {
		filename = "opencode.json"
	}
	return configureJSONClient(*client, filepath.Join(*directory, filename), launcher)
}

func configureJSONClient(client, path, launcher string) error {
	key := "mcpServers"
	entry := map[string]any{"command": launcher, "args": []string{"mcp"}}
	if client == "opencode" {
		key = "mcp"
		entry = map[string]any{"type": "local", "command": []string{launcher, "mcp"}, "enabled": true}
		if _, err := os.Stat(path + "c"); err == nil {
			return errors.New("an OpenCode JSONC configuration exists; merge the Folio server into that file instead")
		}
	}
	value, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	changed, err := mergeClientConfig(path, key, value)
	if err != nil {
		return err
	}
	status := "already_configured"
	if changed {
		status = "configured"
	}
	return output(map[string]string{"client": client, "path": path, "status": status})
}

func mergeCursorConfig(path string) (bool, error) {
	launcher, err := os.Executable()
	if err != nil {
		return false, err
	}
	value, err := json.Marshal(map[string]any{"command": launcher, "args": []string{"mcp"}})
	if err != nil {
		return false, err
	}
	return mergeClientConfig(path, "mcpServers", value)
}

func mergeClientConfig(path, key string, launcher json.RawMessage) (bool, error) {
	info, err := os.Lstat(path)
	exists := err == nil
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return false, err
	}
	if exists && !info.Mode().IsRegular() {
		return false, errors.New("MCP configuration must be a regular file; existing file was not changed")
	}
	var original []byte
	if exists {
		original, err = os.ReadFile(path)
		if err != nil {
			return false, err
		}
	}
	root := map[string]json.RawMessage{}
	if exists {
		if err = json.Unmarshal(original, &root); err != nil || root == nil {
			return false, errors.New("MCP configuration must be a valid JSON object; existing file was not changed")
		}
	}
	servers := map[string]json.RawMessage{}
	if raw, ok := root[key]; ok {
		if err = json.Unmarshal(raw, &servers); err != nil || servers == nil {
			return false, errors.New("MCP server map must be a JSON object; existing file was not changed")
		}
	}
	if existing, ok := servers["folio"]; ok {
		var got, want any
		if json.Unmarshal(existing, &got) == nil && json.Unmarshal(launcher, &want) == nil {
			a, _ := json.Marshal(got)
			b, _ := json.Marshal(want)
			if bytes.Equal(a, b) {
				return false, nil
			}
		}
		return false, errors.New("This client already has a different folio server; review it manually before connecting (existing file was not changed)")
	}
	servers["folio"] = launcher
	root[key], err = json.Marshal(servers)
	if err != nil {
		return false, err
	}
	merged, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return false, err
	}
	merged = append(merged, '\n')
	if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return false, err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".folio-mcp-*")
	if err != nil {
		return false, err
	}
	defer os.Remove(tmp.Name())
	if err = tmp.Chmod(0600); err == nil {
		_, err = tmp.Write(merged)
	}
	if err == nil {
		err = tmp.Sync()
	}
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return false, err
	}
	// Refuse to replace a configuration changed while this command was preparing
	// its additive update. No recovery copy is needed because the old file stays
	// untouched until the final same-directory rename.
	current, readErr := os.ReadFile(path)
	if exists && (readErr != nil || !bytes.Equal(current, original)) || !exists && !errors.Is(readErr, os.ErrNotExist) {
		return false, fmt.Errorf("MCP configuration changed during setup; retry after reviewing %s", path)
	}
	if err = os.Rename(tmp.Name(), path); err != nil {
		return false, err
	}
	return true, nil
}
