package settingsapply_test

import (
	"testing"

	"github.com/Cometline/cometline/cometmind/internal/settingsapply"
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

func TestDesktopKeysInPatch(t *testing.T) {
	keys := settingsapply.DesktopKeysInPatch(map[string]any{
		"appearance": map[string]any{},
		"cometmind":  map[string]any{"maxTokens": 1},
	})
	if len(keys) != 1 || keys[0] != "appearance" {
		t.Fatalf("keys=%v", keys)
	}
}
