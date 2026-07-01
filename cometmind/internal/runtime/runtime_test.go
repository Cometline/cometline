package runtime

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/cometline/cometmind/internal/config"
	"github.com/cometline/cometmind/internal/session"
)

func TestRuntimeWiresServiceAndRunner(t *testing.T) {
	ctx := context.Background()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("ANTHROPIC_API_KEY", "test-key")

	rt, err := New(ctx)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer rt.Close()

	if rt.Config == nil {
		t.Fatal("runtime config is nil")
	}
	if rt.Sessions == nil {
		t.Fatal("runtime sessions is nil")
	}

	// A config file should have been written to the temp home.
	if _, err := os.Stat(filepath.Join(home, ".cometmind", "cometline-settings.json")); err != nil {
		t.Fatalf("expected default settings file: %v", err)
	}

	ws, err := rt.WorkspaceForCommand(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("WorkspaceForCommand() error = %v", err)
	}
	sess, err := rt.Sessions.NewSession(ctx, ws.ID, rt.Config.Model, rt.Config.Provider)
	if err != nil {
		t.Fatalf("NewSession() error = %v", err)
	}

	// RunnerFor is the wiring that historically each command duplicated.
	runner, err := rt.RunnerFor(sess, ws.Path)
	if err != nil {
		t.Fatalf("RunnerFor() error = %v", err)
	}
	if runner == nil {
		t.Fatal("RunnerFor() returned nil")
	}
}

func TestRuntimeNewDoesNotRequireAPIKey(t *testing.T) {
	ctx := context.Background()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("COMETMIND_API_KEY", "")

	rt, err := New(ctx)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer rt.Close()

	if rt.Sessions == nil {
		t.Fatal("runtime sessions is nil")
	}
}

func TestRuntimeReloadUpdatesSystemPromptFromSettings(t *testing.T) {
	ctx := context.Background()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("ANTHROPIC_API_KEY", "test-key")

	soulA := filepath.Join(home, "SOUL_A.md")
	soulB := filepath.Join(home, "SOUL_B.md")
	if err := os.WriteFile(soulA, []byte("persona A soul\n"), 0o600); err != nil {
		t.Fatalf("write soul A: %v", err)
	}
	if err := os.WriteFile(soulB, []byte("persona B soul\n"), 0o600); err != nil {
		t.Fatalf("write soul B: %v", err)
	}

	settingsPath := filepath.Join(home, ".cometmind", "cometline-settings.json")
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o700); err != nil {
		t.Fatalf("mkdir settings dir: %v", err)
	}
	writeSettings := func(promptPath string) {
		t.Helper()
		body := []byte(`{"providers":[{"id":"anthropic","name":"Anthropic","method":"anthropic","enabled":true,"apiKey":"test-key","selectedModel":"claude-sonnet-4-5","models":["claude-sonnet-4-5"],"enabledModels":["claude-sonnet-4-5"]}],"activeProviderId":"anthropic","cometmind":{"systemPromptPath":"` + promptPath + `"}}`)
		if err := os.WriteFile(settingsPath, body, 0o600); err != nil {
			t.Fatalf("write settings: %v", err)
		}
	}
	writeSettings(soulA)

	rt, err := New(ctx)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer rt.Close()
	if rt.SystemPrompt != "persona A soul" {
		t.Fatalf("SystemPrompt = %q, want persona A soul", rt.SystemPrompt)
	}

	writeSettings(soulB)
	if err := rt.Reload(ctx); err != nil {
		t.Fatalf("Reload() error = %v", err)
	}
	if rt.SystemPrompt != "persona B soul" {
		t.Fatalf("after Reload SystemPrompt = %q, want persona B soul", rt.SystemPrompt)
	}

	ws, err := rt.WorkspaceForCommand(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("WorkspaceForCommand() error = %v", err)
	}
	sess, err := rt.Sessions.NewSession(ctx, ws.ID, rt.Config.Model, rt.Config.Provider)
	if err != nil {
		t.Fatalf("NewSession() error = %v", err)
	}
	runner, err := rt.RunnerFor(sess, ws.Path)
	if err != nil {
		t.Fatalf("RunnerFor() error = %v", err)
	}
	if runner.SystemPrompt != "persona B soul" {
		t.Fatalf("runner.SystemPrompt = %q, want persona B soul", runner.SystemPrompt)
	}
}

func TestRuntimeLoadsSystemPromptFromConfiguredPath(t *testing.T) {
	ctx := context.Background()
	home := t.TempDir()
	promptPath := filepath.Join(home, "SOUL.md")
	t.Setenv("HOME", home)
	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("COMETMIND_SYSTEM_PROMPT_PATH", promptPath)
	if err := os.WriteFile(promptPath, []byte("  custom soul prompt\n"), 0o600); err != nil {
		t.Fatalf("write prompt: %v", err)
	}

	rt, err := New(ctx)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer rt.Close()

	if rt.SystemPrompt != "custom soul prompt" {
		t.Fatalf("SystemPrompt = %q, want custom soul prompt", rt.SystemPrompt)
	}

	ws, err := rt.WorkspaceForCommand(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("WorkspaceForCommand() error = %v", err)
	}
	sess, err := rt.Sessions.NewSession(ctx, ws.ID, rt.Config.Model, rt.Config.Provider)
	if err != nil {
		t.Fatalf("NewSession() error = %v", err)
	}
	runner, err := rt.RunnerFor(sess, ws.Path)
	if err != nil {
		t.Fatalf("RunnerFor() error = %v", err)
	}
	if runner.SystemPrompt != "custom soul prompt" {
		t.Fatalf("runner.SystemPrompt = %q, want custom soul prompt", runner.SystemPrompt)
	}
}

func TestRuntimeProviderForSessionUsesSessionIdentifiers(t *testing.T) {
	ctx := context.Background()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("ANTHROPIC_API_KEY", "test-key")

	rt, err := New(ctx)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer rt.Close()

	// ProviderForSession should copy session model/provider into the config
	// passed to the provider factory, without mutating rt.Config.
	origModel := rt.Config.Model
	sess := session.Session{ModelID: "other-model", ProviderID: "other-provider"}

	// Unknown provider id falls back to the legacy top-level config, which is
	// Anthropic in the default config, so the call succeeds.
	_, err = rt.ProviderForSession(sess)
	if err != nil {
		t.Fatalf("ProviderForSession() error = %v", err)
	}
	if rt.Config.Model != origModel {
		t.Fatalf("runtime config mutated: model = %q, want %q", rt.Config.Model, origModel)
	}
}

func TestRuntimeProviderForSessionUsesMultiProviderEntry(t *testing.T) {
	ctx := context.Background()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("OPENAI_API_KEY", "openai-key")

	rt, err := New(ctx)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer rt.Close()

	// Inject a multi-provider entry for an OpenAI-compatible endpoint.
	rt.Config.Providers = []config.ProviderEntry{{
		ID:      "local-llm",
		Name:    "Local LLM",
		Method:  config.ProviderOpenAICompat,
		BaseURL: "http://localhost:11434/v1",
		APIKey:  "ignored",
		Model:   "llama3",
	}}
	sess := session.Session{ModelID: "qwen2.5", ProviderID: "local-llm"}

	_, err = rt.ProviderForSession(sess)
	if err != nil {
		t.Fatalf("ProviderForSession() error = %v", err)
	}
}
