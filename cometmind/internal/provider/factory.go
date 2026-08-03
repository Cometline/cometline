package provider

import (
	"fmt"

	cometsdk "github.com/cometline/comet-sdk"
	"github.com/cometline/comet-sdk/provider/anthropic"
	"github.com/cometline/comet-sdk/provider/codex"
	"github.com/cometline/comet-sdk/provider/openai"
	"github.com/cometline/comet-sdk/provider/openairesponses"
	"github.com/cometline/comet-sdk/provider/xai"
	"github.com/cometline/cometmind/internal/config"
	"github.com/cometline/cometmind/internal/logging"
	"github.com/cometline/cometmind/internal/modelcatalog"
)

// providerConfigFor returns the resolved provider entry, method, and base URL
// for the requested provider id. If the id matches a provider in the
// multi-provider config array, that entry is used. If the id is a known method
// (legacy single-provider session), the top-level base URL is used with that
// method. Otherwise it falls back to the active provider configured at the top
// level.
func providerConfigFor(cfg *config.Config, id string) (*config.ProviderEntry, string, string) {
	if p := cfg.FindProvider(id); p != nil {
		baseURL := p.BaseURL
		if baseURL == "" {
			baseURL = cfg.BaseURL
		}
		return p, p.Method, baseURL
	}

	// Legacy sessions may store the method name as the provider id.
	switch id {
	case config.ProviderAnthropic, config.ProviderOpenAI, config.ProviderOpenAICompat, config.ProviderOpencodeGo, config.ProviderCodex, config.ProviderXAI, config.ProviderOllama:
		return nil, id, cfg.BaseURL
	}

	// Fall back to the active provider.
	return nil, cfg.Provider, cfg.BaseURL
}

// sdkProviderID maps a Cometline provider method to the comet-sdk provider.
func sdkProviderID(method string) string {
	switch method {
	case config.ProviderAnthropic:
		return config.ProviderAnthropic
	case config.ProviderOpenAI, config.ProviderOpenAICompat, config.ProviderOpencodeGo, config.ProviderOllama:
		return config.ProviderOpenAI
	case config.ProviderCodex:
		return config.ProviderCodex
	case config.ProviderXAI:
		return config.ProviderXAI
	default:
		return method
	}
}

// SDKFamily resolves a Cometline provider id (or legacy method name) to the
// comet-sdk provider family it maps to: one of config.ProviderAnthropic,
// config.ProviderOpenAI, or config.ProviderCodex. It mirrors the resolution
// New/NewForModel perform so callers can reason about provider capabilities
// (e.g. reasoning replay support) without constructing a provider.
func SDKFamily(cfg *config.Config, id string) string {
	_, method, _ := providerConfigFor(cfg, id)
	return sdkProviderID(method)
}

// CompatibilityEndpoint returns the stable cache key component for a configured
// provider endpoint. Providers with an implicit SDK default share "default".
func CompatibilityEndpoint(cfg *config.Config, id string) string {
	_, _, baseURL := providerConfigFor(cfg, id)
	if baseURL == "" {
		return "default"
	}
	return cometsdk.NormaliseBaseURL(baseURL)
}

// New returns a concrete SDK provider based on [config.Config.Provider].
func New(cfg *config.Config) (cometsdk.Provider, error) {
	return NewForModel(cfg, cfg.Provider, cfg.Model)
}

// NewMemoryLLM returns the provider used for memory compaction and default
// extraction/update LLM calls. It respects Memory.ExtractionProvider when set
// so compaction does not send a pinned extraction model to the wrong backend.
func NewMemoryLLM(cfg *config.Config) (cometsdk.Provider, error) {
	providerID, model := cfg.ExtractionLLM()
	return NewForModel(cfg, providerID, model)
}

// NewFor returns a concrete SDK provider for a specific provider id using the
// provider entry's primary model. Kept for callers that only know the provider
// id; prefer NewForModel when the model is known.
func NewFor(cfg *config.Config, id string) (cometsdk.Provider, error) {
	entry, _, _ := providerConfigFor(cfg, id)
	modelID := ""
	if entry != nil {
		modelID = entry.Model
	}
	return NewForModel(cfg, id, modelID)
}

// NewForModel returns a concrete SDK provider for a specific provider id and
// model. Model-aware dispatch matters for opencode-go, whose models can speak
// different wire protocols (Chat Completions, Anthropic Messages, or OpenAI
// Responses) depending on their models.dev metadata.
func NewForModel(cfg *config.Config, id, modelID string) (cometsdk.Provider, error) {
	entry, method, baseURL := providerConfigFor(cfg, id)
	// No provider is configured at all (fresh install / user cleared settings).
	// Surface a clear, actionable error instead of a confusing "unknown
	// provider method" or TCP connection failure.
	if entry == nil && method == "" {
		return nil, fmt.Errorf("no provider configured — open Settings to add a provider and model")
	}
	key, err := config.ProviderAPIKey(cfg, entry, method)
	if err != nil {
		return nil, err
	}
	if method == config.ProviderOllama {
		baseURL = OllamaChatBaseURL(baseURL)
		if key == "" {
			// Ollama ignores auth; keep a non-empty bearer for the OpenAI client.
			key = "ollama"
		}
	}

	if method == config.ProviderOpencodeGo {
		return opencodeGoProvider(key, id, baseURL, modelID)
	}

	var opts []cometsdk.Option
	if baseURL != "" {
		opts = append(opts, cometsdk.WithBaseURL(baseURL))
	}
	switch sdkProviderID(method) {
	case config.ProviderAnthropic:
		return anthropic.NewAnthropicProvider(key, opts...), nil
	case config.ProviderOpenAI:
		return openai.NewOpenAIProvider(key, opts...), nil
	case config.ProviderCodex:
		return codex.NewCodexProvider(opts...), nil
	case config.ProviderXAI:
		return xai.NewXAIProvider(key, opts...), nil
	default:
		return nil, fmt.Errorf("unknown provider method %q", method)
	}
}

// opencodeGoProvider dispatches an OpenCode Go model to the SDK provider that
// speaks its wire protocol. Protocol metadata comes from models.dev; when it
// is unavailable the documented OpenCode Go default (Chat Completions) is used
// so existing models keep working.
func opencodeGoProvider(key, id, baseURL, modelID string) (cometsdk.Provider, error) {
	protocol := modelcatalog.ResolveProviderMetadata(config.ProviderOpencodeGo, id, modelID)
	logging.L().Debug("provider.opencode-go.protocol",
		"provider_id", id, "model", modelID, "npm", protocol.NPM, "source", protocol.Source)
	if baseURL == "" {
		baseURL = protocol.API
	}
	var opts []cometsdk.Option
	if baseURL != "" {
		opts = append(opts, cometsdk.WithBaseURL(baseURL))
	}
	if modelcatalog.RequiresEmptyReasoningContentReplay(config.ProviderOpencodeGo, id, modelID) {
		opts = append(opts, cometsdk.WithPreserveEmptyReasoningContent())
	}
	switch protocol.NPM {
	case modelcatalog.NPMOpenAI:
		return openairesponses.NewOpenAIResponsesProvider(key, id, opts...), nil
	case modelcatalog.NPMAnthropic:
		return anthropic.NewAnthropicProvider(key, opts...), nil
	default:
		return openai.NewOpenAIProvider(key, opts...), nil
	}
}
