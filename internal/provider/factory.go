package provider

import (
	"fmt"

	cometsdk "github.com/cometline/comet-sdk"
	"github.com/cometline/comet-sdk/provider/anthropic"
	"github.com/cometline/comet-sdk/provider/openai"
	"github.com/cometline/cometmind/internal/config"
)

// New returns a concrete SDK provider based on [config.Config.Provider].
func New(cfg *config.Config) (cometsdk.Provider, error) {
	key, err := config.ProviderAPIKey(cfg)
	if err != nil {
		return nil, err
	}
	var opts []cometsdk.Option
	if cfg.BaseURL != "" {
		opts = append(opts, cometsdk.WithBaseURL(cfg.BaseURL))
	}
	switch cfg.Provider {
	case config.ProviderAnthropic:
		return anthropic.NewAnthropicProvider(key, opts...), nil
	case config.ProviderOpenAI:
		return openai.NewOpenAIProvider(key, opts...), nil
	default:
		return nil, fmt.Errorf("unknown provider %q", cfg.Provider)
	}
}
