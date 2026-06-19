package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type cometlineProviderJSON struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	Method        string   `json:"method"`
	Enabled       bool     `json:"enabled"`
	BaseURL       string   `json:"baseURL"`
	APIKey        string   `json:"apiKey"`
	SelectedModel string   `json:"selectedModel"`
	Models        []string `json:"models"`
	EnabledModels []string `json:"enabledModels"`
}

type cometlineACPJSON struct {
	Command     string   `json:"command"`
	Args        []string `json:"args"`
	Timeout     string   `json:"timeout"`
	Interactive bool     `json:"interactive"`
}

type cometlineSkillsJSON struct {
	Enabled           bool     `json:"enabled"`
	Roots             []string `json:"roots"`
	IncludeOpenCode   bool     `json:"includeOpenCode"`
	IncludeClaude     bool     `json:"includeClaude"`
	MirrorToCometMind bool     `json:"mirrorToCometMind"`
}

type cometlineMemoryEmbeddingJSON struct {
	ProviderID string `json:"providerId"`
	Provider   string `json:"provider"`
	Model      string `json:"model"`
	BaseURL    string `json:"baseURL"`
	APIKey     string `json:"apiKey"`
}

type cometlineDiscordJSON struct {
	Enabled         bool     `json:"enabled"`
	BotToken        string   `json:"botToken"`
	BotTokenEnv     string   `json:"botTokenEnv"`
	ProviderID      string   `json:"providerId"`
	ModelID         string   `json:"modelId"`
	AllowedUsers    []string `json:"allowedUsers"`
	AllowedChannels []string `json:"allowedChannels"`
	RequireMention  bool     `json:"requireMention"`
	WorkspacePath   string   `json:"workspacePath"`
}

type cometlineStorageJSON struct {
	RetentionDays           int  `json:"retentionDays"`
	MaxSessionsPerWorkspace int  `json:"maxSessionsPerWorkspace"`
	ArchivedMemoryPurgeDays int  `json:"archivedMemoryPurgeDays"`
	VacuumAfterPurge        bool `json:"vacuumAfterPurge"`
}

type cometlineCometmindJSON struct {
	SystemPromptPath string              `json:"systemPromptPath"`
	MaxTokens        int                 `json:"maxTokens"`
	TitleProviderID  string              `json:"titleProviderId"`
	TitleModelID     string              `json:"titleModelId"`
	ACP              cometlineACPJSON    `json:"acp"`
	Skills           cometlineSkillsJSON `json:"skills"`
	Memory           struct {
		ExtractionProviderID string                       `json:"extractionProviderId"`
		ExtractionModel      string                       `json:"extractionModel"`
		Embedding            cometlineMemoryEmbeddingJSON `json:"embedding"`
	} `json:"memory"`
	Storage cometlineStorageJSON `json:"storage"`
	Gateway struct {
		Discord cometlineDiscordJSON `json:"discord"`
	} `json:"gateway"`
}

type cometlineSettingsJSON struct {
	Providers        []cometlineProviderJSON `json:"providers"`
	ActiveProviderID string                  `json:"activeProviderId"`
	Cometmind        cometlineCometmindJSON  `json:"cometmind"`
}

func primaryModel(provider cometlineProviderJSON) string {
	if len(provider.EnabledModels) > 0 {
		return strings.TrimSpace(provider.EnabledModels[0])
	}
	if strings.TrimSpace(provider.SelectedModel) != "" {
		return strings.TrimSpace(provider.SelectedModel)
	}
	if len(provider.Models) > 0 {
		return strings.TrimSpace(provider.Models[0])
	}
	return ""
}

func runtimeProvidersFromJSON(providers []cometlineProviderJSON) []cometlineProviderJSON {
	out := make([]cometlineProviderJSON, 0, len(providers))
	for _, provider := range providers {
		if !provider.Enabled {
			continue
		}
		if len(provider.EnabledModels) == 0 {
			continue
		}
		out = append(out, provider)
	}
	return out
}

func adaptCometlineSettings(raw cometlineSettingsJSON) (*Config, error) {
	def := Defaults()
	runtimeProviders := runtimeProvidersFromJSON(raw.Providers)
	if len(runtimeProviders) == 0 {
		return nil, fmt.Errorf("no enabled providers with models configured")
	}

	activeID := strings.TrimSpace(raw.ActiveProviderID)
	var active *cometlineProviderJSON
	for i := range runtimeProviders {
		if runtimeProviders[i].ID == activeID {
			active = &runtimeProviders[i]
			break
		}
	}
	if active == nil {
		active = &runtimeProviders[0]
	}

	providers := make([]ProviderEntry, 0, len(runtimeProviders))
	for _, provider := range runtimeProviders {
		providers = append(providers, ProviderEntry{
			ID:      strings.TrimSpace(provider.ID),
			Name:    strings.TrimSpace(provider.Name),
			Method:  strings.TrimSpace(provider.Method),
			BaseURL: strings.TrimSpace(provider.BaseURL),
			APIKey:  provider.APIKey,
			Model:   primaryModel(provider),
		})
	}

	cm := raw.Cometmind
	memDef := defaultMemoryConfig()
	cfg := &Config{
		Provider:         strings.TrimSpace(active.ID),
		Model:            primaryModel(*active),
		BaseURL:          strings.TrimSpace(active.BaseURL),
		TitleProvider:    strings.TrimSpace(cm.TitleProviderID),
		TitleModel:       strings.TrimSpace(cm.TitleModelID),
		MaxTokens:        cm.MaxTokens,
		MaxSteps:         50,
		SystemPromptPath: strings.TrimSpace(cm.SystemPromptPath),
		Providers:        providers,
		ACP: ACPConfig{
			Command:     strings.TrimSpace(cm.ACP.Command),
			Args:        append([]string(nil), cm.ACP.Args...),
			Timeout:     strings.TrimSpace(cm.ACP.Timeout),
			Interactive: cm.ACP.Interactive,
		},
		Skills: SkillsConfig{
			Enabled:           cm.Skills.Enabled,
			Roots:             append([]string(nil), cm.Skills.Roots...),
			IncludeOpenCode:   cm.Skills.IncludeOpenCode,
			IncludeClaude:     cm.Skills.IncludeClaude,
			MirrorToCometMind: cm.Skills.MirrorToCometMind,
		},
		Memory: MemoryConfig{
			Enabled:             memDef.Enabled,
			AutoExtract:         memDef.AutoExtract,
			AutoRetrieve:        memDef.AutoRetrieve,
			MaxRetrieved:        memDef.MaxRetrieved,
			SimilarityThreshold: memDef.SimilarityThreshold,
			ExtractionProvider:  strings.TrimSpace(cm.Memory.ExtractionProviderID),
			ExtractionModel:     firstNonEmpty(strings.TrimSpace(cm.Memory.ExtractionModel), memDef.ExtractionModel),
			Lifecycle:           memDef.Lifecycle,
			Embedding: MemoryEmbeddingConfig{
				ProviderID: strings.TrimSpace(cm.Memory.Embedding.ProviderID),
				Provider:   strings.TrimSpace(cm.Memory.Embedding.Provider),
				Model:      strings.TrimSpace(cm.Memory.Embedding.Model),
				BaseURL:    strings.TrimSpace(cm.Memory.Embedding.BaseURL),
				APIKey:     cm.Memory.Embedding.APIKey,
			},
		},
		Storage: StorageConfig{
			RetentionDays:           cm.Storage.RetentionDays,
			MaxSessionsPerWorkspace: cm.Storage.MaxSessionsPerWorkspace,
			ArchivedMemoryPurgeDays: cm.Storage.ArchivedMemoryPurgeDays,
			VacuumAfterPurge:        cm.Storage.VacuumAfterPurge,
		},
		Gateway: GatewayConfig{
			Discord: DiscordGatewayConfig{
				Enabled:         cm.Gateway.Discord.Enabled,
				BotToken:        strings.TrimSpace(cm.Gateway.Discord.BotToken),
				BotTokenEnv:     strings.TrimSpace(cm.Gateway.Discord.BotTokenEnv),
				AllowedUsers:    append([]string(nil), cm.Gateway.Discord.AllowedUsers...),
				AllowedChannels: append([]string(nil), cm.Gateway.Discord.AllowedChannels...),
				RequireMention:  cm.Gateway.Discord.RequireMention,
				WorkspacePath:   strings.TrimSpace(cm.Gateway.Discord.WorkspacePath),
				Provider:        strings.TrimSpace(cm.Gateway.Discord.ProviderID),
				Model:           strings.TrimSpace(cm.Gateway.Discord.ModelID),
			},
		},
	}

	if cfg.ACP.Command == "" {
		cfg.ACP.Command = "opencode"
	}
	if len(cfg.ACP.Args) == 0 {
		cfg.ACP.Args = []string{"acp"}
	}
	if cfg.ACP.Timeout == "" {
		cfg.ACP.Timeout = "30m"
	}
	if cfg.Gateway.Discord.BotTokenEnv == "" {
		cfg.Gateway.Discord.BotTokenEnv = "DISCORD_BOT_TOKEN"
	}
	if cfg.Provider == "" {
		cfg.Provider = def.Provider
	}
	if cfg.Model == "" {
		cfg.Model = def.Model
	}
	if cfg.MaxTokens == 0 {
		cfg.MaxTokens = def.MaxTokens
	}
	if cfg.MaxSteps == 0 {
		cfg.MaxSteps = def.MaxSteps
	}

	return cfg, nil
}

func loadCometlineSettingsJSON(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var raw cometlineSettingsJSON
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse cometline settings: %w", err)
	}
	return adaptCometlineSettings(raw)
}

// ValidateCometlineSettingsJSON checks whether data can be used as Cometline's
// saved settings file without applying environment overrides.
func ValidateCometlineSettingsJSON(data []byte) error {
	var raw cometlineSettingsJSON
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("parse cometline settings: %w", err)
	}
	_, err := adaptCometlineSettings(raw)
	return err
}

func writeMinimalCometlineSettingsJSON(path string, def *Config) error {
	raw := cometlineSettingsJSON{
		Providers: []cometlineProviderJSON{
			{
				ID:            def.Provider,
				Name:          def.Provider,
				Method:        def.Provider,
				Enabled:       true,
				BaseURL:       def.BaseURL,
				EnabledModels: []string{def.Model},
				Models:        []string{def.Model},
				SelectedModel: def.Model,
			},
		},
		ActiveProviderID: def.Provider,
		Cometmind: cometlineCometmindJSON{
			SystemPromptPath: def.SystemPromptPath,
			MaxTokens:        def.MaxTokens,
			ACP: cometlineACPJSON{
				Command:     "opencode",
				Args:        []string{"acp"},
				Timeout:     "30m",
				Interactive: true,
			},
			Skills: cometlineSkillsJSON{
				Enabled:         true,
				IncludeOpenCode: true,
				IncludeClaude:   true,
			},
		},
	}
	data, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}
