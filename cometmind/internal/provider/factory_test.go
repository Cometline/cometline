package provider

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	cometsdk "github.com/cometline/comet-sdk"
	"github.com/cometline/cometmind/internal/config"
	"github.com/cometline/cometmind/internal/modelcatalog"
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

func TestNewMemoryLLMUsesExtractionProvider(t *testing.T) {
	loadProtocolFixture(t)

	cfg := &config.Config{
		Provider: config.ProviderCodex,
		Providers: []config.ProviderEntry{
			{ID: "codex", Method: config.ProviderCodex},
			{
				ID:      "opencode-go",
				Method:  config.ProviderOpencodeGo,
				APIKey:  "opencode-key",
				BaseURL: "http://example.com/v1",
				Model:   "qwen3.7-plus",
			},
		},
		Memory: config.MemoryConfig{
			ExtractionProvider: "opencode-go",
			ExtractionModel:    "qwen3.7-plus",
		},
	}
	p, err := NewMemoryLLM(cfg)
	if err != nil {
		t.Fatalf("NewMemoryLLM() error = %v", err)
	}
	if p == nil {
		t.Fatal("NewMemoryLLM() returned nil")
	}
	providerID, _ := cfg.ExtractionLLM()
	if family := SDKFamily(cfg, providerID); family != config.ProviderOpenAI {
		t.Fatalf("memory LLM family = %q, want openai (opencode-go), not active codex", family)
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

func TestNewForOllamaUsesOpenAIFamilyWithoutAPIKey(t *testing.T) {
	cfg := &config.Config{
		Provider: config.ProviderOllama,
		Providers: []config.ProviderEntry{{
			ID:      "ollama",
			Name:    "Ollama Local",
			Method:  config.ProviderOllama,
			BaseURL: "http://127.0.0.1:11434",
			Model:   "gemma4:e2b-mlx",
		}},
	}

	p, err := NewFor(cfg, "ollama")
	if err != nil {
		t.Fatalf("NewFor() error = %v", err)
	}
	if p == nil {
		t.Fatal("NewFor() returned nil")
	}
	if got := SDKFamily(cfg, "ollama"); got != config.ProviderOpenAI {
		t.Fatalf("SDKFamily = %q, want %q", got, config.ProviderOpenAI)
	}
	if got := OllamaChatBaseURL("http://127.0.0.1:11434"); got != "http://127.0.0.1:11434/v1" {
		t.Fatalf("OllamaChatBaseURL = %q", got)
	}
}

func TestNewForXAIUsesSubscriptionProviderWithoutAPIKey(t *testing.T) {
	cfg := &config.Config{
		Provider: config.ProviderXAI,
		Providers: []config.ProviderEntry{{
			ID:      "xai",
			Name:    "xAI Grok Subscription",
			Method:  config.ProviderXAI,
			BaseURL: "https://api.x.ai/v1",
		}},
	}

	p, err := NewFor(cfg, config.ProviderXAI)
	if err != nil {
		t.Fatalf("NewFor() error = %v", err)
	}
	if p == nil {
		t.Fatal("NewFor() returned nil")
	}
	if p.ID() != config.ProviderXAI {
		t.Fatalf("provider ID = %q, want %q", p.ID(), config.ProviderXAI)
	}
}

// TestNewForNoProviderConfigured covers a fresh install where the sidecar
// booted with no enabled providers. A request must surface a clear, actionable
// error instead of a confusing "unknown provider method" or a network failure.
func TestNewForNoProviderConfigured(t *testing.T) {
	cfg := &config.Config{} // no providers, empty active provider

	_, err := NewFor(cfg, "")
	if err == nil {
		t.Fatal("NewFor() error = nil, want error for empty provider config")
	}
	if !strings.Contains(err.Error(), "no provider configured") {
		t.Fatalf("NewFor() error = %q, want it to mention 'no provider configured'", err.Error())
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

func loadProtocolFixture(t *testing.T) {
	t.Helper()
	modelcatalog.ResetCacheForTest()
	data, err := os.ReadFile(filepath.Join("..", "modelcatalog", "testdata", "models-dev-protocol-snippet.json"))
	if err != nil {
		t.Fatalf("read protocol fixture: %v", err)
	}
	if err := modelcatalog.LoadFromJSONForTest(data); err != nil {
		t.Fatalf("load protocol fixture: %v", err)
	}
	t.Cleanup(modelcatalog.ResetCacheForTest)
}

func protocolRoutingServer(t *testing.T) (*httptest.Server, *[]string) {
	t.Helper()
	var paths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		_, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "text/event-stream")
		switch r.URL.Path {
		case "/v1/responses":
			_, _ = io.WriteString(w, "data: {\"type\":\"response.completed\",\"response\":{\"usage\":{}}}\n\n")
		case "/v1/messages":
			_, _ = io.WriteString(w, "event: message_delta\ndata: {\"delta\":{\"stop_reason\":\"end_turn\"}}\n\n")
			_, _ = io.WriteString(w, "event: message_stop\ndata: {}\n\n")
		default:
			_, _ = io.WriteString(w, "data: {\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}]}\n\n")
			_, _ = io.WriteString(w, "data: [DONE]\n\n")
		}
	}))
	t.Cleanup(srv.Close)
	return srv, &paths
}

func streamProviderRequest(t *testing.T, p cometsdk.Provider, modelID string) {
	t.Helper()
	ch, err := p.Stream(context.Background(), &cometsdk.Request{
		Model:    modelID,
		Messages: []cometsdk.Message{{Role: cometsdk.RoleUser, Content: []cometsdk.Block{cometsdk.TextBlock{Text: "hello"}}}},
	})
	if err != nil {
		t.Fatalf("Stream() error = %v", err)
	}
	for range ch {
	}
}

// TestNewForModelOpencodeGoRoutesByProtocol verifies model-level metadata
// selects the right wire protocol: Responses for gpt-5.6-luna, Chat
// Completions for the provider-default models, and Anthropic Messages for
// qwen3.7-plus.
func TestNewForModelOpencodeGoRoutesByProtocol(t *testing.T) {
	loadProtocolFixture(t)
	srv, paths := protocolRoutingServer(t)

	cfg := &config.Config{
		Providers: []config.ProviderEntry{{
			ID:      "opencode-go",
			Name:    "OpenCode Go",
			Method:  config.ProviderOpencodeGo,
			BaseURL: srv.URL,
			APIKey:  "opencode-key",
		}},
	}

	luna, err := NewForModel(cfg, "opencode-go", "gpt-5.6-luna")
	if err != nil {
		t.Fatalf("NewForModel(luna) error = %v", err)
	}
	streamProviderRequest(t, luna, "gpt-5.6-luna")

	chat, err := NewForModel(cfg, "opencode-go", "deepseek-v4-flash")
	if err != nil {
		t.Fatalf("NewForModel(deepseek) error = %v", err)
	}
	streamProviderRequest(t, chat, "deepseek-v4-flash")

	qwen, err := NewForModel(cfg, "opencode-go", "qwen3.7-plus")
	if err != nil {
		t.Fatalf("NewForModel(qwen) error = %v", err)
	}
	streamProviderRequest(t, qwen, "qwen3.7-plus")

	requirePaths(t, *paths, []string{"/v1/responses", "/v1/chat/completions", "/v1/messages"})
}

// TestNewForOpencodeGoEntryModelRoutesResponses verifies the compatibility
// NewFor entry point dispatches by the entry's primary model.
func TestNewForOpencodeGoEntryModelRoutesResponses(t *testing.T) {
	loadProtocolFixture(t)
	srv, paths := protocolRoutingServer(t)

	cfg := &config.Config{
		Providers: []config.ProviderEntry{{
			ID:      "opencode-go",
			Name:    "OpenCode Go",
			Method:  config.ProviderOpencodeGo,
			BaseURL: srv.URL,
			APIKey:  "opencode-key",
			Model:   "gpt-5.6-luna",
		}},
	}

	p, err := NewFor(cfg, "opencode-go")
	if err != nil {
		t.Fatalf("NewFor() error = %v", err)
	}
	streamProviderRequest(t, p, "gpt-5.6-luna")
	requirePaths(t, *paths, []string{"/v1/responses"})
}

// TestNewForModelOpencodeGoFallbackWithoutCatalog verifies the approved
// fallback: without models.dev metadata, opencode-go models use the documented
// Chat Completions default so existing models keep working.
func TestNewForModelOpencodeGoFallbackWithoutCatalog(t *testing.T) {
	modelcatalog.ResetCacheForTest()
	t.Cleanup(modelcatalog.ResetCacheForTest)
	modelcatalog.SetCachePathForTest(filepath.Join(t.TempDir(), "models-dev.json"))
	t.Cleanup(modelcatalog.ResetCachePathForTest)

	// A dead fetch URL guarantees catalog load fails so resolution falls back.
	dead := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "offline", http.StatusServiceUnavailable)
	}))
	defer dead.Close()
	modelcatalog.SetFetchURLForTest(dead.URL)
	t.Cleanup(func() { modelcatalog.SetFetchURLForTest(modelcatalog.APIURL) })

	srv, paths := protocolRoutingServer(t)

	cfg := &config.Config{
		Providers: []config.ProviderEntry{{
			ID:      "opencode-go",
			Name:    "OpenCode Go",
			Method:  config.ProviderOpencodeGo,
			BaseURL: srv.URL,
			APIKey:  "opencode-key",
		}},
	}

	p, err := NewForModel(cfg, "opencode-go", "gpt-5.6-luna")
	if err != nil {
		t.Fatalf("NewForModel() error = %v", err)
	}
	streamProviderRequest(t, p, "gpt-5.6-luna")
	requirePaths(t, *paths, []string{"/v1/chat/completions"})
}

func requirePaths(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("paths = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("paths = %v, want %v", got, want)
		}
	}
}
