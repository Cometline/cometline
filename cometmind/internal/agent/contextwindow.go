package agent

import (
	"strings"

	"github.com/cometline/cometmind/internal/config"
	"github.com/cometline/cometmind/internal/modelcatalog"
)

const (
	defaultContextWindowLimit = 128_000
	contextWindowLimit256K    = 256_000
	// CompactionOutputBuffer is the minimum reserved output budget (OpenCode-style).
	CompactionOutputBuffer = 20_000
	// OutputTokenMax is OpenCode's per-step output ceiling.
	// Requests use min(current model output, OutputTokenMax). Unknown catalogs
	// fall back to this ceiling instead of a model's theoretical maximum.
	OutputTokenMax = 32_000
	// DefaultOutputTokenCap is the unknown-catalog fallback. Same value as OutputTokenMax.
	DefaultOutputTokenCap = OutputTokenMax
)

// SessionBudget is the model-aware compaction / request budget for one turn.
type SessionBudget struct {
	Context            int
	Output             int
	EffectiveMaxTokens int
	Reserve            int
	Available          int
	LimitSource        string
	Vision             bool
	VisionKnown        bool
}

// ResolveContextWindow returns the legacy user-configured fallback window.
// Prefer ResolveSessionBudget for compaction and context_budget SSE.
func ResolveContextWindow(cfg *config.Config) int {
	if cfg == nil {
		return defaultContextWindowLimit
	}
	if cfg.ContextWindowLimit == contextWindowLimit256K {
		return contextWindowLimit256K
	}
	return defaultContextWindowLimit
}

// ResolveSessionBudget computes per-model context window, effective max tokens,
// reserve, and available prompt budget.
//
//	effectiveMaxTokens = min(this model's catalog output, 32_000)
//	reserve            = max(effectiveMaxTokens, 20_000)
//	available          = context - reserve
//
// The last argument is ignored. Output ceiling follows the model on this
// turn, capped like OpenCode, not a stored percent or absolute token count.
func ResolveSessionBudget(cfg *config.Config, providerID, modelID string, _ int) SessionBudget {
	method := ""
	if cfg != nil {
		if p := cfg.FindProvider(providerID); p != nil {
			method = strings.TrimSpace(p.Method)
		}
	}
	if method == "" {
		method = strings.TrimSpace(providerID)
	}

	limits := modelcatalog.ResolveLimits(method, providerID, modelID)
	contextWindow := limits.Context
	if contextWindow <= 0 {
		contextWindow = defaultContextWindowLimit
	}

	effective := EffectiveMaxTokens(limits.Output)
	reserve, available := ComputeReserveAndAvailable(contextWindow, effective)

	return SessionBudget{
		Context:            contextWindow,
		Output:             limits.Output,
		EffectiveMaxTokens: effective,
		Reserve:            reserve,
		Available:          available,
		LimitSource:        limits.Source,
		Vision:             limits.Vision,
		VisionKnown:        limits.VisionKnown,
	}
}

// EffectiveMaxTokens is min(catalog output, OutputTokenMax).
// A missing catalog uses OutputTokenMax, matching OpenCode's maxOutputTokens.
func EffectiveMaxTokens(catalogOutput int) int {
	if catalogOutput <= 0 {
		return OutputTokenMax
	}
	if catalogOutput > OutputTokenMax {
		return OutputTokenMax
	}
	return catalogOutput
}

// ComputeReserveAndAvailable returns reserve=max(effective, 20k) and available=context-reserve.
func ComputeReserveAndAvailable(contextWindow, effectiveMaxTokens int) (reserve, available int) {
	reserve = effectiveMaxTokens
	if reserve < CompactionOutputBuffer {
		reserve = CompactionOutputBuffer
	}
	available = contextWindow - reserve
	if available < 0 {
		available = 0
	}
	return reserve, available
}
