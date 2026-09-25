package agent

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cometline/cometmind/internal/config"
	"github.com/cometline/cometmind/internal/modelcatalog"
)

func TestResolveContextWindow(t *testing.T) {
	if got := ResolveContextWindow(nil); got != defaultContextWindowLimit {
		t.Fatalf("nil config = %d, want %d", got, defaultContextWindowLimit)
	}
	if got := ResolveContextWindow(&config.Config{}); got != defaultContextWindowLimit {
		t.Fatalf("empty config = %d, want %d", got, defaultContextWindowLimit)
	}
	if got := ResolveContextWindow(&config.Config{ContextWindowLimit: contextWindowLimit256K}); got != contextWindowLimit256K {
		t.Fatalf("256k config = %d, want %d", got, contextWindowLimit256K)
	}
	if got := ResolveContextWindow(&config.Config{ContextWindowLimit: 999_999}); got != defaultContextWindowLimit {
		t.Fatalf("invalid config = %d, want %d", got, defaultContextWindowLimit)
	}
}

func TestEffectiveMaxTokens(t *testing.T) {
	if got := EffectiveMaxTokens(8_192); got != 8_192 {
		t.Fatalf("small model = %d, want 8192", got)
	}
	if got := EffectiveMaxTokens(128_000); got != OutputTokenMax {
		t.Fatalf("large model = %d, want %d", got, OutputTokenMax)
	}
	if got := EffectiveMaxTokens(0); got != OutputTokenMax {
		t.Fatalf("unknown catalog = %d, want %d", got, OutputTokenMax)
	}
}

func TestComputeReserveAndAvailable(t *testing.T) {
	reserve, available := ComputeReserveAndAvailable(128_000, 2048)
	if reserve != CompactionOutputBuffer {
		t.Fatalf("reserve = %d, want %d", reserve, CompactionOutputBuffer)
	}
	if available != 128_000-CompactionOutputBuffer {
		t.Fatalf("available = %d, want %d", available, 128_000-CompactionOutputBuffer)
	}

	reserve, available = ComputeReserveAndAvailable(200_000, 32_000)
	if reserve != 32_000 {
		t.Fatalf("reserve = %d, want 32000", reserve)
	}
	if available != 168_000 {
		t.Fatalf("available = %d, want 168000", available)
	}

	reserve, available = ComputeReserveAndAvailable(10_000, 32_000)
	if available != 0 {
		t.Fatalf("clamped available = %d, want 0", available)
	}
	_ = reserve
}

func TestResolveSessionBudgetUsesCatalog(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "modelcatalog", "testdata", "models-dev-snippet.json"))
	if err != nil {
		t.Fatal(err)
	}
	modelcatalog.ResetCacheForTest()
	if err := modelcatalog.LoadFromJSONForTest(data); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(modelcatalog.ResetCacheForTest)

	cfg := &config.Config{
		MaxTokens: 8192,
		Providers: []config.ProviderEntry{{
			ID:     "anthropic",
			Method: config.ProviderAnthropic,
		}},
	}
	got := ResolveSessionBudget(cfg, "anthropic", "claude-opus-4-1", 0)
	if got.LimitSource != modelcatalog.SourceCatalog {
		t.Fatalf("source = %q, want catalog", got.LimitSource)
	}
	if got.Context != 200_000 {
		t.Fatalf("context = %d, want 200000", got.Context)
	}
	if got.EffectiveMaxTokens != OutputTokenMax {
		t.Fatalf("effective = %d, want %d (min of catalog output and ceiling)", got.EffectiveMaxTokens, OutputTokenMax)
	}
	if got.Reserve != OutputTokenMax {
		t.Fatalf("reserve = %d, want %d", got.Reserve, OutputTokenMax)
	}
	if got.Available != 200_000-OutputTokenMax {
		t.Fatalf("available = %d", got.Available)
	}
}

func TestResolveSessionBudgetFallbackCustom(t *testing.T) {
	modelcatalog.ResetCacheForTest()
	t.Cleanup(modelcatalog.ResetCacheForTest)

	cfg := &config.Config{
		Providers: []config.ProviderEntry{{
			ID:     "local",
			Method: config.ProviderOpenAICompat,
		}},
	}
	got := ResolveSessionBudget(cfg, "local", "llama3", 0)
	if got.LimitSource != modelcatalog.SourceFallback {
		t.Fatalf("source = %q, want fallback", got.LimitSource)
	}
	if got.Context != defaultContextWindowLimit {
		t.Fatalf("context = %d, want %d", got.Context, defaultContextWindowLimit)
	}
	if got.EffectiveMaxTokens != OutputTokenMax {
		t.Fatalf("effective = %d, want %d", got.EffectiveMaxTokens, OutputTokenMax)
	}
	if got.Available != defaultContextWindowLimit-OutputTokenMax {
		t.Fatalf("available = %d", got.Available)
	}
}

func TestShouldCompactUsesAvailable(t *testing.T) {
	if ShouldCompact(10, 100) {
		t.Fatal("under budget should not compact")
	}
	if !ShouldCompact(101, 100) {
		t.Fatal("over budget should compact")
	}
	if !ShouldCompact(1, 0) {
		t.Fatal("zero available should compact")
	}
}
