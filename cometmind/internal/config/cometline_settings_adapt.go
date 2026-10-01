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

	defaultProviderID, defaultModelID, defaultBaseURL := resolveDefaultLLM(raw, runtimeProviders)

	cm := raw.Cometmind
	cfg := &Config{
		DefaultProviderID: defaultProviderID,
		DefaultModelID:    defaultModelID,
		BaseURL:           defaultBaseURL,
		TitleProvider:     strings.TrimSpace(cm.TitleProviderID),
		TitleModel:        strings.TrimSpace(cm.TitleModelID),
		MaxSteps:          Defaults().MaxSteps,
		SystemPromptPath:  strings.TrimSpace(cm.SystemPromptPath),
		Providers:         adaptProvidersJSON(runtimeProviders),
		ACP:               adaptACPJSON(cm.ACP),
		Skills:            adaptSkillsJSON(cm.Skills),
		Memory:            adaptMemoryJSON(cm.Memory),
		Storage:           adaptStorageConfig(cm.Storage),
		Jobs:              adaptJobsJSON(cm.Jobs),
		Autonomy:          adaptAutonomyJSON(cm.Autonomy),
		Scheduler: SchedulerConfig{
			Enabled:             cm.Scheduler.Enabled,
			PollIntervalSeconds: cm.Scheduler.PollIntervalSeconds,
		},
		Generation: adaptGenerationJSON(cm.Generation),
		Gateway:    GatewayConfig{Discord: adaptDiscordJSON(cm.Gateway.Discord)},
		MCP:        adaptMCPJSON(cm.MCP),
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

func adaptProvidersJSON(runtimeProviders []cometlineProviderJSON) []ProviderEntry {
	if len(runtimeProviders) == 0 {
		return nil
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
	return providers
}

func adaptACPJSON(raw cometlineACPJSON) ACPConfig {
	out := ACPConfig{
		// Missing enabled defaults to false (native coding path preferred).
		Enabled:        raw.Enabled != nil && *raw.Enabled,
		DefaultHarness: strings.TrimSpace(raw.DefaultHarness),
	}
	if out.DefaultHarness == "" {
		out.DefaultHarness = "opencode"
	}
	return out
}

func adaptSkillsJSON(raw cometlineSkillsJSON) SkillsConfig {
	return SkillsConfig{
		Enabled:             raw.Enabled,
		Roots:               append([]string(nil), raw.Roots...),
		IncludeOpenCode:     raw.IncludeOpenCode,
		IncludeClaude:       raw.IncludeClaude,
		SynthesisEnabled:    raw.SynthesisEnabled,
		SynthesisProviderID: strings.TrimSpace(raw.SynthesisProviderID),
		SynthesisModel:      strings.TrimSpace(raw.SynthesisModel),
	}
}

func adaptMemoryJSON(raw cometlineMemoryJSON) MemoryConfig {
	memDef := defaultMemoryConfig()
	return MemoryConfig{
		Enabled:             raw.Enabled,
		AutoExtract:         raw.AutoExtract,
		AutoRetrieve:        raw.AutoRetrieve,
		MaxRetrieved:        raw.MaxRetrieved,
		TaskOutcomeLimit:    raw.TaskOutcomeLimit,
		SimilarityThreshold: raw.SimilarityThreshold,
		ExtractionProvider:  strings.TrimSpace(raw.ExtractionProviderID),
		ExtractionModel:     firstNonEmpty(strings.TrimSpace(raw.ExtractionModel), memDef.ExtractionModel),
		Lifecycle: MemoryLifecycleConfig{
			DecayHalfLifeDays:     raw.Lifecycle.DecayHalfLifeDays,
			ForgetThreshold:       raw.Lifecycle.ForgetThreshold,
			UsageBoostFactor:      raw.Lifecycle.UsageBoostFactor,
			MaxUsageBoost:         raw.Lifecycle.MaxUsageBoost,
			MaxMemories:           raw.Lifecycle.MaxMemories,
			CompactionTargetRatio: raw.Lifecycle.CompactionTargetRatio,
			CompactionOnExtract:   raw.Lifecycle.CompactionOnExtract,
		},
		Embedding: MemoryEmbeddingConfig{
			ProviderID: strings.TrimSpace(raw.Embedding.ProviderID),
			Provider:   strings.TrimSpace(raw.Embedding.Provider),
			Model:      strings.TrimSpace(raw.Embedding.Model),
			BaseURL:    strings.TrimSpace(raw.Embedding.BaseURL),
			APIKey:     raw.Embedding.APIKey,
		},
	}
}

func adaptJobsJSON(raw cometlineJobsJSON) JobsConfig {
	deletedPurgeDays := DefaultJobSettings().DeletedPurgeDays
	if raw.DeletedPurgeDays != nil {
		deletedPurgeDays = *raw.DeletedPurgeDays
	}
	return JobsConfig{
		Notifications: JobNotificationSettings{
			Enabled:     raw.Notifications.Enabled,
			OnClaimed:   raw.Notifications.OnClaimed,
			OnCompleted: raw.Notifications.OnCompleted,
			OnReleased:  raw.Notifications.OnReleased,
			OnBlocked:   raw.Notifications.OnBlocked,
		},
		LeaseMinutes:             raw.LeaseMinutes,
		DeletedPurgeDays:         deletedPurgeDays,
		DoneArchiveDays:          raw.DoneArchiveDays,
		ArchivedPurgeDays:        raw.ArchivedPurgeDays,
		StaleReviewMinutes:       raw.StaleReviewMinutes,
		MaxConsecutiveFailures:   raw.MaxConsecutiveFailures,
		RetryCooldownMinutes:     raw.RetryCooldownMinutes,
		MaxRetryCooldownMinutes:  raw.MaxRetryCooldownMinutes,
		ReconcileIntervalSeconds: raw.ReconcileIntervalSeconds,
	}
}

func adaptAutonomyJSON(raw cometlineAutonomyJSON) AutonomousJobsConfig {
	return AutonomousJobsConfig{
		Enabled:             raw.Enabled,
		MaxConcurrent:       raw.MaxConcurrent,
		PollIntervalSeconds: raw.PollIntervalSeconds,
		MaxStepsPerRun:      raw.MaxStepsPerRun,
		ProviderID:          strings.TrimSpace(raw.ProviderID),
		ModelID:             strings.TrimSpace(raw.ModelID),
	}
}

func adaptGenerationJSON(raw cometlineGenerationJSON) GenerationConfig {
	return GenerationConfig{
		Image: GenerationModelConfig{
			ProviderID: strings.TrimSpace(raw.Image.ProviderID),
			Model:      strings.TrimSpace(raw.Image.Model),
		},
		Video: GenerationModelConfig{
			ProviderID: strings.TrimSpace(raw.Video.ProviderID),
			Model:      strings.TrimSpace(raw.Video.Model),
		},
	}
}

func adaptDiscordJSON(raw cometlineDiscordJSON) DiscordGatewayConfig {
	out := DiscordGatewayConfig{
		Enabled:         raw.Enabled,
		BotToken:        strings.TrimSpace(raw.BotToken),
		BotTokenEnv:     strings.TrimSpace(raw.BotTokenEnv),
		AllowedUsers:    append([]string(nil), raw.AllowedUsers...),
		AllowedChannels: append([]string(nil), raw.AllowedChannels...),
		RequireMention:  raw.RequireMention,
		WorkspacePath:   strings.TrimSpace(raw.WorkspacePath),
		Provider:        strings.TrimSpace(raw.ProviderID),
		Model:           strings.TrimSpace(raw.ModelID),
	}
	if out.BotTokenEnv == "" {
		out.BotTokenEnv = "DISCORD_BOT_TOKEN"
	}
	return out
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
