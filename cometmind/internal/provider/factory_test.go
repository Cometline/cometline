package provider

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	cometsdk "github.com/cometline/comet-sdk"
	"github.com/cometline/cometmind/internal/config"
)

func TestNewForFallsBackToLegacyMethod(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "anthropic-key")

	cfg := &config.Config{
		Provider: config.ProviderOpenAI,
		BaseURL:  "http://example.com/v1",
		Providers: []config.ProviderEntry{{
			ID:      "my-openai",
			Method:  config.ProviderOpenAI,
			BaseURL: "http://example.com/v1",
			APIKey:  "openai-key",
		}},
	}

	// A legacy session stored "anthropic" as the provider id. There is no
	// matching provider entry, so the factory should treat it as the method and
	// resolve the Anthropic API key.
	p, err := NewFor(cfg, config.ProviderAnthropic)
	if err != nil {
		t.Fatalf("NewFor() error = %v", err)
	}
	if p == nil {
		t.Fatal("NewFor() returned nil")
	}
}

func TestNewForUsesMultiProviderEntry(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "entry-key")

	cfg := &config.Config{
		Provider: config.ProviderAnthropic,
		Providers: []config.ProviderEntry{{
			ID:      "local-llm",
			Name:    "Local LLM",
			Method:  config.ProviderOpenAICompat,
			BaseURL: "http://localhost:11434/v1",
			APIKey:  "entry-key",
			Model:   "qwen2.5",
		}},
	}

	p, err := NewFor(cfg, "local-llm")
	if err != nil {
		t.Fatalf("NewFor() error = %v", err)
	}
	if p == nil {
		t.Fatal("NewFor() returned nil")
	}
}

func TestNewForCodexDoesNotRequireAPIKey(t *testing.T) {
	cfg := &config.Config{
		Provider: config.ProviderCodex,
		Providers: []config.ProviderEntry{{
			ID:      "codex",
			Name:    "ChatGPT Codex",
			Method:  config.ProviderCodex,
			BaseURL: "https://chatgpt.com/backend-api/codex",
			Model:   "gpt-5.4",
		}},
	}

	p, err := NewFor(cfg, "codex")
	if err != nil {
		t.Fatalf("NewFor() error = %v", err)
	}
	if p == nil {
		t.Fatal("NewFor() returned nil")
	}
	if p.ID() != config.ProviderCodex {
		t.Fatalf("provider ID = %q, want %q", p.ID(), config.ProviderCodex)
	}
}

func TestNewForFallsBackToLegacyCodexMethod(t *testing.T) {
	p, err := NewFor(&config.Config{Provider: config.ProviderOpenAI}, config.ProviderCodex)
	if err != nil {
		t.Fatalf("NewFor() error = %v", err)
	}
	if p == nil {
		t.Fatal("NewFor() returned nil")
	}
}

func TestNewOpenAIProviderUsesConfiguredBaseURL(t *testing.T) {
	var gotPath string
	var gotAuth string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		_, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"ok\"}}]}\n\n")
		_, _ = io.WriteString(w, "data: {\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}]}\n\n")
		_, _ = io.WriteString(w, "data: {\"choices\":[],\"usage\":{\"prompt_tokens\":1,\"completion_tokens\":1}}\n\n")
		_, _ = io.WriteString(w, "data: [DONE]\n\n")
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
	}))
	defer srv.Close()

	t.Setenv("COMETMIND_API_KEY", "dummy-key")

	p, err := New(&config.Config{
		Provider: config.ProviderOpenAI,
		Model:    "test-model",
		BaseURL:  srv.URL,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	req := &cometsdk.Request{
		Model: "test-model",
		Messages: []cometsdk.Message{{
			Role:    cometsdk.RoleUser,
			Content: []cometsdk.Block{cometsdk.TextBlock{Text: "hello"}},
		}},
	}

	ch, err := p.Stream(context.Background(), req)
	if err != nil {
		t.Fatalf("Stream() error = %v", err)
	}
	for range ch {
	}

	if gotPath != "/v1/chat/completions" {
		t.Fatalf("request path = %q, want %q", gotPath, "/v1/chat/completions")
	}
	if gotAuth != "Bearer dummy-key" {
		t.Fatalf("authorization header = %q, want %q", gotAuth, "Bearer dummy-key")
	}
}
