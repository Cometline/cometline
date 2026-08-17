package session

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/cometline/cometmind/internal/db"
)

func TestRecentWindowStartForBudget_shrinksHugeTurn(t *testing.T) {
	huge := strings.Repeat("x", MaxToolResultPromptRunes*20)
	payload, err := json.Marshal(toolResultPayload{ToolCallID: "tc1", Content: huge})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	rows := []db.Message{
		{ID: "u1", Role: "user", Content: "first question"},
		{ID: "a1", Role: "assistant", Content: ""},
		{ID: "t1", Role: "tool_result", Content: string(payload)},
		{ID: "u2", Role: "user", Content: "second question"},
	}
	calls := map[string][]db.ToolCall{
		"a1": {{ID: "tc1", MessageID: "a1", ToolName: "grep", Arguments: `{}`}},
	}

	turnOnly := RecentWindowStartForBudget(rows, calls, 10, 32_000, 2048)
	if turnOnly != 0 {
		t.Fatalf("turn-only recent start = %d, want 0", turnOnly)
	}

	budgeted := RecentWindowStartForBudget(rows, calls, 10, 4_000, 2048)
	if budgeted == 0 {
		t.Fatalf("expected budgeted recent window to drop first turn, got start=0")
	}
	if rows[budgeted].ID != "u2" {
		t.Fatalf("budgeted recent start = %d (%q), want u2", budgeted, rows[budgeted].ID)
	}
}

func TestEstimateRowsTokensCountsImagesAsVisionTokens(t *testing.T) {
	t.Parallel()
	raw, err := marshalMessageContent([]ContentBlock{
		{Type: "text", Text: "look at this"},
		{Type: "image", MediaType: "image/png", Data: strings.Repeat("A", 800_000)},
	}, "", nil)
	if err != nil {
		t.Fatalf("marshalMessageContent() error = %v", err)
	}
	rows := []db.Message{{ID: "u1", Role: "user", Content: raw}}
	got := EstimateRowsTokens(rows, nil)
	textOnly := EstimateTokens("look at this")
	if got <= textOnly {
		t.Fatalf("image row tokens = %d, want more than text-only %d", got, textOnly)
	}
	if got >= 800_000/4 {
		t.Fatalf("image row tokens = %d, must not count base64 chars/4", got)
	}
	if got != textOnly+imageTokenFallback {
		t.Fatalf("image row tokens = %d, want text %d + fallback %d", got, textOnly, imageTokenFallback)
	}
}

func TestRecentWindowStartForBudget_shrinksScreenshotTurn(t *testing.T) {
	t.Parallel()
	screenshot, err := marshalMessageContent([]ContentBlock{
		{Type: "text", Text: "align the sidebar icon"},
		{Type: "image", MediaType: "image/png", Data: strings.Repeat("A", 800_000)},
		{Type: "image", MediaType: "image/png", Data: strings.Repeat("B", 600_000)},
	}, "", nil)
	if err != nil {
		t.Fatalf("marshalMessageContent() error = %v", err)
	}
	rows := []db.Message{
		{ID: "u1", Role: "user", Content: screenshot},
		{ID: "a1", Role: "assistant", Content: "ok"},
		{ID: "u2", Role: "user", Content: "thanks"},
	}

	// Two fallback images (4096) exceed 45% of 4k-2k available (~878).
	got := RecentWindowStartForBudget(rows, nil, 10, 4_000, 2048)
	if got == 0 {
		t.Fatal("expected screenshot turn to be dropped from the recent window")
	}
	if rows[got].ID != "u2" {
		t.Fatalf("recent start = %d (%q), want u2", got, rows[got].ID)
	}
}
