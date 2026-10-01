package config

import (
	"strings"

	"github.com/Cometline/cometline/cometmind/internal/logging"
)

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

	var providers []ProviderEntry
	if !noProviders {
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

	defaultProviderID, defaultModelID, defaultBaseURL := resolveDefaultLLM(raw, runtimeProviders)

	cm := raw.Cometmind
	memDef := defaultMemoryConfig()
	deletedPurgeDays := DefaultJobSettings().DeletedPurgeDays
	if cm.Jobs.DeletedPurgeDays != nil {
		deletedPurgeDays = *cm.Jobs.DeletedPurgeDays
	}
	cfg := &Config{
		DefaultProviderID: defaultProviderID,
		DefaultModelID:    defaultModelID,
		BaseURL:           defaultBaseURL,
		TitleProvider:     strings.TrimSpace(cm.TitleProviderID),
		TitleModel:        strings.TrimSpace(cm.TitleModelID),
		MaxSteps:          Defaults().MaxSteps,
		SystemPromptPath:  strings.TrimSpace(cm.SystemPromptPath),
		Providers:         providers,
		ACP: ACPConfig{
			// Missing enabled defaults to false (native coding path preferred).
			Enabled:        cm.ACP.Enabled != nil && *cm.ACP.Enabled,
			DefaultHarness: strings.TrimSpace(cm.ACP.DefaultHarness),
		},
		Skills: SkillsConfig{
			Enabled:             cm.Skills.Enabled,
			Roots:               append([]string(nil), cm.Skills.Roots...),
			IncludeOpenCode:     cm.Skills.IncludeOpenCode,
			IncludeClaude:       cm.Skills.IncludeClaude,
			SynthesisEnabled:    cm.Skills.SynthesisEnabled,
			SynthesisProviderID: strings.TrimSpace(cm.Skills.SynthesisProviderID),
			SynthesisModel:      strings.TrimSpace(cm.Skills.SynthesisModel),
		},
		Memory: MemoryConfig{
			Enabled:             cm.Memory.Enabled,
			AutoExtract:         cm.Memory.AutoExtract,
			AutoRetrieve:        cm.Memory.AutoRetrieve,
			MaxRetrieved:        cm.Memory.MaxRetrieved,
			TaskOutcomeLimit:    cm.Memory.TaskOutcomeLimit,
			SimilarityThreshold: cm.Memory.SimilarityThreshold,
			ExtractionProvider:  strings.TrimSpace(cm.Memory.ExtractionProviderID),
			ExtractionModel:     firstNonEmpty(strings.TrimSpace(cm.Memory.ExtractionModel), memDef.ExtractionModel),
			Lifecycle: MemoryLifecycleConfig{
				DecayHalfLifeDays:     cm.Memory.Lifecycle.DecayHalfLifeDays,
				ForgetThreshold:       cm.Memory.Lifecycle.ForgetThreshold,
				UsageBoostFactor:      cm.Memory.Lifecycle.UsageBoostFactor,
				MaxUsageBoost:         cm.Memory.Lifecycle.MaxUsageBoost,
				MaxMemories:           cm.Memory.Lifecycle.MaxMemories,
				CompactionTargetRatio: cm.Memory.Lifecycle.CompactionTargetRatio,
				CompactionOnExtract:   cm.Memory.Lifecycle.CompactionOnExtract,
			},
			Embedding: MemoryEmbeddingConfig{
				ProviderID: strings.TrimSpace(cm.Memory.Embedding.ProviderID),
				Provider:   strings.TrimSpace(cm.Memory.Embedding.Provider),
				Model:      strings.TrimSpace(cm.Memory.Embedding.Model),
				BaseURL:    strings.TrimSpace(cm.Memory.Embedding.BaseURL),
				APIKey:     cm.Memory.Embedding.APIKey,
			},
		},
		Storage: adaptStorageConfig(cm.Storage),
		Jobs: JobsConfig{
			Notifications: JobNotificationSettings{
				Enabled:     cm.Jobs.Notifications.Enabled,
				OnClaimed:   cm.Jobs.Notifications.OnClaimed,
				OnCompleted: cm.Jobs.Notifications.OnCompleted,
				OnReleased:  cm.Jobs.Notifications.OnReleased,
				OnBlocked:   cm.Jobs.Notifications.OnBlocked,
			},
			LeaseMinutes:             cm.Jobs.LeaseMinutes,
			DeletedPurgeDays:         deletedPurgeDays,
			DoneArchiveDays:          cm.Jobs.DoneArchiveDays,
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
			ProviderID:          strings.TrimSpace(cm.Autonomy.ProviderID),
			ModelID:             strings.TrimSpace(cm.Autonomy.ModelID),
		},
		Scheduler: SchedulerConfig{
			Enabled:             cm.Scheduler.Enabled,
			PollIntervalSeconds: cm.Scheduler.PollIntervalSeconds,
		},
		Generation: GenerationConfig{
			Image: GenerationModelConfig{
				ProviderID: strings.TrimSpace(cm.Generation.Image.ProviderID),
				Model:      strings.TrimSpace(cm.Generation.Image.Model),
			},
			Video: GenerationModelConfig{
				ProviderID: strings.TrimSpace(cm.Generation.Video.ProviderID),
				Model:      strings.TrimSpace(cm.Generation.Video.Model),
			},
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

	if cfg.ACP.DefaultHarness == "" {
		cfg.ACP.DefaultHarness = "opencode"
	}
	if cfg.Gateway.Discord.BotTokenEnv == "" {
		cfg.Gateway.Discord.BotTokenEnv = "DISCORD_BOT_TOKEN"
	}
	if cfg.DefaultProviderID == "" && !noProviders {
		cfg.DefaultProviderID = def.DefaultProviderID
	}
	if cfg.DefaultModelID == "" && !noProviders {
		cfg.DefaultModelID = def.DefaultModelID
	}
	if cfg.MaxSteps == 0 {
		cfg.MaxSteps = def.MaxSteps
	}

	return cfg, nil
}

// resolveDefaultLLM picks the Default model pair from defaultProviderId and defaultModelId.
func resolveDefaultLLM(raw cometlineSettingsJSON, runtimeProviders []cometlineProviderJSON) (providerID, modelID, baseURL string) {
	if len(runtimeProviders) == 0 {
		return "", "", ""
	}
	byID := make(map[string]cometlineProviderJSON, len(runtimeProviders))
	for _, p := range runtimeProviders {
		byID[strings.TrimSpace(p.ID)] = p
	}

	defID := strings.TrimSpace(raw.DefaultProviderID)
	defModel := strings.TrimSpace(raw.DefaultModelID)
	if defID != "" {
		if p, ok := byID[defID]; ok {
			if defModel == "" {
				defModel = primaryModel(p)
			}
			return defID, defModel, strings.TrimSpace(p.BaseURL)
		}
	}

	p := runtimeProviders[0]
	return strings.TrimSpace(p.ID), primaryModel(p), strings.TrimSpace(p.BaseURL)
}

func normalizeMCPTransport(raw string) MCPTransport {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case string(MCPTransportHTTP), "sse":
		return MCPTransportHTTP
	default:
		return MCPTransportStdio
	}
}

func adaptMCPJSON(raw cometlineMCPJSON) MCPConfig {
	out := MCPConfig{Enabled: raw.Enabled}
	for _, srv := range raw.Servers {
		entry := MCPServerConfig{
			ID:           strings.TrimSpace(srv.ID),
			Name:         strings.TrimSpace(srv.Name),
			Enabled:      srv.Enabled,
			Transport:    normalizeMCPTransport(srv.Transport),
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

func adaptStorageConfig(cm cometlineStorageJSON) StorageConfig {
	def := defaultStorageConfig()
	s := StorageConfig{
		CleanupIntervalMinutes:  cm.CleanupIntervalMinutes,
		RetentionDays:           cm.RetentionDays,
		MaxSessionsPerWorkspace: cm.MaxSessionsPerWorkspace,
		ArchivedMemoryPurgeDays: cm.ArchivedMemoryPurgeDays,
		VacuumAfterPurge:        cm.VacuumAfterPurge,
	}
	// Omitted keys (pre-upgrade JSON) get defaults when other storage rules
	// are present; explicit 0 still means disable.
	hasOther := s.RetentionDays != 0 ||
		s.CleanupIntervalMinutes != 0 ||
		s.MaxSessionsPerWorkspace != 0 ||
		s.ArchivedMemoryPurgeDays != 0 ||
		s.VacuumAfterPurge
	if cm.ToolOutputRetentionDays != nil {
		s.ToolOutputRetentionDays = *cm.ToolOutputRetentionDays
	} else if hasOther {
		s.ToolOutputRetentionDays = def.ToolOutputRetentionDays
	}
	if cm.DetachedMediaRetentionDays != nil {
		s.DetachedMediaRetentionDays = *cm.DetachedMediaRetentionDays
	} else if hasOther {
		s.DetachedMediaRetentionDays = def.DetachedMediaRetentionDays
	}
	if cm.AgentTmpRetentionDays != nil {
		s.AgentTmpRetentionDays = *cm.AgentTmpRetentionDays
	} else if hasOther {
		s.AgentTmpRetentionDays = def.AgentTmpRetentionDays
	}
	s.Backup = StorageBackupConfig{
		Enabled:        cm.Backup.Enabled,
		DestinationDir: strings.TrimSpace(cm.Backup.DestinationDir),
		IntervalHours:  cm.Backup.IntervalHours,
		MaxBackups:     cm.Backup.MaxBackups,
	}
	if s.Backup.IntervalHours == 0 {
		s.Backup.IntervalHours = def.Backup.IntervalHours
	}
	if s.Backup.MaxBackups == 0 && !s.Backup.Enabled {
		s.Backup.MaxBackups = def.Backup.MaxBackups
	}
	return s
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
