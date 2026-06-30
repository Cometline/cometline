package config

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/cometline/cometmind/internal/logging"
	"github.com/cometline/cometmind/internal/paths"
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
	Command string   `mapstructure:"command"`
	Args    []string `mapstructure:"args"`
	Timeout string   `mapstructure:"timeout"`
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
	Provider           string           `mapstructure:"provider"`
	Model              string           `mapstructure:"model"`
	BaseURL            string           `mapstructure:"base_url"`
	TitleProvider      string           `mapstructure:"title_provider"`
	TitleModel         string           `mapstructure:"title_model"`
	MaxTokens          int              `mapstructure:"max_tokens"`
	ContextWindowLimit int              `mapstructure:"context_window_limit"`
	MaxSteps           int              `mapstructure:"max_steps"`
	SystemPromptPath   string           `mapstructure:"system_prompt_path"`
	Providers          []ProviderEntry  `mapstructure:"providers"`
	ACP                ACPConfig        `mapstructure:"acp"`
	Skills             SkillsConfig     `mapstructure:"skills"`
	Memory             MemoryConfig     `mapstructure:"memory"`
	Storage            StorageConfig    `mapstructure:"storage"`
	Subagent           SubagentSettings `mapstructure:"subagent"`
	Gateway            GatewayConfig    `mapstructure:"gateway"`
	MCP                MCPConfig        `mapstructure:"mcp"`
	Jobs               JobsConfig       `mapstructure:"jobs"`
}

// Defaults returns baseline values when the config file is missing keys.
func Defaults() *Config {
	return &Config{
		Provider:           ProviderAnthropic,
		Model:              "claude-sonnet-4-5",
		MaxTokens:          2048,
		ContextWindowLimit: 128_000,
		MaxSteps:           50,
		Skills:             SkillsConfig{Enabled: true, IncludeOpenCode: true, IncludeClaude: true},
		Memory:             defaultMemoryConfig(),
		Storage:            defaultStorageConfig(),
		Jobs:               defaultJobsConfig(),
	}
}

// Load reads ~/.cometmind/cometline-settings.json (with legacy config.toml migration), merges env, and unmarshals.
func Load() (*Config, error) {
	if _, err := paths.DataDir(); err != nil {
		return nil, err
	}
	settingsPath, err := paths.SettingsPath()
	if err != nil {
		return nil, err
	}
	legacyTomlPath, err := paths.LegacyConfigPath()
	if err != nil {
		return nil, err
	}

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
		logging.L().Info("config.legacy_toml_loaded",
			"legacy_path", legacyTomlPath,
			"settings_path", settingsPath,
		)
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
	cfg.Subagent = cfg.EffectiveSubagentSettings()
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
	if v.IsSet("storage_cleanup_interval_minutes") {
		c.Storage.CleanupIntervalMinutes = v.GetInt("storage_cleanup_interval_minutes")
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
	if v.IsSet("subagent_general_max_steps") {
		c.Subagent.GeneralMaxSteps = v.GetInt("subagent_general_max_steps")
	}
	if v.IsSet("subagent_max_concurrent_per_parent") {
		c.Subagent.MaxConcurrentPerParent = v.GetInt("subagent_max_concurrent_per_parent")
	}
	if v.IsSet("storage_subagent_retention_days") {
		c.Storage.SubagentRetentionDays = v.GetInt("storage_subagent_retention_days")
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
