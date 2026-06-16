package gateway

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/cometline/cometmind/internal/config"
	"github.com/cometline/cometmind/internal/session"
	"github.com/cometline/cometmind/internal/store"
)

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
	if r.allowed(InboundMessage{Platform: "discord", GuildID: "guild-1", UserID: "user-1", ChannelID: "chan-1", Mentioned: false}) != false {
		t.Fatal("expected blocked without mention")
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
