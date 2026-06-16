package memory

import (
	"math"
	"time"
)

// ScoredMemory is an active memory with computed weights.
type ScoredMemory struct {
	Record
	Similarity      float64
	EffectiveWeight float64
	RetrievalScore  float64
}

// EffectiveWeight computes base_weight × decay × usage_boost.
func EffectiveWeight(m Record, now time.Time, lc LifecycleSettings) float64 {
	if m.Pinned {
		return m.BaseWeight
	}
	ref := m.CreatedAt
	if m.LastAccessedAt != nil {
		ref = *m.LastAccessedAt
	}
	days := now.Sub(ref).Hours() / 24
	if days < 0 {
		days = 0
	}
	halfLife := lc.DecayHalfLifeDays
	if halfLife <= 0 {
		halfLife = 30
	}
	decay := math.Pow(0.5, days/halfLife)
	boostFactor := lc.UsageBoostFactor
	if boostFactor <= 0 {
		boostFactor = 0.15
	}
	maxBoost := lc.MaxUsageBoost
	if maxBoost <= 0 {
		maxBoost = 2.0
	}
	usageBoost := 1 + math.Log1p(float64(m.AccessCount))*boostFactor
	if usageBoost > maxBoost {
		usageBoost = maxBoost
	}
	return m.BaseWeight * decay * usageBoost
}

// RetrievalScore combines semantic similarity with effective weight.
func RetrievalScore(similarity, effectiveWeight float64) float64 {
	return similarity * effectiveWeight
}
