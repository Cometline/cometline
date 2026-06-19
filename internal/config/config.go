package config

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

const (
	ProviderAnthropic    = "anthropic"
	ProviderOpenAI       = "openai"
	ProviderOpenAICompat = "openai-compatible"
	ProviderOpencodeGo   = "opencode-go"
	ProviderCodex        = "codex"
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

// Config holds user-visible runtime settings loaded from ~/.cometmind/cometline-settings.json and environment.
type Config struct {
	Provider         string          `mapstructure:"provider"`
	Model            string          `mapstructure:"model"`
	BaseURL          string          `mapstructure:"base_url"`
	TitleProvider    string          `mapstructure:"title_provider"`
	TitleModel       string          `mapstructure:"title_model"`
	MaxTokens        int             `mapstructure:"max_tokens"`
	MaxSteps         int             `mapstructure:"max_steps"`
	SystemPromptPath string          `mapstructure:"system_prompt_path"`
	Providers        []ProviderEntry `mapstructure:"providers"`
	ACP              ACPConfig       `mapstructure:"acp"`
	Skills           SkillsConfig    `mapstructure:"skills"`
	Memory           MemoryConfig    `mapstructure:"memory"`
	Storage          StorageConfig   `mapstructure:"storage"`
	Gateway          GatewayConfig   `mapstructure:"gateway"`
}

// Defaults returns baseline values when the config file is missing keys.
func Defaults() *Config {
	return &Config{
		Provider:  ProviderAnthropic,
		Model:     "claude-sonnet-4-5",
		MaxTokens: 2048,
		MaxSteps:  50,
		ACP:       ACPConfig{Interactive: true},
		Skills:    SkillsConfig{Enabled: true, IncludeOpenCode: true, IncludeClaude: true},
		Memory:    defaultMemoryConfig(),
		Storage:   defaultStorageConfig(),
	}
}

// Load reads ~/.cometmind/cometline-settings.json (with legacy config.toml migration), merges env, and unmarshals.
func Load() (*Config, error) {
	dataDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	dataDir = filepath.Join(dataDir, ".cometmind")
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return nil, err
	}
	settingsPath := filepath.Join(dataDir, "cometline-settings.json")
	legacyTomlPath := filepath.Join(dataDir, "config.toml")

	def := Defaults()

	var cfg *Config
	switch {
	case fileExists(settingsPath):
		cfg, err = loadCometlineSettingsJSON(settingsPath)
		if err != nil {
			return nil, err
		}
	case fileExists(legacyTomlPath):
		cfg, err = loadLegacyTomlConfig(legacyTomlPath, def)
		if err != nil {
			return nil, err
		}
		log.Printf("cometmind: loaded legacy %s; migrate to %s via Cometline Settings", legacyTomlPath, settingsPath)
	default:
		if err := writeMinimalCometlineSettingsJSON(settingsPath, def); err != nil {
			return nil, err
		}
		cfg, err = loadCometlineSettingsJSON(settingsPath)
		if err != nil {
			return nil, err
		}
	}

	applyEnvOverrides(cfg, def)
	cfg.Memory = cfg.EffectiveMemoryConfig()
	cfg.Storage = cfg.EffectiveStorageConfig()
	return cfg, nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func loadLegacyTomlConfig(cfgPath string, def *Config) (*Config, error) {
	v := viper.New()
	v.SetConfigType("toml")
	v.SetConfigFile(cfgPath)
	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("read legacy config: %w", err)
	}
	var c Config
	if err := v.Unmarshal(&c); err != nil {
		return nil, fmt.Errorf("unmarshal legacy config: %w", err)
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

func applyEnvOverrides(c *Config, def *Config) {
	v := viper.New()
	v.SetEnvPrefix("COMETMIND")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	if provider := strings.TrimSpace(v.GetString("provider")); provider != "" {
		c.Provider = provider
	}
	if model := strings.TrimSpace(v.GetString("model")); model != "" {
		c.Model = model
	}
	if baseURL := strings.TrimSpace(v.GetString("base_url")); baseURL != "" {
		c.BaseURL = baseURL
	}
	if titleProvider := strings.TrimSpace(v.GetString("title_provider")); titleProvider != "" {
		c.TitleProvider = titleProvider
	}
	if titleModel := strings.TrimSpace(v.GetString("title_model")); titleModel != "" {
		c.TitleModel = titleModel
	}
	if prompt := strings.TrimSpace(v.GetString("system_prompt_path")); prompt != "" {
		c.SystemPromptPath = prompt
	}
	if v.IsSet("max_tokens") {
		c.MaxTokens = v.GetInt("max_tokens")
	}
	if v.IsSet("max_steps") {
		c.MaxSteps = v.GetInt("max_steps")
	}
	if c.MaxTokens == 0 {
		c.MaxTokens = def.MaxTokens
	}
	if c.MaxSteps == 0 {
		c.MaxSteps = def.MaxSteps
	}
	if v.IsSet("storage_retention_days") {
		c.Storage.RetentionDays = v.GetInt("storage_retention_days")
	}
	if v.IsSet("storage_max_sessions_per_workspace") {
		c.Storage.MaxSessionsPerWorkspace = v.GetInt("storage_max_sessions_per_workspace")
	}
	if v.IsSet("storage_archived_memory_purge_days") {
		c.Storage.ArchivedMemoryPurgeDays = v.GetInt("storage_archived_memory_purge_days")
	}
	if v.IsSet("storage_vacuum_after_purge") {
		c.Storage.VacuumAfterPurge = v.GetBool("storage_vacuum_after_purge")
	}
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
	_ = path
	_ = def
	return errors.New("writeDefaultFile is deprecated; use cometline-settings.json")
}
