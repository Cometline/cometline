package agent

import (
	"strings"
	"testing"

	cometsdk "github.com/cometline/comet-sdk"
)

func TestMemoryTokenAllowanceUsesFivePercentCapAndRemainingBudget(t *testing.T) {
	budget := SessionBudget{Available: 100_000}
	if got := memoryTokenAllowance(budget, "", nil, nil); got != 4096 {
		t.Fatalf("allowance = %d, want hard cap 4096", got)
	}
	budget.Available = 10_000
	if got := memoryTokenAllowance(budget, "", nil, nil); got != 500 {
		t.Fatalf("allowance = %d, want 5%% model-aware allowance", got)
	}
	large := []cometsdk.Message{{Role: cometsdk.RoleUser, Content: []cometsdk.Block{cometsdk.TextBlock{Text: strings.Repeat("x", 39_000)}}}}
	if got := memoryTokenAllowance(budget, "", large, nil); got != 250 {
		t.Fatalf("allowance = %d, want remaining budget 250", got)
	}
}
