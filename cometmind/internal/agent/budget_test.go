package agent

import (
	"strings"
	"testing"

	cometsdk "github.com/cometline/comet-sdk"
)

func TestEstimatePromptTokensIncludesSummary(t *testing.T) {
	base := EstimatePromptTokens(PromptBudgetInput{
		System: "system",
	})
	withSummary := EstimatePromptTokens(PromptBudgetInput{
		System:  "system",
		Summary: "prior goals and decisions",
	})
	if withSummary <= base {
		t.Fatalf("summary should increase estimate: base=%d with=%d", base, withSummary)
	}
}

func TestEstimatePromptTokensDoesNotCountImageBase64(t *testing.T) {
	t.Parallel()
	huge := strings.Repeat("A", 800_000)
	got := EstimatePromptTokens(PromptBudgetInput{
		Messages: []cometsdk.Message{{
			Role: cometsdk.RoleUser,
			Content: []cometsdk.Block{
				cometsdk.TextBlock{Text: "look"},
				cometsdk.ImageBlock{MediaType: "image/png", Data: huge},
			},
		}},
	})
	textOnly := EstimateTokens("look")
	if got <= textOnly {
		t.Fatalf("estimate %d should include image tokens above text %d", got, textOnly)
	}
	if got >= len(huge)/4 {
		t.Fatalf("estimate %d must not use base64 chars/4 (%d)", got, len(huge)/4)
	}
	if got > textOnly+3000 {
		t.Fatalf("estimate %d is still far above a vision fallback (text=%d)", got, textOnly)
	}
}
