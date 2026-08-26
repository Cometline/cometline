package usage

import (
	"testing"

	"github.com/cometline/cometmind/internal/modelcatalog"
)

func TestEstimateUSD(t *testing.T) {
	t.Parallel()
	got := EstimateUSD(modelcatalog.Cost{Input: 3, Output: 15, Found: true}, 1_000_000, 1_000_000, 0, 0)
	if got != 18 {
		t.Fatalf("got %v want 18", got)
	}
	if EstimateUSD(modelcatalog.Cost{}, 100, 100, 0, 0) != 0 {
		t.Fatal("unpriced should be 0")
	}
}

func TestNormalizeUsageExclusiveKeepsInput(t *testing.T) {
	t.Parallel()
	got := NormalizeUsage("anthropic", 1000, 50, 200, 100)
	if got != (BilledUsage{Input: 1000, Output: 50, CacheRead: 200, CacheWrite: 100}) {
		t.Fatalf("got %+v", got)
	}
	if got.Tokens() != 1350 {
		t.Fatalf("tokens=%d", got.Tokens())
	}
}

func TestNormalizeUsageInclusiveSubtractsCache(t *testing.T) {
	t.Parallel()
	got := NormalizeUsage("openai", 1000, 50, 200, 0)
	if got != (BilledUsage{Input: 800, Output: 50, CacheRead: 200, CacheWrite: 0}) {
		t.Fatalf("got %+v", got)
	}
	codex := NormalizeUsage("codex", 1000, 10, 400, 0)
	if codex.Input != 600 || codex.CacheRead != 400 {
		t.Fatalf("codex=%+v", codex)
	}
	xai := NormalizeUsage("xai", 1000, 10, 100, 50)
	if xai.Input != 850 {
		t.Fatalf("xai=%+v", xai)
	}
}

func TestNormalizeUsageInclusiveDoesNotUnderflow(t *testing.T) {
	t.Parallel()
	got := NormalizeUsage("openai", 100, 0, 200, 50)
	if got.Input != 0 {
		t.Fatalf("input=%d", got.Input)
	}
	if got.CacheRead != 200 {
		t.Fatalf("cache_read=%d", got.CacheRead)
	}
}

func TestNormalizeUsageCodexWithoutCacheKeepsInput(t *testing.T) {
	t.Parallel()
	got := NormalizeUsage("codex", 1000, 10, 0, 0)
	if got != (BilledUsage{Input: 1000, Output: 10}) {
		t.Fatalf("got %+v", got)
	}
	cost := modelcatalog.Cost{Input: 3, Output: 15, Found: true}
	usd := EstimateUSD(cost, got.Input, got.Output, got.CacheRead, got.CacheWrite)
	if usd != 0.00315 {
		t.Fatalf("codex estimate %v want 0.00315 (still API-priced)", usd)
	}
}

func TestNormalizeUsageUnknownProviderIsExclusive(t *testing.T) {
	t.Parallel()
	got := NormalizeUsage("mystery-gateway", 1000, 0, 200, 0)
	if got.Input != 1000 {
		t.Fatalf("got %+v", got)
	}
}

func TestEstimateUSDUsesCacheRateAfterNormalize(t *testing.T) {
	t.Parallel()
	cost := modelcatalog.Cost{Input: 3, Output: 15, CacheRead: 0.3, CacheWrite: 3.75, Found: true}
	anthropic := NormalizeUsage("anthropic", 1_000_000, 0, 1_000_000, 0)
	got := EstimateUSD(cost, anthropic.Input, anthropic.Output, anthropic.CacheRead, anthropic.CacheWrite)
	if got != 3.3 {
		t.Fatalf("anthropic got %v want 3.3", got)
	}
	openai := NormalizeUsage("openai", 1_000_000, 0, 200_000, 0)
	got = EstimateUSD(cost, openai.Input, openai.Output, openai.CacheRead, openai.CacheWrite)
	want := 0.8*3 + 0.2*0.3
	if got != want {
		t.Fatalf("openai got %v want %v", got, want)
	}
	zeroCache := NormalizeUsage("openai", 1_000_000, 1_000_000, 0, 0)
	got = EstimateUSD(cost, zeroCache.Input, zeroCache.Output, zeroCache.CacheRead, zeroCache.CacheWrite)
	if got != 18 {
		t.Fatalf("zero cache got %v want 18", got)
	}
}
