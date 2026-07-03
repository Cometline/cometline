package session

import (
	"encoding/json"
	"testing"

	cometsdk "github.com/cometline/comet-sdk"
)

func TestAPISession(t *testing.T) {
	t.Parallel()

	usageJSON, err := json.Marshal(cometsdk.TokenUsage{
		InputTokens:  100,
		OutputTokens: 50,
		CacheRead:    10,
		CacheWrite:   5,
	})
	if err != nil {
		t.Fatalf("marshal usage: %v", err)
	}

	parentID := "parent-1"
	sess := Session{
		ID:               "sess-1",
		WorkspaceID:      "ws-1",
		Title:            "Test session",
		ModelID:          "claude-sonnet",
		ProviderID:       "anthropic",
		Status:           "idle",
		Origin:           "autonomy",
		TokenUsage:       string(usageJSON),
		ParentSessionID:  parentID,
		Purpose:          "review diff",
		DelegationStatus: DelegationCompleted,
		OutputSummary:    "done",
		ACPSessionID:     "acp-1",
		PendingQuestion:  "legacy",
		SubagentKind:     "coding",
		Gateway: &SessionGateway{
			Platform:  "discord",
			ChannelID: "chan-1",
			ThreadID:  "thread-1",
		},
		Pinned:    true,
		CreatedAt: 1_700_000_000_000,
		UpdatedAt: 1_700_000_100_000,
	}

	got, err := APISession(sess, "/tmp/ws")
	if err != nil {
		t.Fatalf("APISession() error = %v", err)
	}

	if got.Id != sess.ID {
		t.Errorf("Id = %q, want %q", got.Id, sess.ID)
	}
	if got.WorkspacePath != "/tmp/ws" {
		t.Errorf("WorkspacePath = %q, want /tmp/ws", got.WorkspacePath)
	}
	if got.TokenUsage.InputTokens != 100 || got.TokenUsage.OutputTokens != 50 {
		t.Errorf("TokenUsage = %+v, want input=100 output=50", got.TokenUsage)
	}
	if got.ParentSessionId == nil || *got.ParentSessionId != parentID {
		t.Errorf("ParentSessionId = %v, want %q", got.ParentSessionId, parentID)
	}
	if string(got.Origin) != "autonomy" {
		t.Errorf("Origin = %q, want autonomy", got.Origin)
	}
	if got.Gateway == nil || got.Gateway.Platform == nil || string(*got.Gateway.Platform) != "discord" {
		t.Fatalf("Gateway = %+v, want discord platform", got.Gateway)
	}
	if got.Gateway.ChannelId == nil || *got.Gateway.ChannelId != "chan-1" {
		t.Errorf("Gateway.ChannelId = %v, want chan-1", got.Gateway.ChannelId)
	}
	if !got.Pinned {
		t.Error("Pinned = false, want true")
	}
}

func TestAPISessionEmptyTokenUsage(t *testing.T) {
	t.Parallel()

	got, err := APISession(Session{ID: "s1", Status: "idle"}, "/ws")
	if err != nil {
		t.Fatalf("APISession() error = %v", err)
	}
	if got.TokenUsage.InputTokens != 0 || got.TokenUsage.OutputTokens != 0 {
		t.Errorf("TokenUsage = %+v, want zero values", got.TokenUsage)
	}
	if got.Gateway != nil {
		t.Errorf("Gateway = %+v, want nil", got.Gateway)
	}
}

func TestAPISessionList(t *testing.T) {
	t.Parallel()

	sessions := []Session{
		{ID: "s1", WorkspaceID: "ws-a", Status: "idle"},
		{ID: "s2", WorkspaceID: "ws-b", Status: "idle"},
	}
	paths := map[string]string{"ws-a": "/a", "ws-b": "/b"}

	got, err := APISessionList(sessions, paths)
	if err != nil {
		t.Fatalf("APISessionList() error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if got[0].WorkspacePath != "/a" || got[1].WorkspacePath != "/b" {
		t.Errorf("paths = %q, %q; want /a, /b", got[0].WorkspacePath, got[1].WorkspacePath)
	}
}

func TestAPISessionInvalidTokenUsage(t *testing.T) {
	t.Parallel()

	_, err := APISession(Session{TokenUsage: "not-json", Status: "idle"}, "/ws")
	if err == nil {
		t.Fatal("APISession() error = nil, want decode error")
	}
}
