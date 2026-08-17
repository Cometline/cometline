package config

import (
	"strings"

	"github.com/cometline/cometmind/internal/generation"
)

// GenerationModelConfig is one Settings binding for image or video generation.
type GenerationModelConfig struct {
	ProviderID string `mapstructure:"provider_id"`
	Model      string `mapstructure:"model"`
}

// GenerationConfig holds the configured image and video generation models.
type GenerationConfig struct {
	Image GenerationModelConfig `mapstructure:"image"`
	Video GenerationModelConfig `mapstructure:"video"`
}

func defaultGenerationConfig() GenerationConfig {
	return GenerationConfig{
		Image: GenerationModelConfig{ProviderID: generation.DefaultProviderID, Model: generation.DefaultImageModel},
		Video: GenerationModelConfig{ProviderID: generation.DefaultProviderID, Model: generation.DefaultVideoModel},
	}
}

// EffectiveGenerationConfig fills empty bindings with xAI Imagine defaults.
func (c *Config) EffectiveGenerationConfig() GenerationConfig {
	if c == nil {
		return defaultGenerationConfig()
	}
	out := c.Generation
	if strings.TrimSpace(out.Image.ProviderID) == "" {
		out.Image.ProviderID = generation.DefaultProviderID
	}
	if strings.TrimSpace(out.Image.Model) == "" {
		out.Image.Model = generation.DefaultImageModel
	}
	if strings.TrimSpace(out.Video.ProviderID) == "" {
		out.Video.ProviderID = generation.DefaultProviderID
	}
	if strings.TrimSpace(out.Video.Model) == "" {
		out.Video.Model = generation.DefaultVideoModel
	}
	return out
}

// GenerationBinding resolves the Settings pair plus the provider method.
func (c *Config) GenerationBinding(kind string) generation.Binding {
	cfg := c.EffectiveGenerationConfig()
	var selected GenerationModelConfig
	switch kind {
	case generation.KindVideo:
		selected = cfg.Video
	default:
		selected = cfg.Image
	}
	method := ""
	if entry := c.FindProvider(selected.ProviderID); entry != nil {
		method = entry.Method
	}
	if method == "" {
		method = selected.ProviderID
	}
	return generation.Binding{
		ProviderID: selected.ProviderID,
		Model:      selected.Model,
		Method:     method,
	}
}
