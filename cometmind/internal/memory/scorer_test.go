package memory

import (
	"testing"
	"time"
)

func TestEffectiveWeightPinnedSkipsDecay(t *testing.T) {
	now := time.Now()
	old := now.Add(-90 * 24 * time.Hour)
	rec := Record{
		BaseWeight:     1.0,
		Pinned:         true,
		LastAccessedAt: &old,
		CreatedAt:      old,
	}
	w := EffectiveWeight(rec, now, DefaultSettings().Lifecycle)
	if w != 1.0 {
		t.Fatalf("pinned weight = %v want 1.0", w)
	}
}

func TestEffectiveWeightDecaysOverTime(t *testing.T) {
	now := time.Now()
	old := now.Add(-30 * 24 * time.Hour)
	rec := Record{
		BaseWeight:     1.0,
		LastAccessedAt: &old,
		CreatedAt:      old,
	}
	lc := LifecycleSettings{DecayHalfLifeDays: 30, UsageBoostFactor: 0.15, MaxUsageBoost: 2}
	w := EffectiveWeight(rec, now, lc)
	if w >= 0.55 || w <= 0.45 {
		t.Fatalf("expected ~0.5 weight after one half-life, got %v", w)
	}
}

func TestEffectiveWeightUsageBoost(t *testing.T) {
	now := time.Now()
	rec := Record{
		BaseWeight:     0.5,
		AccessCount:    10,
		LastAccessedAt: &now,
		CreatedAt:      now,
	}
	lc := LifecycleSettings{DecayHalfLifeDays: 30, UsageBoostFactor: 0.15, MaxUsageBoost: 2}
	w := EffectiveWeight(rec, now, lc)
	if w <= 0.5 {
		t.Fatalf("expected usage boost above base, got %v", w)
	}
}

func TestCosineSimilarityIdentical(t *testing.T) {
	a := []float32{1, 0, 0}
	b := []float32{1, 0, 0}
	if cosineSimilarity(a, b) < 0.99 {
		t.Fatal("expected similarity ~1")
	}
}

func TestRetrievalScore(t *testing.T) {
	s := RetrievalScore(0.8, 0.5)
	if s != 0.4 {
		t.Fatalf("got %v want 0.4", s)
	}
}
