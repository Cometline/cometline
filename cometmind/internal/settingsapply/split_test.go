package settingsapply_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/cometline/cometmind/internal/settingsapply"
)

func TestSplitMergeRoundTrip(t *testing.T) {
	merged := map[string]any{
		"providers": []any{map[string]any{"id": "openai"}},
		"appearance": map[string]any{
			"heroComposer": map[string]any{"presetId": "blue"},
		},
		"shortcuts": map[string]any{"toggleMiniWindow": map[string]any{"key": "j"}},
		"app":       map[string]any{"personaId": "minako", "openAtLogin": false},
		"cometmind": map[string]any{
			"systemPromptPath": "/tmp/SOUL.md",
			"memory":           map[string]any{"enabled": true},
		},
	}
	settings, desktop := settingsapply.SplitDocument(merged)
	if _, ok := settings["appearance"]; ok {
		t.Fatal("appearance should not remain in settings")
	}
	if desktop["app"].(map[string]any)["personaId"] != "minako" {
		t.Fatalf("desktop app = %#v", desktop["app"])
	}
	if settings["cometmind"].(map[string]any)["systemPromptPath"] != "/tmp/SOUL.md" {
		t.Fatal("settings should keep systemPromptPath stamp")
	}
	if desktop["systemPromptPath"] != "/tmp/SOUL.md" {
		t.Fatal("desktop should own systemPromptPath")
	}

	again := settingsapply.MergeDocuments(settings, desktop)
	if again["app"].(map[string]any)["personaId"] != "minako" {
		t.Fatalf("merged app = %#v", again["app"])
	}
	if again["cometmind"].(map[string]any)["memory"].(map[string]any)["enabled"] != true {
		t.Fatal("runtime memory lost")
	}
}

func TestMigrateSplitFilesIfNeeded(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("COMETMIND_DATA_DIR", dir)
	settingsPath := filepath.Join(dir, "cometline-settings.json")
	monolith := map[string]any{
		"providers": []any{},
		"appearance": map[string]any{
			"heroComposer": map[string]any{"presetId": "rose"},
		},
		"app": map[string]any{"personaId": "minako"},
		"cometmind": map[string]any{
			"systemPromptPath": "/soul.md",
			"maxTokens":        2048,
		},
	}
	raw, _ := json.MarshalIndent(monolith, "", "  ")
	if err := os.WriteFile(settingsPath, append(raw, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := settingsapply.MigrateSplitFilesIfNeeded(); err != nil {
		t.Fatal(err)
	}

	settingsRaw, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatal(err)
	}
	var settingsDoc map[string]any
	if err := json.Unmarshal(settingsRaw, &settingsDoc); err != nil {
		t.Fatal(err)
	}
	if settingsapply.HasDesktopKeys(settingsDoc) {
		t.Fatalf("settings still has desktop keys: %#v", settingsDoc)
	}

	desktopRaw, err := os.ReadFile(filepath.Join(dir, "cometline-desktop.json"))
	if err != nil {
		t.Fatal(err)
	}
	var desktopDoc map[string]any
	if err := json.Unmarshal(desktopRaw, &desktopDoc); err != nil {
		t.Fatal(err)
	}
	if desktopDoc["appearance"] == nil || desktopDoc["app"] == nil {
		t.Fatalf("desktop missing peeled keys: %#v", desktopDoc)
	}

	// Idempotent.
	if err := settingsapply.MigrateSplitFilesIfNeeded(); err != nil {
		t.Fatal(err)
	}
}

func TestDesktopKeysInPatch(t *testing.T) {
	keys := settingsapply.DesktopKeysInPatch(map[string]any{
		"appearance": map[string]any{},
		"cometmind":  map[string]any{"maxTokens": 1},
	})
	if len(keys) != 1 || keys[0] != "appearance" {
		t.Fatalf("keys=%v", keys)
	}
}
