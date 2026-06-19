package session

import (
	"context"
	"path/filepath"
	"testing"

	cometsdk "github.com/cometline/comet-sdk"
	"github.com/cometline/cometmind/internal/store"
)

func newForkTestService(t *testing.T) *Service {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "fork-test.db")
	sqlDB, err := store.OpenSQLite(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	return New(sqlDB)
}

func TestForkSessionRemapsToolCallIDs(t *testing.T) {
	ctx := context.Background()
	svc := newForkTestService(t)

	srcWs, err := svc.EnsureWorkspace(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("EnsureWorkspace() error = %v", err)
	}
	src, err := svc.NewSession(ctx, srcWs.ID, "model", "provider")
	if err != nil {
		t.Fatalf("NewSession() error = %v", err)
	}

	if _, err := svc.AppendUserMessage(ctx, src.ID, "run pwd"); err != nil {
		t.Fatalf("AppendUserMessage() error = %v", err)
	}
	_, toolIDs, err := svc.AppendAssistantStep(ctx, src.ID, "calling tool", nil, []cometsdk.ToolCallBlock{
		{ID: "provider-1", Name: "run_command", Input: []byte(`{"command":"pwd"}`)},
	})
	if err != nil {
		t.Fatalf("AppendAssistantStep() error = %v", err)
	}
	persistedID := toolIDs["provider-1"]
	if _, err := svc.AppendToolResultMessage(ctx, src.ID, persistedID, "/tmp", false); err != nil {
		t.Fatalf("AppendToolResultMessage() error = %v", err)
	}

	forkWs := t.TempDir()
	forked, err := svc.ForkSession(ctx, src.ID, forkWs)
	if err != nil {
		t.Fatalf("ForkSession() error = %v", err)
	}

	msgs, err := svc.BuildSDKMessages(ctx, forked.ID)
	if err != nil {
		t.Fatalf("BuildSDKMessages() error = %v", err)
	}

	var toolCallID string
	var toolResultID string
	for _, msg := range msgs {
		for _, block := range msg.Content {
			switch b := block.(type) {
			case cometsdk.ToolCallBlock:
				toolCallID = b.ID
			case cometsdk.ToolResultBlock:
				toolResultID = b.ToolCallID
			}
		}
	}

	if toolCallID == "" || toolResultID == "" {
		t.Fatalf("expected both tool_call and tool_result blocks, got call=%q result=%q", toolCallID, toolResultID)
	}
	if toolCallID != toolResultID {
		t.Fatalf("forked tool_call_id mismatch: call=%q result=%q", toolCallID, toolResultID)
	}
	if toolCallID == persistedID {
		t.Fatalf("forked tool_call ID should be remapped, but equals original %q", persistedID)
	}
}
