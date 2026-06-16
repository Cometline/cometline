package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cometline/cometmind/internal/paths"
	"github.com/spf13/viper"
)

const (
	ProviderAnthropic    = "anthropic"
	ProviderOpenAI       = "openai"
	ProviderOpenAICompat = "openai-compatible"
	ProviderOpencodeGo   = "opencode-go"
)

// ProviderEntry is one configured LLM provider managed by Cometline.
type ProviderEntry struct {
	ID      string `mapstructure:"id"`
	Name    string `mapstructure:"name"`
	Method  string `mapstructure:"method"`
	BaseURL string `mapstructure:"base_url"`
	APIKey  string `mapstructure:"api_key"`
	Model   string `mapstructure:"model"`
}

// ACPConfig controls external coding agent delegation.
type ACPConfig struct {
	Command     string   `mapstructure:"command"`
	Args        []string `mapstructure:"args"`
	Timeout     string   `mapstructure:"timeout"`
	Interactive bool     `mapstructure:"interactive"`
}

// SkillsConfig controls local Agent Skills discovery.
type SkillsConfig struct {
	Enabled           bool     `mapstructure:"enabled"`
	Roots             []string `mapstructure:"roots"`
	IncludeOpenCode   bool     `mapstructure:"include_opencode"`
	IncludeClaude     bool     `mapstructure:"include_claude"`
	MirrorToCometMind bool     `mapstructure:"mirror_to_cometmind"`
}

// DiscordGatewayConfig configures the Discord messaging adapter.
type DiscordGatewayConfig struct {
	Enabled         bool     `mapstructure:"enabled"`
	BotToken        string   `mapstructure:"bot_token"`
	BotTokenEnv     string   `mapstructure:"bot_token_env"`
	AllowedUsers    []string `mapstructure:"allowed_users"`
	AllowedChannels []string `mapstructure:"allowed_channels"`
	RequireMention  bool     `mapstructure:"require_mention"`
	WorkspacePath   string   `mapstructure:"workspace_path"`
	Provider        string   `mapstructure:"provider"`
	Model           string   `mapstructure:"model"`
}

// GatewayConfig groups messaging gateway settings.
type GatewayConfig struct {
	Discord DiscordGatewayConfig `mapstructure:"discord"`
}

// Config holds user-visible runtime settings loaded from ~/.cometmind/config.toml and environment.
type Config struct {
	Provider         string          `mapstructure:"provider"`
	Model            string          `mapstructure:"model"`
	BaseURL          string          `mapstructure:"base_url"`
	MaxTokens        int             `mapstructure:"max_tokens"`
	MaxSteps         int             `mapstructure:"max_steps"`
	SystemPromptPath string          `mapstructure:"system_prompt_path"`
	Providers        []ProviderEntry `mapstructure:"providers"`
	ACP              ACPConfig       `mapstructure:"acp"`
	Skills           SkillsConfig    `mapstructure:"skills"`
	Memory           MemoryConfig    `mapstructure:"memory"`
	Gateway          GatewayConfig   `mapstructure:"gateway"`
}

// Defaults returns baseline values when the config file is missing keys.
func Defaults() *Config {
	return &Config{
		Provider:  ProviderAnthropic,
		Model:     "claude-sonnet-4-5",
		MaxTokens: 8192,
		MaxSteps:  50,
		ACP:       ACPConfig{Interactive: true},
		Skills:    SkillsConfig{Enabled: true, IncludeOpenCode: true, IncludeClaude: true},
		Memory:    defaultMemoryConfig(),
	}
}

// Load reads config from ~/.cometmind/config.toml (creating the parent dir), merges env, and unmarshals.
func Load() (*Config, error) {
	dataDir, err := paths.DataDir()
	if err != nil {
		return nil, err
	}
	cfgPath := filepath.Join(dataDir, "config.toml")

	v := viper.New()
	v.SetConfigType("toml")
	v.SetConfigFile(cfgPath)

	// Environment
	v.SetEnvPrefix("COMETMIND")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	def := Defaults()
	v.SetDefault("provider", def.Provider)
	v.SetDefault("model", def.Model)
	v.SetDefault("base_url", def.BaseURL)
	v.SetDefault("max_tokens", def.MaxTokens)
	v.SetDefault("max_steps", def.MaxSteps)
	v.SetDefault("system_prompt_path", def.SystemPromptPath)
	v.SetDefault("skills.enabled", def.Skills.Enabled)
	v.SetDefault("skills.roots", def.Skills.Roots)
	v.SetDefault("skills.include_opencode", def.Skills.IncludeOpenCode)
	v.SetDefault("skills.include_claude", def.Skills.IncludeClaude)
	v.SetDefault("skills.mirror_to_cometmind", def.Skills.MirrorToCometMind)
	memDef := defaultMemoryConfig()
	v.SetDefault("memory.enabled", memDef.Enabled)
	v.SetDefault("memory.auto_extract", memDef.AutoExtract)
	v.SetDefault("memory.auto_retrieve", memDef.AutoRetrieve)
	v.SetDefault("memory.max_retrieved", memDef.MaxRetrieved)
	v.SetDefault("memory.similarity_threshold", memDef.SimilarityThreshold)
	v.SetDefault("memory.lifecycle.decay_half_life_days", memDef.Lifecycle.DecayHalfLifeDays)
	v.SetDefault("memory.lifecycle.forget_threshold", memDef.Lifecycle.ForgetThreshold)
	v.SetDefault("memory.lifecycle.usage_boost_factor", memDef.Lifecycle.UsageBoostFactor)
	v.SetDefault("memory.lifecycle.max_usage_boost", memDef.Lifecycle.MaxUsageBoost)
	v.SetDefault("memory.lifecycle.max_memories", memDef.Lifecycle.MaxMemories)
	v.SetDefault("memory.lifecycle.compaction_target_ratio", memDef.Lifecycle.CompactionTargetRatio)
	v.SetDefault("memory.lifecycle.compaction_on_extract", memDef.Lifecycle.CompactionOnExtract)
	v.SetDefault("memory.embedding.provider", memDef.Embedding.Provider)
	v.SetDefault("memory.embedding.model", memDef.Embedding.Model)

	if _, err := os.Stat(cfgPath); errors.Is(err, os.ErrNotExist) {
		if err := writeDefaultFile(cfgPath, def); err != nil {
			return nil, err
		}
	}

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var c Config
	if err := v.Unmarshal(&c); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}
	if !v.IsSet("acp.interactive") {
		c.ACP.Interactive = def.ACP.Interactive
	}
	if !v.IsSet("skills.enabled") {
		c.Skills.Enabled = def.Skills.Enabled
	}
	if !v.IsSet("skills.include_opencode") {
		c.Skills.IncludeOpenCode = def.Skills.IncludeOpenCode
	}
	if !v.IsSet("skills.include_claude") {
		c.Skills.IncludeClaude = def.Skills.IncludeClaude
	}
	if c.Provider == "" {
		c.Provider = def.Provider
	}
	if c.Model == "" {
		c.Model = def.Model
	}
	if c.BaseURL == "" {
		c.BaseURL = def.BaseURL
	}
	if c.MaxTokens == 0 {
		c.MaxTokens = def.MaxTokens
	}
	if c.MaxSteps == 0 {
		c.MaxSteps = def.MaxSteps
	}
	if c.SystemPromptPath == "" {
		c.SystemPromptPath = def.SystemPromptPath
	}

	return &c, nil
}

// FindProvider returns the provider entry matching id, or nil if none exists.
func (c *Config) FindProvider(id string) *ProviderEntry {
	for i := range c.Providers {
		if c.Providers[i].ID == id {
			return &c.Providers[i]
		}
	}
	return nil
}

func writeDefaultFile(path string, def *Config) error {
	content := fmt.Sprintf(`# CometMind — https://github.com/cometline/cometmind
provider = %q
model = %q
base_url = %q
max_tokens = %d
max_steps = %d
system_prompt_path = %q
`, def.Provider, def.Model, def.BaseURL, def.MaxTokens, def.MaxSteps, def.SystemPromptPath)
	return os.WriteFile(path, []byte(content), 0o600)
}
