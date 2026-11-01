package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRewriteSettingsMCPServersPreservesUnmodeledFields(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cometline-settings.json")
	initial := `{
  "providers": [],
  "cometmind": {
    "logLevel": "debug",
    "mcp": {
      "enabled": true,
      "servers": [
        {
          "id": "linear",
          "name": "Linear",
          "enabled": true,
          "transport": "http",
          "url": "https://mcp.linear.app/mcp",
          "args": null,
          "source": "catalog",
          "catalogId": "linear",
          "catalogVersion": "1"
        }
      ]
    }
  }
}
`
	if err := os.WriteFile(path, []byte(initial), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := loadCometlineSettingsJSON(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !containsJSONField(after, `"logLevel": "debug"`) {
		t.Fatalf("logLevel dropped from settings:\n%s", after)
	}
	_ = cfg
}

func TestRewriteSettingsMCPServersDoesNotLoopOnNullArgs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cometline-settings.json")
	initial := `{
  "cometmind": {
    "mcp": {
      "enabled": true,
      "servers": [
        {
          "id": "linear",
          "name": "Linear",
          "enabled": true,
          "transport": "http",
          "url": "https://mcp.linear.app/mcp",
          "args": null,
          "source": "catalog",
          "catalogId": "linear",
          "catalogVersion": "1"
        }
      ]
    }
  }
}
`
	if err := os.WriteFile(path, []byte(initial), 0o600); err != nil {
		t.Fatal(err)
	}
	first, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := loadCometlineSettingsJSON(path); err != nil {
		t.Fatalf("first load: %v", err)
	}
	second, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Fatalf("settings rewritten on no-op reload\nbefore:\n%s\nafter:\n%s", first, second)
	}
}

func containsJSONField(body []byte, needle string) bool {
	return strings.Contains(string(body), needle)
}
