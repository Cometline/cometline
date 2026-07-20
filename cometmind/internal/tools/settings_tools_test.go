package tools_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/cometline/cometmind/internal/settingsapply"
	"github.com/cometline/cometmind/internal/tools"
)

type stubReload struct {
	calls int
}

func (s *stubReload) Reload(context.Context) error {
	s.calls++
	return nil
}

func TestSettingsToolsParentOnlyAndRedactionRoundTrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("COMETMIND_DATA_DIR", dir)
	settingsPath := filepath.Join(dir, "cometline-settings.json")
	initial := map[string]any{
		"providers": []any{
			map[string]any{
				"id":            "openai",
				"name":          "OpenAI",
				"method":        "openai",
				"enabled":       true,
				"baseURL":       "https://api.openai.com/v1",
				"apiKey":        "sk-secret",
				"selectedModel": "gpt-4o",
				"models":        []any{"gpt-4o"},
				"enabledModels": []any{"gpt-4o"},
			},
		},
		"defaultProviderId": "openai",
		"defaultModelId": "gpt-4o",
		"cometmind": map[string]any{
			"systemPromptPath":   "",
			"maxTokens":          4096,
			"contextWindowLimit": 0,
			"acp":                map[string]any{"defaultHarness": "opencode"},
			"skills":             map[string]any{"enabled": true},
			"memory": map[string]any{
				"enabled": false,
				"embedding": map[string]any{
					"provider": "openai",
					"model":    "text-embedding-3-small",
					"apiKey":   "",
				},
			},
			"storage": map[string]any{},
			"gateway": map[string]any{
				"discord": map[string]any{"enabled": false},
			},
			"mcp":       map[string]any{"enabled": false, "servers": []any{}},
			"jobs":      map[string]any{},
			"autonomy":  map[string]any{"enabled": false},
			"scheduler": map[string]any{"enabled": false},
		},
	}
	raw, err := json.MarshalIndent(initial, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(settingsPath, append(raw, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}

	stub := &stubReload{}
	r := tools.NewRegistry(dir, tools.RegistryOptions{SettingsRuntime: stub})
	for _, name := range []string{"list_settings", "get_settings", "patch_settings"} {
		if !r.Has(name) {
			t.Fatalf("parent registry missing %s", name)
		}
	}
	sub := tools.NewSubagentRegistry(dir, nil, tools.SubagentModeCoding)
	if sub.Has("patch_settings") {
		t.Fatal("coding subagent must not expose patch_settings")
	}

	getRes, err := r.Execute(context.Background(), "get_settings", []byte(`{"path":"providers"}`))
	if err != nil {
		t.Fatal(err)
	}
	if !getRes.OK {
		t.Fatalf("get_settings: %s", getRes.Output)
	}
	if !contains(getRes.Output, settingsapply.SecretSentinel) {
		t.Fatalf("expected redacted apiKey, got %s", getRes.Output)
	}
	if contains(getRes.Output, "sk-secret") {
		t.Fatal("raw api key leaked from get_settings")
	}

	patch := map[string]any{
		"patch": map[string]any{
			"providers": []any{
				map[string]any{
					"id":            "openai",
					"name":          "OpenAI",
					"method":        "openai",
					"enabled":       true,
					"baseURL":       "https://api.openai.com/v1",
					"apiKey":        settingsapply.SecretSentinel,
					"selectedModel": "gpt-4o",
					"models":        []any{"gpt-4o"},
					"enabledModels": []any{"gpt-4o"},
				},
			},
			"cometmind": map[string]any{
				"maxTokens": 2048,
			},
		},
	}
	patchRaw, _ := json.Marshal(patch)
	patchRes, err := r.Execute(context.Background(), "patch_settings", patchRaw)
	if err != nil {
		t.Fatal(err)
	}
	if !patchRes.OK {
		t.Fatalf("patch_settings: %s", patchRes.Output)
	}
	if stub.calls != 1 {
		t.Fatalf("reload calls=%d want 1", stub.calls)
	}

	written, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatal(err)
	}
	if !contains(string(written), "sk-secret") {
		t.Fatal("expected previous api key preserved on disk")
	}
	if !contains(string(written), "2048") {
		t.Fatal("expected maxTokens patch persisted")
	}

	deny, err := r.Execute(context.Background(), "patch_settings", []byte(`{"patch":{"appearance":{"heroComposer":{"presetId":"rose"}}}}`))
	if err != nil {
		t.Fatal(err)
	}
	if deny.OK {
		t.Fatal("patch_settings should reject appearance (desktop file)")
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 ||
		(func() bool {
			for i := 0; i+len(sub) <= len(s); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
			return false
		})())
}
