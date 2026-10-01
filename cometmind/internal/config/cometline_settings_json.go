package config

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
	Enabled        *bool  `json:"enabled"`
	DefaultHarness string `json:"defaultHarness"`
}

type cometlineSkillsJSON struct {
	Enabled             bool     `json:"enabled"`
	Roots               []string `json:"roots"`
	IncludeOpenCode     bool     `json:"includeOpenCode"`
	IncludeClaude       bool     `json:"includeClaude"`
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

type cometlineMemoryLifecycleJSON struct {
	DecayHalfLifeDays     float64 `json:"decayHalfLifeDays"`
	ForgetThreshold       float64 `json:"forgetThreshold"`
	UsageBoostFactor      float64 `json:"usageBoostFactor"`
	MaxUsageBoost         float64 `json:"maxUsageBoost"`
	MaxMemories           int     `json:"maxMemories"`
	CompactionTargetRatio float64 `json:"compactionTargetRatio"`
	CompactionOnExtract   bool    `json:"compactionOnExtract"`
}

type cometlineMemoryJSON struct {
	Enabled              bool                         `json:"enabled"`
	AutoExtract          bool                         `json:"autoExtract"`
	AutoRetrieve         bool                         `json:"autoRetrieve"`
	MaxRetrieved         int                          `json:"maxRetrieved"`
	TaskOutcomeLimit     int                          `json:"taskOutcomeLimit"`
	SimilarityThreshold  float64                      `json:"similarityThreshold"`
	ExtractionProviderID string                       `json:"extractionProviderId"`
	ExtractionModel      string                       `json:"extractionModel"`
	Lifecycle            cometlineMemoryLifecycleJSON `json:"lifecycle"`
	Embedding            cometlineMemoryEmbeddingJSON `json:"embedding"`
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

type cometlineStorageBackupJSON struct {
	Enabled        bool   `json:"enabled"`
	DestinationDir string `json:"destinationDir"`
	IntervalHours  int    `json:"intervalHours"`
	MaxBackups     int    `json:"maxBackups"`
}

type cometlineStorageJSON struct {
	CleanupIntervalMinutes     int  `json:"cleanupIntervalMinutes"`
	RetentionDays              int  `json:"retentionDays"`
	DetachedMediaRetentionDays *int `json:"detachedMediaRetentionDays"`
	MaxSessionsPerWorkspace    int  `json:"maxSessionsPerWorkspace"`
	ArchivedMemoryPurgeDays    int  `json:"archivedMemoryPurgeDays"`

	VacuumAfterPurge        bool                       `json:"vacuumAfterPurge"`
	ToolOutputRetentionDays *int                       `json:"toolOutputRetentionDays"`
	AgentTmpRetentionDays   *int                       `json:"agentTmpRetentionDays"`
	Backup                  cometlineStorageBackupJSON `json:"backup"`
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
	DeletedPurgeDays         *int                           `json:"deletedPurgeDays"`
	DoneArchiveDays          int                            `json:"doneArchiveDays"`
	ArchivedPurgeDays        int                            `json:"archivedPurgeDays"`
	StaleReviewMinutes       int                            `json:"staleReviewMinutes"`
	MaxConsecutiveFailures   int                            `json:"maxConsecutiveFailures"`
	RetryCooldownMinutes     int                            `json:"retryCooldownMinutes"`
	MaxRetryCooldownMinutes  int                            `json:"maxRetryCooldownMinutes"`
	ReconcileIntervalSeconds int                            `json:"reconcileIntervalSeconds"`
}

type cometlineAutonomyJSON struct {
	Enabled             bool   `json:"enabled"`
	MaxConcurrent       int    `json:"maxConcurrent"`
	PollIntervalSeconds int    `json:"pollIntervalSeconds"`
	MaxStepsPerRun      int    `json:"maxStepsPerRun"`
	ProviderID          string `json:"providerId"`
	ModelID             string `json:"modelId"`
}

type cometlineSchedulerJSON struct {
	Enabled             bool `json:"enabled"`
	PollIntervalSeconds int  `json:"pollIntervalSeconds"`
}

type cometlineGenerationModelJSON struct {
	ProviderID string `json:"providerId"`
	Model      string `json:"model"`
}

type cometlineGenerationJSON struct {
	Image cometlineGenerationModelJSON `json:"image"`
	Video cometlineGenerationModelJSON `json:"video"`
}

type cometlineCometmindJSON struct {
	SystemPromptPath string               `json:"systemPromptPath"`
	TitleProviderID  string               `json:"titleProviderId"`
	TitleModelID     string               `json:"titleModelId"`
	ACP              cometlineACPJSON     `json:"acp"`
	Skills           cometlineSkillsJSON  `json:"skills"`
	Memory           cometlineMemoryJSON  `json:"memory"`
	Storage          cometlineStorageJSON `json:"storage"`
	Gateway          struct {
		Discord cometlineDiscordJSON `json:"discord"`
	} `json:"gateway"`
	MCP        cometlineMCPJSON        `json:"mcp"`
	Jobs       cometlineJobsJSON       `json:"jobs"`
	Autonomy   cometlineAutonomyJSON   `json:"autonomy"`
	Scheduler  cometlineSchedulerJSON  `json:"scheduler"`
	Generation cometlineGenerationJSON `json:"generation"`
}

type cometlineSettingsJSON struct {
	Providers         []cometlineProviderJSON `json:"providers"`
	DefaultProviderID string                  `json:"defaultProviderId"`
	DefaultModelID    string                  `json:"defaultModelId"`
	Cometmind         cometlineCometmindJSON  `json:"cometmind"`
}
