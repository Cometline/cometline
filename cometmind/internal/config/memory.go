package config

import (
	"os"
	"strings"

	"github.com/cometline/cometmind/internal/memory"
)

// MemoryConfig controls global memory behavior.
type MemoryConfig struct {
	Enabled             bool                  `json:"enabled" mapstructure:"enabled"`
	AutoExtract         bool                  `json:"auto_extract" mapstructure:"auto_extract"`
	AutoRetrieve        bool                  `json:"auto_retrieve" mapstructure:"auto_retrieve"`
	MaxRetrieved        int                   `json:"max_retrieved" mapstructure:"max_retrieved"`
	SimilarityThreshold float64               `json:"similarity_threshold" mapstructure:"similarity_threshold"`
	ExtractionProvider  string                `json:"extraction_provider" mapstructure:"extraction_provider"`
	ExtractionModel     string                `json:"extraction_model" mapstructure:"extraction_model"`
	Lifecycle           MemoryLifecycleConfig `json:"lifecycle" mapstructure:"lifecycle"`
	Embedding           MemoryEmbeddingConfig `json:"embedding" mapstructure:"embedding"`
}

// MemoryLifecycleConfig controls decay, forget, and compaction.
type MemoryLifecycleConfig struct {
	DecayHalfLifeDays     float64 `json:"decay_half_life_days" mapstructure:"decay_half_life_days"`
	ForgetThreshold       float64 `json:"forget_threshold" mapstructure:"forget_threshold"`
	UsageBoostFactor      float64 `json:"usage_boost_factor" mapstructure:"usage_boost_factor"`
	MaxUsageBoost         float64 `json:"max_usage_boost" mapstructure:"max_usage_boost"`
	MaxMemories           int     `json:"max_memories" mapstructure:"max_memories"`
	CompactionTargetRatio float64 `json:"compaction_target_ratio" mapstructure:"compaction_target_ratio"`
	CompactionOnExtract   bool    `json:"compaction_on_extract" mapstructure:"compaction_on_extract"`
}

// MemoryEmbeddingConfig selects the embedding backend.
type MemoryEmbeddingConfig struct {
	ProviderID string `json:"provider_id,omitempty" mapstructure:"provider_id"`
	Provider   string `json:"provider" mapstructure:"provider"`
	Model      string `json:"model" mapstructure:"model"`
	BaseURL    string `json:"base_url" mapstructure:"base_url"`
	APIKey     string `json:"api_key,omitempty" mapstructure:"api_key"`
}

func defaultMemoryConfig() MemoryConfig {
	def := memory.DefaultSettings()
	return MemoryConfig{
		Enabled:             def.Enabled,
		AutoExtract:         def.AutoExtract,
		AutoRetrieve:        def.AutoRetrieve,
		MaxRetrieved:        def.MaxRetrieved,
		SimilarityThreshold: def.SimilarityThreshold,
		Lifecycle: MemoryLifecycleConfig{
			DecayHalfLifeDays:     def.Lifecycle.DecayHalfLifeDays,
			ForgetThreshold:       def.Lifecycle.ForgetThreshold,
			UsageBoostFactor:      def.Lifecycle.UsageBoostFactor,
			MaxUsageBoost:         def.Lifecycle.MaxUsageBoost,
			MaxMemories:           def.Lifecycle.MaxMemories,
			CompactionTargetRatio: def.Lifecycle.CompactionTargetRatio,
			CompactionOnExtract:   def.Lifecycle.CompactionOnExtract,
		},
		Embedding: MemoryEmbeddingConfig{
			Provider: def.Embedding.Provider,
			Model:    def.Embedding.Model,
		},
	}
}

// MemorySettings converts config into runtime memory settings.
func (c *Config) MemorySettings() memory.Settings {
	mc := c.Memory
	def := defaultMemoryConfig()
	if !c.memoryBehaviorConfigured() {
		embedding := mc.Embedding
		mc = def
		mc.Embedding = embedding
	}
	s := memory.Settings{
		Enabled:             mc.Enabled,
		AutoExtract:         mc.AutoExtract,
		AutoRetrieve:        mc.AutoRetrieve,
		MaxRetrieved:        mc.MaxRetrieved,
		SimilarityThreshold: mc.SimilarityThreshold,
		ExtractionProvider:  mc.ExtractionProvider,
		ExtractionModel:     mc.ExtractionModel,
		Lifecycle: memory.LifecycleSettings{
			DecayHalfLifeDays:     mc.Lifecycle.DecayHalfLifeDays,
			ForgetThreshold:       mc.Lifecycle.ForgetThreshold,
			UsageBoostFactor:      mc.Lifecycle.UsageBoostFactor,
			MaxUsageBoost:         mc.Lifecycle.MaxUsageBoost,
			MaxMemories:           mc.Lifecycle.MaxMemories,
			CompactionTargetRatio: mc.Lifecycle.CompactionTargetRatio,
			CompactionOnExtract:   mc.Lifecycle.CompactionOnExtract,
		},
		Embedding: resolveMemoryEmbedding(c, mc.Embedding),
	}
	if s.MaxRetrieved <= 0 {
		s.MaxRetrieved = def.MaxRetrieved
	}
	if s.SimilarityThreshold <= 0 {
		s.SimilarityThreshold = def.SimilarityThreshold
	}
	if s.Lifecycle.DecayHalfLifeDays <= 0 {
		s.Lifecycle.DecayHalfLifeDays = def.Lifecycle.DecayHalfLifeDays
	}
	if s.Lifecycle.ForgetThreshold <= 0 {
		s.Lifecycle.ForgetThreshold = def.Lifecycle.ForgetThreshold
	}
	if s.Lifecycle.MaxMemories <= 0 {
		s.Lifecycle.MaxMemories = def.Lifecycle.MaxMemories
	}
	if s.Lifecycle.CompactionTargetRatio <= 0 {
		s.Lifecycle.CompactionTargetRatio = def.Lifecycle.CompactionTargetRatio
	}
	if strings.TrimSpace(s.Embedding.Model) == "" {
		s.Embedding = memory.EmbeddingSettings{}
	}
	return s
}

func resolveMemoryEmbedding(c *Config, mc MemoryEmbeddingConfig) memory.EmbeddingSettings {
	if c != nil {
		if id := strings.TrimSpace(mc.ProviderID); id != "" && strings.TrimSpace(mc.Model) != "" {
			if resolved, ok := c.embeddingFromProvider(id, mc.Model); ok {
				return resolved
			}
		}
		for _, entry := range c.Providers {
			if !entrySupportsEmbeddings(entry) {
				continue
			}
			if model := strings.TrimSpace(entry.Model); model != "" && isEmbeddingModelName(model) {
				if resolved, ok := c.embeddingFromProvider(entry.ID, model); ok {
					return resolved
				}
			}
		}
	}

	provider := strings.TrimSpace(mc.Provider)
	model := strings.TrimSpace(mc.Model)
	if provider == "" || model == "" {
		return memory.EmbeddingSettings{}
	}
	out := memory.EmbeddingSettings{
		Provider: provider,
		Model:    model,
		BaseURL:  strings.TrimSpace(mc.BaseURL),
		APIKey:   mc.APIKey,
	}
	if out.APIKey == "" && usesOpenAIEmbeddingAPI(out.Provider) {
		out.APIKey = firstNonEmpty(os.Getenv("OPENAI_API_KEY"), os.Getenv("COMETMIND_API_KEY"))
	}
	if out.BaseURL == "" && usesOpenAIEmbeddingAPI(out.Provider) && c != nil {
		out.BaseURL = c.BaseURL
	}
	return out
}

func (c *Config) embeddingFromProvider(providerID, model string) (memory.EmbeddingSettings, bool) {
	entry := c.FindProvider(providerID)
	if entry == nil {
		return memory.EmbeddingSettings{}, false
	}
	method := strings.TrimSpace(entry.Method)
	if method == "" {
		method = providerMethodFromID(entry.ID)
	}
	if !entrySupportsEmbeddings(*entry) {
		return memory.EmbeddingSettings{}, false
	}
	embedProvider := embeddingProviderForMethod(method)
	if embedProvider == "" {
		return memory.EmbeddingSettings{}, false
	}
	key, err := ProviderAPIKey(c, entry, method)
	if err != nil {
		key = entry.APIKey
	}
	baseURL := strings.TrimSpace(entry.BaseURL)
	if baseURL == "" {
		baseURL = c.BaseURL
	}
	return memory.EmbeddingSettings{
		Provider: embedProvider,
		Model:    strings.TrimSpace(model),
		BaseURL:  baseURL,
		APIKey:   key,
	}, true
}

func entrySupportsEmbeddings(entry ProviderEntry) bool {
	switch strings.TrimSpace(entry.Method) {
	case ProviderOpenAI, ProviderOpenAICompat, ProviderOpencodeGo:
		return true
	default:
		return providerMethodFromID(entry.ID) != ProviderAnthropic
	}
}

func providerMethodFromID(id string) string {
	switch strings.TrimSpace(id) {
	case ProviderAnthropic, ProviderOpenAI, ProviderOpenAICompat, ProviderOpencodeGo:
		return id
	default:
		return ProviderOpenAICompat
	}
}

func embeddingProviderForMethod(method string) string {
	switch strings.TrimSpace(method) {
	case ProviderOpenAI:
		return ProviderOpenAI
	case ProviderOpenAICompat, ProviderOpencodeGo:
		return ProviderOpenAICompat
	default:
		return ""
	}
}

func isEmbeddingModelName(model string) bool {
	return strings.Contains(strings.ToLower(strings.TrimSpace(model)), "embed")
}

func usesOpenAIEmbeddingAPI(provider string) bool {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case ProviderOpenAI, ProviderOpenAICompat, ProviderOpencodeGo, "":
		return true
	default:
		return false
	}
}

func (c *Config) memoryBehaviorConfigured() bool {
	return c.Memory.Enabled || c.Memory.AutoExtract || c.Memory.AutoRetrieve ||
		c.Memory.MaxRetrieved > 0 || c.Memory.SimilarityThreshold > 0 ||
		strings.TrimSpace(c.Memory.ExtractionProvider) != "" ||
		strings.TrimSpace(c.Memory.ExtractionModel) != "" ||
		c.Memory.Lifecycle.DecayHalfLifeDays > 0 ||
		c.Memory.Lifecycle.MaxMemories > 0
}

func (c *Config) memoryEmbeddingConfigured() bool {
	return strings.TrimSpace(c.Memory.Embedding.ProviderID) != "" ||
		strings.TrimSpace(c.Memory.Embedding.Provider) != "" ||
		strings.TrimSpace(c.Memory.Embedding.Model) != ""
}

// MemoryRuntimeEnabled reports whether the memory subsystem should start.
func (c *Config) MemoryRuntimeEnabled() bool {
	return c.Memory.Enabled || c.Memory.AutoExtract || c.Memory.AutoRetrieve || c.memoryEmbeddingConfigured()
}

// EffectiveMemoryConfig returns memory settings merged with defaults for API responses.
func (c *Config) EffectiveMemoryConfig() MemoryConfig {
	s := c.MemorySettings()
	emb := c.Memory.Embedding
	if strings.TrimSpace(s.Embedding.Model) != "" {
		emb.Provider = s.Embedding.Provider
		emb.Model = s.Embedding.Model
		emb.BaseURL = s.Embedding.BaseURL
		emb.APIKey = s.Embedding.APIKey
	}
	return MemoryConfig{
		Enabled:             s.Enabled,
		AutoExtract:         s.AutoExtract,
		AutoRetrieve:        s.AutoRetrieve,
		MaxRetrieved:        s.MaxRetrieved,
		SimilarityThreshold: s.SimilarityThreshold,
		ExtractionProvider:  s.ExtractionProvider,
		ExtractionModel:     s.ExtractionModel,
		Lifecycle: MemoryLifecycleConfig{
			DecayHalfLifeDays:     s.Lifecycle.DecayHalfLifeDays,
			ForgetThreshold:       s.Lifecycle.ForgetThreshold,
			UsageBoostFactor:      s.Lifecycle.UsageBoostFactor,
			MaxUsageBoost:         s.Lifecycle.MaxUsageBoost,
			MaxMemories:           s.Lifecycle.MaxMemories,
			CompactionTargetRatio: s.Lifecycle.CompactionTargetRatio,
			CompactionOnExtract:   s.Lifecycle.CompactionOnExtract,
		},
		Embedding: emb,
	}
}

func (c *Config) memoryConfigured() bool {
	return c.memoryBehaviorConfigured() || c.memoryEmbeddingConfigured()
}

// ExtractionLLMForSession returns the provider id and model id to use for memory
// extraction. Configured extraction provider/model override the session values
// when set; otherwise the session's own provider/model are used.
func (c *Config) ExtractionLLMForSession(providerID, modelID string) (string, string) {
	outProvider := strings.TrimSpace(providerID)
	outModel := strings.TrimSpace(modelID)
	if c == nil {
		return outProvider, outModel
	}
	if pid := strings.TrimSpace(c.Memory.ExtractionProvider); pid != "" {
		outProvider = pid
	}
	if model := strings.TrimSpace(c.Memory.ExtractionModel); model != "" {
		outModel = model
	}
	return outProvider, outModel
}
