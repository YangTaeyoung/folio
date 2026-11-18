package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// connect registers only a token-free launcher. Authentication stays in the
// Folio CLI config written by login, never in a third-party MCP configuration.
func connect(args []string) error {
	f := flags("connect")
	client := f.String("client", "", "Client to configure: cursor")
	directory := f.String("config-dir", os.Getenv("FOLIO_CONFIG_DIR"), "Client config directory (default: ~/.cursor for Cursor)")
	if err := f.Parse(args); err != nil {
		return err
	}
	if *client != "cursor" || f.NArg() != 0 {
		return errors.New("usage: folio connect --client cursor [--config-dir DIRECTORY]")
	}
	if *directory == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		*directory = filepath.Join(home, ".cursor")
	}
	path := filepath.Join(*directory, "mcp.json")
	changed, err := mergeCursorConfig(path)
	if err != nil {
		return err
	}
	status := "already_configured"
	if changed {
		status = "configured"
	}
	return output(map[string]string{"client": "cursor", "path": path, "status": status})
}

func mergeCursorConfig(path string) (bool, error) {
	info, err := os.Lstat(path)
	exists := err == nil
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return false, err
	}
	if exists && !info.Mode().IsRegular() {
		return false, errors.New("Cursor configuration must be a regular file; existing file was not changed")
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
			return false, errors.New("Cursor mcp.json must be a valid JSON object; existing file was not changed")
		}
	}
	servers := map[string]json.RawMessage{}
	if raw, ok := root["mcpServers"]; ok {
		if err = json.Unmarshal(raw, &servers); err != nil || servers == nil {
			return false, errors.New("Cursor mcpServers must be a JSON object; existing file was not changed")
		}
	}
	launcher := json.RawMessage(`{"command":"folio","args":["mcp"]}`)
	if existing, ok := servers["folio"]; ok {
		var got, want any
		if json.Unmarshal(existing, &got) == nil && json.Unmarshal(launcher, &want) == nil {
			a, _ := json.Marshal(got)
			b, _ := json.Marshal(want)
			if bytes.Equal(a, b) {
				return false, nil
			}
		}
		return false, errors.New("Cursor already has a different folio server; review it manually before connecting (existing file was not changed)")
	}
	servers["folio"] = launcher
	root["mcpServers"], err = json.Marshal(servers)
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
		return false, fmt.Errorf("Cursor configuration changed during setup; retry after reviewing %s", path)
	}
	if err = os.Rename(tmp.Name(), path); err != nil {
		return false, err
	}
	return true, nil
}
