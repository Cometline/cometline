package gateway

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cometline/cometmind/internal/config"
	"github.com/cometline/cometmind/internal/event"
	"github.com/cometline/cometmind/internal/session"
	"github.com/cometline/cometmind/internal/store"
	"github.com/cometline/cometmind/internal/subagent"
)

type routerTestRunner struct{}

func (routerTestRunner) RunTurn(_ context.Context, _ session.Session, _ string, msg InboundMessage, onEvent func(event.Event)) error {
	if onEvent != nil {
		onEvent(event.TextDelta("ok"))
	}
	return nil
}

type subagentNoiseRunner struct{}

func (subagentNoiseRunner) RunTurn(_ context.Context, _ session.Session, _ string, msg InboundMessage, onEvent func(event.Event)) error {
	if onEvent == nil {
		return nil
	}
	onEvent(event.TextDelta("final answer"))
	onEvent(event.SubagentProgress("child-1", "tool", "web_fetch"))
	onEvent(event.SubagentFinished("child-1", "completed", "intermediate summary"))
	return nil
}

type cancelAwareRunner struct {
	started  chan struct{}
	canceled chan struct{}
	cleanup  chan struct{}
}

func (r *cancelAwareRunner) RunTurn(ctx context.Context, _ session.Session, _ string, _ InboundMessage, onEvent func(event.Event)) error {
	if onEvent != nil {
		onEvent(event.TextDelta("partial response"))
	}
	close(r.started)
	<-ctx.Done()
	close(r.canceled)
	<-r.cleanup
	return context.Canceled
}

type cancelAwareTyping struct {
	channelID chan string
	canceled  chan struct{}
}

func (t *cancelAwareTyping) KeepTyping(ctx context.Context, channelID string) func() {
	t.channelID <- channelID
	go func() {
		<-ctx.Done()
		close(t.canceled)
	}()
	return func() {}
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

	if err := r.EnsureThreadSession(ctx, "discord", "user-1", "chan-1", "thread-1"); err != nil {
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

func TestHandleClearSlashClearsMappedSessionTranscript(t *testing.T) {
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

	sess, err := svc.NewSession(ctx, ws.ID, "test-model", "test-provider")
	if err != nil {
		t.Fatalf("NewSession() error = %v", err)
	}
	if _, err := svc.UpsertGatewaySession(ctx, "discord", "user-1", "chan-1", "", sess.ID, ws.ID); err != nil {
		t.Fatalf("UpsertGatewaySession() error = %v", err)
	}
	if _, err := svc.AppendUserMessageContent(ctx, sess.ID, []session.ContentBlock{{Type: "text", Text: "hello"}}, ""); err != nil {
		t.Fatalf("AppendUserMessageContent() error = %v", err)
	}
	if err := svc.SetTitleIfEmpty(ctx, sess.ID, "hello"); err != nil {
		t.Fatalf("SetTitleIfEmpty() error = %v", err)
	}

	msg, err := r.HandleClearSlash(ctx, InboundMessage{
		Platform:  "discord",
		UserID:    "user-1",
		ChannelID: "chan-1",
		Mentioned: true,
	})
	if err != nil {
		t.Fatalf("HandleClearSlash() error = %v", err)
	}
	if msg != "Cleared this CometMind conversation transcript." {
		t.Fatalf("confirmation = %q", msg)
	}

	transcript, err := svc.LoadTranscript(ctx, sess.ID)
	if err != nil {
		t.Fatalf("LoadTranscript() error = %v", err)
	}
	if len(transcript) != 0 {
		t.Fatalf("transcript len = %d, want 0", len(transcript))
	}
	updated, err := svc.GetSession(ctx, sess.ID)
	if err != nil {
		t.Fatalf("GetSession() error = %v", err)
	}
	if updated.Title != "" {
		t.Fatalf("title = %q, want empty", updated.Title)
	}
	if updated.TokenUsage != "{}" {
		t.Fatalf("token_usage = %q, want {}", updated.TokenUsage)
	}
}

func TestHandleClearSlashRequiresExistingSession(t *testing.T) {
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

	_, err = r.HandleClearSlash(ctx, InboundMessage{
		Platform:  "discord",
		UserID:    "user-1",
		ChannelID: "chan-1",
		Mentioned: true,
	})
	if err == nil || !strings.Contains(err.Error(), "no active session in this channel") {
		t.Fatalf("HandleClearSlash() error = %v, want no active session", err)
	}
}

func TestHandleClearSlashRejectsRunningSession(t *testing.T) {
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

	turns := NewTurnRunTracker()
	r := &Router{
		Sessions: svc,
		Turns:    turns,
		Config: &config.Config{
			Gateway: config.GatewayConfig{
				Discord: config.DiscordGatewayConfig{
					WorkspacePath: ws.Path,
				},
			},
		},
	}

	sess, err := svc.NewSession(ctx, ws.ID, "test-model", "test-provider")
	if err != nil {
		t.Fatalf("NewSession() error = %v", err)
	}
	if _, err := svc.UpsertGatewaySession(ctx, "discord", "user-1", "chan-1", "", sess.ID, ws.ID); err != nil {
		t.Fatalf("UpsertGatewaySession() error = %v", err)
	}

	_, finish, err := turns.Start(ctx, sess.ID)
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer finish()

	_, err = r.HandleClearSlash(ctx, InboundMessage{
		Platform:  "discord",
		UserID:    "user-1",
		ChannelID: "chan-1",
		Mentioned: true,
	})
	if err == nil || err.Error() != "session is running" {
		t.Fatalf("HandleClearSlash() error = %v, want session is running", err)
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

func TestHandleInboundReplyOmitsSubagentEvents(t *testing.T) {
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

	var replyText string
	r := &Router{
		Sessions: svc,
		Runner:   subagentNoiseRunner{},
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
	r.SetReplyHandler(func(_ context.Context, msg OutboundMessage) error {
		replyText = msg.Text
		return nil
	})

	if err := r.HandleInbound(ctx, InboundMessage{
		Platform:  "discord",
		UserID:    "user-1",
		ChannelID: "chan-1",
		Text:      "research this",
		Mentioned: true,
	}); err != nil {
		t.Fatalf("HandleInbound() error = %v", err)
	}
	if replyText != "final answer" {
		t.Fatalf("reply = %q, want %q", replyText, "final answer")
	}
}

func TestHandleStopSlashWaitsForTurnAndSubagentCleanup(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, ws, sess := newMappedGatewaySession(t, "thread-1")
	turns := NewTurnRunTracker()
	orch := subagent.NewOrchestrator(5)
	runner := &cancelAwareRunner{
		started:  make(chan struct{}),
		canceled: make(chan struct{}),
		cleanup:  make(chan struct{}),
	}
	typing := &cancelAwareTyping{channelID: make(chan string, 1), canceled: make(chan struct{})}
	r := &Router{
		Sessions:  svc,
		Config:    gatewayTestConfig(ws.Path),
		Runner:    runner,
		Typing:    typing,
		Turns:     turns,
		Subagents: orch,
	}
	replies := make(chan OutboundMessage, 1)
	r.SetReplyHandler(func(_ context.Context, msg OutboundMessage) error {
		replies <- msg
		return nil
	})

	inbound := InboundMessage{
		Platform:  "discord",
		UserID:    "user-1",
		ChannelID: "chan-1",
		ThreadID:  "thread-1",
		Text:      "do work",
		Mentioned: true,
	}
	runDone := make(chan error, 1)
	go func() { runDone <- r.HandleInbound(ctx, inbound) }()
	<-runner.started
	if got := <-typing.channelID; got != "thread-1" {
		t.Fatalf("typing channel = %q, want thread-1", got)
	}

	generalCtx, cancelGeneral := context.WithCancel(context.Background())
	acpCtx, cancelACP := context.WithCancel(context.Background())
	if err := orch.Register(sess.ID, "general-child", subagent.KindGeneral, cancelGeneral); err != nil {
		t.Fatalf("Register(general) error = %v", err)
	}
	if err := orch.Register(sess.ID, "acp-child", subagent.KindACP, cancelACP); err != nil {
		t.Fatalf("Register(acp) error = %v", err)
	}
	childrenDone := make(chan struct{})
	go func() {
		<-generalCtx.Done()
		<-acpCtx.Done()
		orch.Complete("general-child", subagent.Result{Status: "cancelled"})
		orch.Complete("acp-child", subagent.Result{Status: "cancelled"})
		close(childrenDone)
	}()

	stopDone := make(chan struct {
		text string
		err  error
	}, 1)
	go func() {
		text, err := r.HandleStopSlash(ctx, inbound)
		stopDone <- struct {
			text string
			err  error
		}{text: text, err: err}
	}()

	<-runner.canceled
	<-typing.canceled
	<-childrenDone
	select {
	case result := <-stopDone:
		t.Fatalf("stop completed before runner cleanup: %+v", result)
	default:
	}
	if _, err := r.HandleClearSlash(ctx, inbound); err == nil || err.Error() != "session is running" {
		t.Fatalf("HandleClearSlash() during cleanup error = %v, want session is running", err)
	}
	second := inbound
	second.Text = "must not be persisted during cleanup"
	if err := r.HandleInbound(ctx, second); err == nil || !strings.Contains(err.Error(), "already running") {
		t.Fatalf("second HandleInbound() error = %v, want already running", err)
	}
	transcript, err := svc.LoadTranscript(ctx, sess.ID)
	if err != nil {
		t.Fatalf("LoadTranscript() during cleanup error = %v", err)
	}
	if len(transcript) != 1 || transcript[0].Text != "do work" {
		t.Fatalf("transcript during cleanup = %#v, want only the active turn's user message", transcript)
	}

	close(runner.cleanup)
	if err := <-runDone; err != nil {
		t.Fatalf("HandleInbound() error = %v", err)
	}
	result := <-stopDone
	if result.err != nil || result.text != "Stopped the active turn." {
		t.Fatalf("HandleStopSlash() = (%q, %v), want success", result.text, result.err)
	}
	if _, err := r.HandleClearSlash(ctx, inbound); err != nil {
		t.Fatalf("HandleClearSlash() after stop error = %v", err)
	}
	select {
	case reply := <-replies:
		t.Fatalf("received reply after cancellation: %#v", reply)
	default:
	}
}

func TestHandleStopSlashTimeoutAndRepeatedStop(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, ws, sess := newMappedGatewaySession(t, "")
	turns := NewTurnRunTracker()
	_, finish, err := turns.Start(ctx, sess.ID)
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	r := &Router{
		Sessions:        svc,
		Config:          gatewayTestConfig(ws.Path),
		Turns:           turns,
		StopWaitTimeout: 10 * time.Millisecond,
	}
	msg := InboundMessage{Platform: "discord", UserID: "user-1", ChannelID: "chan-1", Mentioned: true}

	text, err := r.HandleStopSlash(ctx, msg)
	if err != nil || text != "Stop requested, but the turn is still cleaning up." {
		t.Fatalf("first HandleStopSlash() = (%q, %v), want cleanup timeout", text, err)
	}
	go func() {
		time.Sleep(5 * time.Millisecond)
		finish()
	}()
	text, err = r.HandleStopSlash(ctx, msg)
	if err != nil || text != "Stopped the active turn." {
		t.Fatalf("second HandleStopSlash() = (%q, %v), want success", text, err)
	}
	text, err = r.HandleStopSlash(ctx, msg)
	if err != nil || text != "There is no active turn to stop." {
		t.Fatalf("third HandleStopSlash() = (%q, %v), want no active turn", text, err)
	}
}

func TestHandleStopSlashWithoutMappedSession(t *testing.T) {
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
	r := &Router{Sessions: svc, Config: gatewayTestConfig(ws.Path), Turns: NewTurnRunTracker()}
	text, err := r.HandleStopSlash(ctx, InboundMessage{
		Platform: "discord", UserID: "user-1", ChannelID: "chan-1", Mentioned: true,
	})
	if err != nil || text != "There is no active turn to stop." {
		t.Fatalf("HandleStopSlash() = (%q, %v), want no active turn", text, err)
	}
}

func newMappedGatewaySession(t *testing.T, threadID string) (*session.Service, session.Workspace, session.Session) {
	t.Helper()
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
	sess, err := svc.NewSession(ctx, ws.ID, "test-model", "test-provider")
	if err != nil {
		t.Fatalf("NewSession() error = %v", err)
	}
	if _, err := svc.UpsertGatewaySession(ctx, "discord", "user-1", "chan-1", threadID, sess.ID, ws.ID); err != nil {
		t.Fatalf("UpsertGatewaySession() error = %v", err)
	}
	return svc, ws, sess
}

func gatewayTestConfig(workspacePath string) *config.Config {
	return &config.Config{
		Model:    "test-model",
		Provider: "test-provider",
		Gateway: config.GatewayConfig{Discord: config.DiscordGatewayConfig{
			WorkspacePath: workspacePath,
		}},
	}
}
