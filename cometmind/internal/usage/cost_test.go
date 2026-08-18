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
