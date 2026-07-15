package provider

import (
	"fmt"

	cometsdk "github.com/cometline/comet-sdk"
	"github.com/cometline/comet-sdk/provider/anthropic"
	"github.com/cometline/comet-sdk/provider/codex"
	"github.com/cometline/comet-sdk/provider/openai"
	"github.com/cometline/comet-sdk/provider/xai"
	"github.com/cometline/cometmind/internal/config"
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
	case config.ProviderAnthropic, config.ProviderOpenAI, config.ProviderOpenAICompat, config.ProviderOpencodeGo, config.ProviderCodex, config.ProviderXAI:
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
	case config.ProviderOpenAI, config.ProviderOpenAICompat, config.ProviderOpencodeGo:
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
// New/NewFor perform so callers can reason about provider capabilities (e.g.
// reasoning replay support) without constructing a provider.
func SDKFamily(cfg *config.Config, id string) string {
	_, method, _ := providerConfigFor(cfg, id)
	return sdkProviderID(method)
}

// New returns a concrete SDK provider based on [config.Config.Provider].
func New(cfg *config.Config) (cometsdk.Provider, error) {
	return NewFor(cfg, cfg.Provider)
}

// NewFor returns a concrete SDK provider for a specific provider id.
func NewFor(cfg *config.Config, id string) (cometsdk.Provider, error) {
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
		// Prefer SSE over the SDK's default WebSocket transport. Keep the
		// WebSocket implementation available for a future switch once it reliably
		// survives network changes such as Wi-Fi handoff.
		opts = append(opts, cometsdk.WithResponseTransport(cometsdk.ResponseTransportSSE))
		return codex.NewCodexProvider(opts...), nil
	case config.ProviderXAI:
		return xai.NewXAIProvider(key, opts...), nil
	default:
		return nil, fmt.Errorf("unknown provider method %q", method)
	}
}
