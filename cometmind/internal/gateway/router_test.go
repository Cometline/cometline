package gateway

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/cometline/cometmind/internal/config"
	"github.com/cometline/cometmind/internal/event"
	"github.com/cometline/cometmind/internal/session"
	"github.com/cometline/cometmind/internal/store"
)

type routerTestRunner struct{}

func (routerTestRunner) RunTurn(_ context.Context, _ session.Session, _, _ string, onEvent func(event.Event)) error {
	if onEvent != nil {
		onEvent(event.TextDelta("ok"))
	}
	return nil
}

func TestRouterAllowed(t *testing.T) {
	t.Parallel()
	r := &Router{
		Config: &config.Config{
			Gateway: config.GatewayConfig{
				Discord: config.DiscordGatewayConfig{
					AllowedUsers:    []string{"user-1"},
					AllowedChannels: []string{"chan-1"},
					RequireMention:  true,
				},
			},
		},
	}

	if r.allowed(InboundMessage{Platform: "discord", GuildID: "guild-1", UserID: "user-1", ChannelID: "chan-1", Mentioned: true}) != true {
		t.Fatal("expected allowed mention")
	}
	if r.allowed(InboundMessage{Platform: "discord", GuildID: "guild-1", UserID: "user-1", ChannelID: "chan-1", ThreadID: "thread-1", ParentChannelID: "chan-1", Mentioned: true}) != true {
		t.Fatal("expected thread allowed via parent channel")
	}
	if r.allowed(InboundMessage{Platform: "discord", GuildID: "guild-1", UserID: "user-1", ChannelID: "chan-1", ThreadID: "thread-1", ParentChannelID: "chan-1", Mentioned: false}) != true {
		t.Fatal("expected thread allowed without mention")
	}
	if r.allowed(InboundMessage{Platform: "discord", GuildID: "guild-1", UserID: "user-1", ChannelID: "chan-1", Mentioned: false}) != false {
		t.Fatal("expected blocked without mention in parent channel")
	}
	if r.allowed(InboundMessage{Platform: "discord", GuildID: "", UserID: "user-1", ChannelID: "dm-chan", Mentioned: true}) != true {
		t.Fatal("expected DM allowed without channel allowlist match")
	}
	if r.allowed(InboundMessage{Platform: "discord", GuildID: "guild-1", UserID: "other", ChannelID: "chan-1", Mentioned: true}) != false {
		t.Fatal("expected blocked user")
	}
}

func TestEnsureThreadSessionCreatesSeparateMapping(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "cometmind.db")
	sqlDB, err := store.OpenSQLite(ctx, dbPath)
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	svc := session.New(sqlDB)
	ws, err := svc.EnsureWorkspace(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("EnsureWorkspace() error = %v", err)
	}

	r := &Router{
		Sessions: svc,
		Config: &config.Config{
			Model:    "test-model",
			Provider: "test-provider",
			Gateway: config.GatewayConfig{
				Discord: config.DiscordGatewayConfig{
					WorkspacePath: ws.Path,
				},
			},
		},
	}

	if err := r.EnsureThreadSession(ctx, "user-1", "chan-1", "thread-1"); err != nil {
		t.Fatalf("EnsureThreadSession() error = %v", err)
	}
	threadMapped, err := svc.LookupGatewaySession(ctx, "discord", "user-1", "chan-1", "thread-1")
	if err != nil {
		t.Fatalf("LookupGatewaySession(thread) error = %v", err)
	}

	parentSess, err := svc.NewSession(ctx, ws.ID, "test-model", "test-provider")
	if err != nil {
		t.Fatalf("NewSession(parent) error = %v", err)
	}
	if _, err := svc.UpsertGatewaySession(ctx, "discord", "user-1", "chan-1", "", parentSess.ID, ws.ID); err != nil {
		t.Fatalf("UpsertGatewaySession(parent) error = %v", err)
	}
	parentMapped, err := svc.LookupGatewaySession(ctx, "discord", "user-1", "chan-1", "")
	if err != nil {
		t.Fatalf("LookupGatewaySession(parent) error = %v", err)
	}

	if threadMapped.CometmindSessionID == parentMapped.CometmindSessionID {
		t.Fatalf("thread and parent share session %q", threadMapped.CometmindSessionID)
	}
}

func TestChangeWorkspaceUpdatesSessionPath(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "cometmind.db")
	sqlDB, err := store.OpenSQLite(ctx, dbPath)
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	svc := session.New(sqlDB)
	ws1, err := svc.EnsureWorkspace(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("EnsureWorkspace(ws1) error = %v", err)
	}
	ws2Dir := t.TempDir()
	ws2, err := svc.EnsureWorkspace(ctx, ws2Dir)
	if err != nil {
		t.Fatalf("EnsureWorkspace(ws2) error = %v", err)
	}

	r := &Router{
		Sessions: svc,
		Config: &config.Config{
			Gateway: config.GatewayConfig{
				Discord: config.DiscordGatewayConfig{
					WorkspacePath: ws1.Path,
				},
			},
		},
	}

	sess, err := svc.NewSession(ctx, ws1.ID, "test-model", "test-provider")
	if err != nil {
		t.Fatalf("NewSession() error = %v", err)
	}
	if _, err := svc.UpsertGatewaySession(ctx, "discord", "user-1", "chan-1", "", sess.ID, ws1.ID); err != nil {
		t.Fatalf("UpsertGatewaySession() error = %v", err)
	}

	msg, err := r.ChangeWorkspace(ctx, InboundMessage{
		Platform:  "discord",
		UserID:    "user-1",
		ChannelID: "chan-1",
		Mentioned: true,
	}, ws2Dir)
	if err != nil {
		t.Fatalf("ChangeWorkspace() error = %v", err)
	}
	if msg == "" {
		t.Fatal("expected confirmation message")
	}

	updated, err := svc.GetSession(ctx, sess.ID)
	if err != nil {
		t.Fatalf("GetSession() error = %v", err)
	}
	if updated.WorkspaceID != ws2.ID {
		t.Fatalf("workspace_id = %q, want %q", updated.WorkspaceID, ws2.ID)
	}
}

func TestSuggestWorkspacePathsIncludesConfiguredDefault(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "cometmind.db")
	sqlDB, err := store.OpenSQLite(ctx, dbPath)
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	svc := session.New(sqlDB)
	ws, err := svc.EnsureWorkspace(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("EnsureWorkspace() error = %v", err)
	}

	r := &Router{
		Sessions: svc,
		Config: &config.Config{
			Gateway: config.GatewayConfig{
				Discord: config.DiscordGatewayConfig{
					WorkspacePath: ws.Path,
				},
			},
		},
	}

	paths, err := r.SuggestWorkspacePaths(ctx, "", 25)
	if err != nil {
		t.Fatalf("SuggestWorkspacePaths() error = %v", err)
	}
	found := false
	for _, path := range paths {
		if path == ws.Path {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("paths = %+v, want %q", paths, ws.Path)
	}
}

func TestHandleInboundPersistsImages(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "cometmind.db")
	sqlDB, err := store.OpenSQLite(ctx, dbPath)
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	svc := session.New(sqlDB)
	ws, err := svc.EnsureWorkspace(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("EnsureWorkspace() error = %v", err)
	}

	r := &Router{
		Sessions: svc,
		Runner:   routerTestRunner{},
		Config: &config.Config{
			Model:    "test-model",
			Provider: "test-provider",
			Gateway: config.GatewayConfig{
				Discord: config.DiscordGatewayConfig{
					WorkspacePath: ws.Path,
				},
			},
		},
	}
	if err := r.HandleInbound(ctx, InboundMessage{
		Platform:  "discord",
		UserID:    "user-1",
		ChannelID: "chan-1",
		Text:      "what is this?",
		Images: []InboundImage{{
			MediaType: "image/png",
			Data:      "aGVsbG8=",
		}},
		Mentioned: true,
	}); err != nil {
		t.Fatalf("HandleInbound() error = %v", err)
	}

	mapped, err := svc.LookupGatewaySession(ctx, "discord", "user-1", "chan-1", "")
	if err != nil {
		t.Fatalf("LookupGatewaySession() error = %v", err)
	}
	transcript, err := svc.LoadTranscript(ctx, mapped.CometmindSessionID)
	if err != nil {
		t.Fatalf("LoadTranscript() error = %v", err)
	}
	if len(transcript) != 1 {
		t.Fatalf("transcript len = %d, want 1", len(transcript))
	}
	if transcript[0].Text != "what is this?" {
		t.Fatalf("text = %q, want prompt", transcript[0].Text)
	}
	if len(transcript[0].Images) != 1 || transcript[0].Images[0].MediaType != "image/png" || transcript[0].Images[0].Data != "aGVsbG8=" {
		t.Fatalf("images = %#v, want one png image", transcript[0].Images)
	}
}
