package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/cometline/cometmind/internal/logging"
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
	Command string   `json:"command"`
	Args    []string `json:"args"`
	Timeout string   `json:"timeout"`
}

type cometlineSkillsJSON struct {
	Enabled             bool     `json:"enabled"`
	Roots               []string `json:"roots"`
	IncludeOpenCode     bool     `json:"includeOpenCode"`
	IncludeClaude       bool     `json:"includeClaude"`
	MirrorToCometMind   bool     `json:"mirrorToCometMind"`
	SynthesisEnabled    bool     `json:"synthesisEnabled"`
	SynthesisProviderID string   `json:"synthesisProviderId"`
	SynthesisModel      string   `json:"synthesisModel"`
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
	CleanupIntervalMinutes  int  `json:"cleanupIntervalMinutes"`
	RetentionDays           int  `json:"retentionDays"`
	MaxSessionsPerWorkspace int  `json:"maxSessionsPerWorkspace"`
	ArchivedMemoryPurgeDays int  `json:"archivedMemoryPurgeDays"`
	DeletedJobPurgeDays     int  `json:"deletedJobPurgeDays"`
	VacuumAfterPurge        bool `json:"vacuumAfterPurge"`
}

type cometlineMCPOAuthJSON struct {
	ClientID         string   `json:"clientId"`
	Scopes           []string `json:"scopes"`
	AuthorizationURL string   `json:"authorizationUrl"`
	TokenURL         string   `json:"tokenUrl"`
}

type cometlineMCPServerJSON struct {
	ID           string                 `json:"id"`
	Name         string                 `json:"name"`
	Enabled      bool                   `json:"enabled"`
	Transport    string                 `json:"transport"`
	Command      string                 `json:"command"`
	Args         []string               `json:"args"`
	Env          map[string]string      `json:"env"`
	URL          string                 `json:"url"`
	Headers      map[string]string      `json:"headers"`
	OAuth        *cometlineMCPOAuthJSON `json:"oauth"`
	AllowedTools []string               `json:"allowedTools"`
}

type cometlineMCPJSON struct {
	Enabled bool                     `json:"enabled"`
	Servers []cometlineMCPServerJSON `json:"servers"`
}

type cometlineJobsNotificationsJSON struct {
	Enabled     bool `json:"enabled"`
	OnClaimed   bool `json:"onClaimed"`
	OnCompleted bool `json:"onCompleted"`
	OnReleased  bool `json:"onReleased"`
	OnBlocked   bool `json:"onBlocked"`
}

type cometlineJobsJSON struct {
	Notifications            cometlineJobsNotificationsJSON `json:"notifications"`
	LeaseMinutes             int                            `json:"leaseMinutes"`
	DeletedPurgeDays         int                            `json:"deletedPurgeDays"`
	ArchivedPurgeDays        int                            `json:"archivedPurgeDays"`
	StaleReviewMinutes       int                            `json:"staleReviewMinutes"`
	MaxConsecutiveFailures   int                            `json:"maxConsecutiveFailures"`
	RetryCooldownMinutes     int                            `json:"retryCooldownMinutes"`
	MaxRetryCooldownMinutes  int                            `json:"maxRetryCooldownMinutes"`
	ReconcileIntervalSeconds int                            `json:"reconcileIntervalSeconds"`
}

type cometlineAutonomyJSON struct {
	Enabled             bool `json:"enabled"`
	MaxConcurrent       int  `json:"maxConcurrent"`
	PollIntervalSeconds int  `json:"pollIntervalSeconds"`
	MaxStepsPerRun      int  `json:"maxStepsPerRun"`
}

type cometlineSchedulerJSON struct {
	Enabled             bool `json:"enabled"`
	PollIntervalSeconds int  `json:"pollIntervalSeconds"`
}

type cometlinePlanningJSON struct {
	Enabled bool `json:"enabled"`
}

type cometlineCometmindJSON struct {
	SystemPromptPath   string              `json:"systemPromptPath"`
	MaxTokens          int                 `json:"maxTokens"`
	ContextWindowLimit int                 `json:"contextWindowLimit"`
	TitleProviderID    string              `json:"titleProviderId"`
	TitleModelID       string              `json:"titleModelId"`
	ACP                cometlineACPJSON    `json:"acp"`
	Skills             cometlineSkillsJSON `json:"skills"`
	Memory             struct {
		ExtractionProviderID string                       `json:"extractionProviderId"`
		ExtractionModel      string                       `json:"extractionModel"`
		Embedding            cometlineMemoryEmbeddingJSON `json:"embedding"`
	} `json:"memory"`
	Storage cometlineStorageJSON `json:"storage"`
	Gateway struct {
		Discord cometlineDiscordJSON `json:"discord"`
	} `json:"gateway"`
	MCP       cometlineMCPJSON       `json:"mcp"`
	Jobs      cometlineJobsJSON      `json:"jobs"`
	Autonomy  cometlineAutonomyJSON  `json:"autonomy"`
	Scheduler cometlineSchedulerJSON `json:"scheduler"`
	Planning  cometlinePlanningJSON  `json:"planning"`
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

	// When no provider is enabled with models, boot anyway with an empty
	// provider configuration. The sidecar stays healthy and the UI remains
	// usable; sending a message returns a clear "no provider configured"
	// error from the provider factory instead of a TCP connection refused.
	noProviders := len(runtimeProviders) == 0
	if noProviders {
		logging.L().Info("config.no_providers_configured")
	}

	var active cometlineProviderJSON
	var providers []ProviderEntry
	if !noProviders {
		activeID := strings.TrimSpace(raw.ActiveProviderID)
		activeIdx := 0
		for i := range runtimeProviders {
			if runtimeProviders[i].ID == activeID {
				activeIdx = i
				break
			}
		}
		active = runtimeProviders[activeIdx]
		providers = make([]ProviderEntry, 0, len(runtimeProviders))
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
	}

	cm := raw.Cometmind
	memDef := defaultMemoryConfig()
	cfg := &Config{
		Provider:           strings.TrimSpace(active.ID),
		Model:              primaryModel(active),
		BaseURL:            strings.TrimSpace(active.BaseURL),
		TitleProvider:      strings.TrimSpace(cm.TitleProviderID),
		TitleModel:         strings.TrimSpace(cm.TitleModelID),
		MaxTokens:          cm.MaxTokens,
		ContextWindowLimit: normalizeContextWindowLimit(cm.ContextWindowLimit),
		MaxSteps:           50,
		SystemPromptPath:   strings.TrimSpace(cm.SystemPromptPath),
		Providers:          providers,
		ACP: ACPConfig{
			Command: strings.TrimSpace(cm.ACP.Command),
			Args:    append([]string(nil), cm.ACP.Args...),
			Timeout: strings.TrimSpace(cm.ACP.Timeout),
		},
		Skills: SkillsConfig{
			Enabled:             cm.Skills.Enabled,
			Roots:               append([]string(nil), cm.Skills.Roots...),
			IncludeOpenCode:     cm.Skills.IncludeOpenCode,
			IncludeClaude:       cm.Skills.IncludeClaude,
			MirrorToCometMind:   cm.Skills.MirrorToCometMind,
			SynthesisEnabled:    cm.Skills.SynthesisEnabled,
			SynthesisProviderID: strings.TrimSpace(cm.Skills.SynthesisProviderID),
			SynthesisModel:      strings.TrimSpace(cm.Skills.SynthesisModel),
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
			CleanupIntervalMinutes:  cm.Storage.CleanupIntervalMinutes,
			RetentionDays:           cm.Storage.RetentionDays,
			MaxSessionsPerWorkspace: cm.Storage.MaxSessionsPerWorkspace,
			ArchivedMemoryPurgeDays: cm.Storage.ArchivedMemoryPurgeDays,
			DeletedJobPurgeDays:     cm.Storage.DeletedJobPurgeDays,
			VacuumAfterPurge:        cm.Storage.VacuumAfterPurge,
		},
		Jobs: JobsConfig{
			Notifications: JobNotificationSettings{
				Enabled:     cm.Jobs.Notifications.Enabled,
				OnClaimed:   cm.Jobs.Notifications.OnClaimed,
				OnCompleted: cm.Jobs.Notifications.OnCompleted,
				OnReleased:  cm.Jobs.Notifications.OnReleased,
				OnBlocked:   cm.Jobs.Notifications.OnBlocked,
			},
			LeaseMinutes:             cm.Jobs.LeaseMinutes,
			DeletedPurgeDays:         cm.Jobs.DeletedPurgeDays,
			ArchivedPurgeDays:        cm.Jobs.ArchivedPurgeDays,
			StaleReviewMinutes:       cm.Jobs.StaleReviewMinutes,
			MaxConsecutiveFailures:   cm.Jobs.MaxConsecutiveFailures,
			RetryCooldownMinutes:     cm.Jobs.RetryCooldownMinutes,
			MaxRetryCooldownMinutes:  cm.Jobs.MaxRetryCooldownMinutes,
			ReconcileIntervalSeconds: cm.Jobs.ReconcileIntervalSeconds,
		},
		Autonomy: AutonomousJobsConfig{
			Enabled:             cm.Autonomy.Enabled,
			MaxConcurrent:       cm.Autonomy.MaxConcurrent,
			PollIntervalSeconds: cm.Autonomy.PollIntervalSeconds,
			MaxStepsPerRun:      cm.Autonomy.MaxStepsPerRun,
		},
		Scheduler: SchedulerConfig{
			Enabled:             cm.Scheduler.Enabled,
			PollIntervalSeconds: cm.Scheduler.PollIntervalSeconds,
		},
		Planning: PlanningConfig{
			Enabled: cm.Planning.Enabled,
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
		MCP: adaptMCPJSON(cm.MCP),
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
	if cfg.Provider == "" && !noProviders {
		cfg.Provider = def.Provider
	}
	if cfg.Model == "" && !noProviders {
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

func normalizeContextWindowLimit(value int) int {
	if value == 256_000 {
		return 256_000
	}
	return 128_000
}

func adaptMCPJSON(raw cometlineMCPJSON) MCPConfig {
	out := MCPConfig{Enabled: raw.Enabled}
	for _, srv := range raw.Servers {
		entry := MCPServerConfig{
			ID:           strings.TrimSpace(srv.ID),
			Name:         strings.TrimSpace(srv.Name),
			Enabled:      srv.Enabled,
			Transport:    MCPTransport(strings.TrimSpace(srv.Transport)),
			Command:      strings.TrimSpace(srv.Command),
			Args:         append([]string(nil), srv.Args...),
			Env:          copyStringMapGo(srv.Env),
			URL:          strings.TrimSpace(srv.URL),
			Headers:      copyStringMapGo(srv.Headers),
			AllowedTools: append([]string(nil), srv.AllowedTools...),
		}
		if srv.OAuth != nil {
			entry.OAuth = &MCPOAuthConfig{
				ClientID:         strings.TrimSpace(srv.OAuth.ClientID),
				Scopes:           append([]string(nil), srv.OAuth.Scopes...),
				AuthorizationURL: strings.TrimSpace(srv.OAuth.AuthorizationURL),
				TokenURL:         strings.TrimSpace(srv.OAuth.TokenURL),
			}
		}
		if entry.ID != "" {
			out.Servers = append(out.Servers, entry)
		}
	}
	return out
}

func copyStringMapGo(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
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
			SystemPromptPath:   def.SystemPromptPath,
			MaxTokens:          def.MaxTokens,
			ContextWindowLimit: def.ContextWindowLimit,
			ACP: cometlineACPJSON{
				Command: "opencode",
				Args:    []string{"acp"},
				Timeout: "30m",
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
