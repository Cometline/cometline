package agent

import (
	"strings"
	"testing"

	cometsdk "github.com/cometline/comet-sdk"
)

func TestContinueUserNudgeMessages_EmptyWhenNoFlags(t *testing.T) {
	t.Parallel()
	if got := ContinueUserNudgeMessages(false, false, false, "", false, ""); got != nil {
		t.Fatalf("got %#v, want nil", got)
	}
}

func TestContinueUserNudgeMessages_CombinesIntoTrailingUser(t *testing.T) {
	t.Parallel()
	msgs := ContinueUserNudgeMessages(true, false, true, "job-1", true, "child done")
	if len(msgs) != 1 {
		t.Fatalf("len = %d, want 1", len(msgs))
	}
	if msgs[0].Role != cometsdk.RoleUser {
		t.Fatalf("role = %q, want user", msgs[0].Role)
	}
	text, ok := msgs[0].Content[0].(cometsdk.TextBlock)
	if !ok {
		t.Fatalf("content = %#v", msgs[0].Content)
	}
	for _, want := range []string{
		FormatOutputTruncationContinueBlock(),
		FormatJobCompletionGateBlock("job-1"),
		FormatWaitForSubagentsBlock(),
		"child done",
	} {
		if !strings.Contains(text.Text, want) {
			t.Fatalf("missing %q in:\n%s", want, text.Text)
		}
	}
}
