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
//	effectiveMaxTokens = min(userMaxTokens, catalogOutput) when catalogOutput > 0
//	reserve            = max(effectiveMaxTokens, 20_000)
//	available          = context - reserve
func ResolveSessionBudget(cfg *config.Config, providerID, modelID string, userMaxTokens int) SessionBudget {
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

	effective := EffectiveMaxTokens(userMaxTokens, limits.Output)
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

// EffectiveMaxTokens caps the user max-tokens setting by catalog output when known.
func EffectiveMaxTokens(userMaxTokens, catalogOutput int) int {
	if userMaxTokens <= 0 {
		userMaxTokens = 2048
	}
	if catalogOutput > 0 && catalogOutput < userMaxTokens {
		return catalogOutput
	}
	return userMaxTokens
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
