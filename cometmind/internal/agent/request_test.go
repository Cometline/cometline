package agent

import (
	"strings"
	"testing"

	cometsdk "github.com/cometline/comet-sdk"
	"github.com/cometline/cometmind/internal/session"
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

func TestFinalAnswerNudgeMessages_IsTrailingUserInstruction(t *testing.T) {
	t.Parallel()
	msgs := FinalAnswerNudgeMessages()
	if len(msgs) != 1 || msgs[0].Role != cometsdk.RoleUser {
		t.Fatalf("messages = %#v", msgs)
	}
	text, ok := msgs[0].Content[0].(cometsdk.TextBlock)
	if !ok || !strings.Contains(text.Text, "Do not call tools") {
		t.Fatalf("content = %#v", msgs[0].Content)
	}
}

func TestFormatOutputBudgetPromptBlock_IsModeNeutral(t *testing.T) {
	t.Parallel()
	got := FormatOutputBudgetPromptBlock(4096)
	for _, want := range []string{"4096", "one large tool call", "parallel"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in:\n%s", want, got)
		}
	}
	if strings.Contains(got, "write_file") {
		t.Fatalf("mode-neutral budget prompt mentions write_file:\n%s", got)
	}
}

func TestOutputContinuationPrompts_AreModeNeutral(t *testing.T) {
	t.Parallel()
	for name, prompt := range map[string]string{
		"text truncation":      FormatOutputTruncationContinueBlock(),
		"incomplete tool call": FormatIncompleteToolTruncationContinueBlock(),
	} {
		if strings.Contains(prompt, "write_file") {
			t.Fatalf("%s prompt mentions write_file:\n%s", name, prompt)
		}
		if !strings.Contains(prompt, "one") || !strings.Contains(prompt, "tool call") {
			t.Fatalf("%s prompt lacks bounded tool guidance:\n%s", name, prompt)
		}
	}
}

func TestFormatAgentModePrompt(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		mode    session.AgentMode
		want    []string
		notWant []string
	}{
		{
			name:    "empty",
			notWant: []string{"mode is active"},
		},
		{
			name:    "auto",
			mode:    session.AgentModeAuto,
			want:    []string{"Auto mode is active", "end to end", "write_file"},
			notWant: []string{"Research and design only"},
		},
		{
			name:    "plan",
			mode:    session.AgentModePlan,
			want:    []string{"Plan mode is active", "Research and design only", "Do not claim that changes were made"},
			notWant: []string{"write_file", "make the necessary changes"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := FormatAgentModePrompt(tt.mode)
			if tt.name == "empty" && got != "" {
				t.Fatalf("empty mode prompt = %q, want empty", got)
			}
			for _, want := range tt.want {
				if !strings.Contains(got, want) {
					t.Fatalf("missing %q in:\n%s", want, got)
				}
			}
			for _, notWant := range tt.notWant {
				if strings.Contains(got, notWant) {
					t.Fatalf("unexpected %q in:\n%s", notWant, got)
				}
			}
		})
	}
}

func TestBuildSystemPrompt_AppendsAgentModeLast(t *testing.T) {
	t.Parallel()
	r := &Runner{SystemPrompt: "base", AgentMode: session.AgentModePlan}
	got := r.buildSystemPrompt("summary", 4096)
	mode := FormatAgentModePrompt(session.AgentModePlan)
	if !strings.HasSuffix(got, mode) {
		t.Fatalf("mode prompt is not last:\n%s", got)
	}
	for _, want := range []string{"base", "Earlier conversation summary", "4096", mode} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in:\n%s", want, got)
		}
	}
}
