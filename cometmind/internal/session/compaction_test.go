package session

import (
	"strings"
	"testing"

	"github.com/cometline/cometmind/internal/db"
)

func TestRecentWindowStartIndex(t *testing.T) {
	rows := []db.Message{
		{ID: "m1", Role: "user"},
		{ID: "m2", Role: "assistant"},
		{ID: "m3", Role: "user"},
		{ID: "m4", Role: "assistant"},
		{ID: "m5", Role: "user"},
	}
	if got := RecentWindowStartIndex(rows, 2); got != 2 {
		t.Fatalf("RecentWindowStartIndex() = %d, want 2", got)
	}
}

func TestCompactionPrefixRange(t *testing.T) {
	rows := []db.Message{
		{ID: "m1", Role: "user"},
		{ID: "m2", Role: "assistant"},
		{ID: "m3", Role: "user"},
		{ID: "m4", Role: "assistant"},
	}
	start, end := CompactionPrefixRange(rows, "", 2)
	if start != 0 || end != 2 {
		t.Fatalf("range = [%d,%d), want [0,2)", start, end)
	}
	start, end = CompactionPrefixRange(rows, "m1", 3)
	if start != 1 || end != 3 {
		t.Fatalf("range after compact = [%d,%d), want [1,3)", start, end)
	}
}

func TestFilterMessagesAfterCompacted(t *testing.T) {
	rows := []db.Message{
		{ID: "m1", Role: "user"},
		{ID: "m2", Role: "assistant"},
		{ID: "m3", Role: "user"},
	}
	got := FilterMessagesAfterCompacted(rows, "m1")
	if len(got) != 2 || got[0].ID != "m2" {
		t.Fatalf("filtered = %+v", got)
	}
}

func TestFormatTranscriptForSummaryMentionsImages(t *testing.T) {
	t.Parallel()
	raw, err := marshalMessageContent([]ContentBlock{
		{Type: "text", Text: "align the icon"},
		{Type: "image", MediaType: "image/png", Data: "aGVsbG8="},
		{Type: "image", MediaType: "image/png", Data: "aGVsbG8="},
	}, "", nil)
	if err != nil {
		t.Fatalf("marshalMessageContent() error = %v", err)
	}
	got := FormatTranscriptForSummary([]db.Message{{ID: "u1", Role: "user", Content: raw}})
	if !strings.Contains(got, "align the icon") {
		t.Fatalf("summary missing caption: %q", got)
	}
	if !strings.Contains(got, "[user attached 2 image(s)]") {
		t.Fatalf("summary missing image note: %q", got)
	}
}
