package agent

import (
	"strings"
	"testing"

	cometsdk "github.com/cometline/comet-sdk"
)

func TestContinueUserNudgeMessages_EmptyWhenNoFlags(t *testing.T) {
	t.Parallel()
	if got := ContinueUserNudgeMessages(false, false, false, false, "", false, ""); got != nil {
		t.Fatalf("got %#v, want nil", got)
	}
}

func TestContinueUserNudgeMessages_CombinesIntoTrailingUser(t *testing.T) {
	t.Parallel()
	msgs := ContinueUserNudgeMessages(true, false, false, true, "job-1", true, "child done")
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

func TestContinueUserNudgeMessages_IncompleteToolSupersedesTextTruncation(t *testing.T) {
	t.Parallel()
	msgs := ContinueUserNudgeMessages(true, true, false, false, "", false, "")
	if len(msgs) != 1 {
		t.Fatalf("len = %d, want 1", len(msgs))
	}
	text, ok := msgs[0].Content[0].(cometsdk.TextBlock)
	if !ok {
		t.Fatalf("content = %#v", msgs[0].Content)
	}
	if !strings.Contains(text.Text, FormatIncompleteToolTruncationContinueBlock()) {
		t.Fatalf("missing incomplete-tool nudge in:\n%s", text.Text)
	}
	if strings.Contains(text.Text, FormatOutputTruncationContinueBlock()) {
		t.Fatal("text truncation nudge must not appear when incomplete-tool continue is set")
	}
}

func TestFormatOutputBudgetPromptBlock_MentionsOneWriteFilePerStep(t *testing.T) {
	t.Parallel()
	got := FormatOutputBudgetPromptBlock(4096)
	for _, want := range []string{"4096", "one complete write_file", "parallel"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in:\n%s", want, got)
		}
	}
}
