package provider

import (
	"fmt"

	cometsdk "github.com/cometline/comet-sdk"
	"github.com/cometline/comet-sdk/provider/anthropic"
	"github.com/cometline/comet-sdk/provider/codex"
	"github.com/cometline/comet-sdk/provider/openai"
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
	case config.ProviderAnthropic, config.ProviderOpenAI, config.ProviderOpenAICompat, config.ProviderOpencodeGo, config.ProviderCodex:
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
	default:
		return method
	}
}

// New returns a concrete SDK provider based on [config.Config.Provider].
func New(cfg *config.Config) (cometsdk.Provider, error) {
	return NewFor(cfg, cfg.Provider)
}

// NewFor returns a concrete SDK provider for a specific provider id.
func NewFor(cfg *config.Config, id string) (cometsdk.Provider, error) {
	entry, method, baseURL := providerConfigFor(cfg, id)
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
		return codex.NewCodexProvider(opts...), nil
	default:
		return nil, fmt.Errorf("unknown provider method %q", method)
	}
}
