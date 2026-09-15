package session

import (
	"context"
	"database/sql"
	"strings"
	"testing"

	cometsdk "github.com/cometline/comet-sdk"
	"github.com/cometline/cometmind/internal/db"
)

func TestSelectToolCallsToPrune_SkipsRecentTurnsAndProtectsSkill(t *testing.T) {
	t.Parallel()
	old := strings.Repeat("x", (PruneProtectTokens+PruneMinimumTokens+1)*4)
	rows := []db.Message{
		{ID: "u1", Role: "user"},
		{ID: "a1", Role: "assistant"},
		{ID: "u2", Role: "user"},
		{ID: "a2", Role: "assistant"},
		{ID: "u3", Role: "user"},
		{ID: "a3", Role: "assistant"},
	}
	calls := map[string][]db.ToolCall{
		"a1": {
			{ID: "skill-1", MessageID: "a1", ToolName: "skill", Result: old},
			{ID: "old-1", MessageID: "a1", ToolName: "read_file", Result: old},
		},
		"a2": {
			{ID: "prev-1", MessageID: "a2", ToolName: "read_file", Result: old},
		},
		"a3": {
			{ID: "cur-1", MessageID: "a3", ToolName: "read_file", Result: old},
		},
	}

	got := SelectToolCallsToPrune(rows, calls)
	if len(got) != 1 || got[0] != "old-1" {
		t.Fatalf("pruned = %#v, want [old-1]", got)
	}
}

func TestSelectToolCallsToPrune_StopsAtAlreadyCompacted(t *testing.T) {
	t.Parallel()
	old := strings.Repeat("x", (PruneProtectTokens+PruneMinimumTokens+1)*4)
	rows := []db.Message{
		{ID: "u1", Role: "user"},
		{ID: "a0", Role: "assistant"},
		{ID: "u2", Role: "user"},
		{ID: "a1", Role: "assistant"},
		{ID: "u3", Role: "user"},
		{ID: "a2", Role: "assistant"},
		{ID: "u4", Role: "user"},
		{ID: "a3", Role: "assistant"},
	}
	calls := map[string][]db.ToolCall{
		"a0": {{ID: "ancient", MessageID: "a0", ToolName: "read_file", Result: old}},
		"a1": {{
			ID: "already", MessageID: "a1", ToolName: "read_file", Result: old,
			CompactedAt: sql.NullInt64{Int64: 1, Valid: true},
		}},
	}

	got := SelectToolCallsToPrune(rows, calls)
	if got != nil {
		t.Fatalf("pruned = %#v, want nil when walk hits compacted marker first", got)
	}
}

func TestToolResultPromptContent_UsesStubWhenCompacted(t *testing.T) {
	t.Parallel()
	if got := ToolResultPromptContent("hello", true); got != ClearedToolResultStub {
		t.Fatalf("compacted = %q", got)
	}
	if got := ToolResultPromptContent("hello", false); got != "hello" {
		t.Fatalf("live = %q", got)
	}
}

func TestBuildSDKMessages_StubsCompactedToolResults(t *testing.T) {
	ctx := context.Background()
	svc, _ := newForkTestService(t)
	ws, err := svc.EnsureWorkspace(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("EnsureWorkspace() error = %v", err)
	}
	sess, err := svc.NewSession(ctx, ws.ID, "model", "provider")
	if err != nil {
		t.Fatalf("NewSession() error = %v", err)
	}
	if _, err := svc.AppendUserMessage(ctx, sess.ID, "read it"); err != nil {
		t.Fatalf("AppendUserMessage() error = %v", err)
	}
	_, toolIDs, err := svc.AppendAssistantStep(ctx, sess.ID, "reading", nil, []cometsdk.ToolCallBlock{
		{ID: "provider-1", Name: "read_file", Input: []byte(`{"path":"a.txt"}`)},
	}, nil)
	if err != nil {
		t.Fatalf("AppendAssistantStep() error = %v", err)
	}
	persistedID := toolIDs["provider-1"]
	if _, err := svc.AppendToolResultMessage(ctx, sess.ID, persistedID, "FULL TOOL OUTPUT", false); err != nil {
		t.Fatalf("AppendToolResultMessage() error = %v", err)
	}
	if err := svc.MarkToolCallsCompacted(ctx, []string{persistedID}, 1); err != nil {
		t.Fatalf("MarkToolCallsCompacted() error = %v", err)
	}

	msgs, err := svc.BuildSDKMessages(ctx, sess.ID)
	if err != nil {
		t.Fatalf("BuildSDKMessages() error = %v", err)
	}
	var got string
	for _, msg := range msgs {
		if msg.Role != cometsdk.RoleToolResult {
			continue
		}
		for _, block := range msg.Content {
			if tr, ok := block.(cometsdk.ToolResultBlock); ok {
				got = tr.Content
			}
		}
	}
	if got != ClearedToolResultStub {
		t.Fatalf("tool result content = %q, want stub", got)
	}
}
