package memory

// Settings mirrors [config.MemoryConfig] at runtime.
type Settings struct {
	Enabled             bool
	AutoExtract         bool
	AutoRetrieve        bool
	MaxRetrieved        int
	SimilarityThreshold float64
	ExtractionProvider string
	ExtractionModel     string
	Lifecycle           LifecycleSettings
	Embedding           EmbeddingSettings
}

// LifecycleSettings controls decay, forget, and compaction.
type LifecycleSettings struct {
	DecayHalfLifeDays     float64
	ForgetThreshold       float64
	UsageBoostFactor      float64
	MaxUsageBoost         float64
	MaxMemories           int
	CompactionTargetRatio float64
	CompactionOnExtract   bool
}

// EmbeddingSettings selects the embedding backend.
type EmbeddingSettings struct {
	Provider string
	Model    string
	BaseURL  string
	APIKey   string
}

// DefaultSettings returns plan defaults.
func DefaultSettings() Settings {
	return Settings{
		Enabled:             true,
		AutoExtract:         true,
		AutoRetrieve:        true,
		MaxRetrieved:        5,
		SimilarityThreshold: 0.5,
		Lifecycle: LifecycleSettings{
			DecayHalfLifeDays:     30,
			ForgetThreshold:       0.1,
			UsageBoostFactor:      0.15,
			MaxUsageBoost:         2.0,
			MaxMemories:           500,
			CompactionTargetRatio: 0.8,
			CompactionOnExtract:   true,
		},
		Embedding: EmbeddingSettings{
			Provider: "openai",
			Model:    "text-embedding-3-small",
		},
	}
}
