package session

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	cometsdk "github.com/cometline/comet-sdk"
	"github.com/cometline/cometmind/internal/db"
	"github.com/cometline/cometmind/internal/media"
	"github.com/cometline/cometmind/internal/store"
)

func newForkTestService(t *testing.T) (*Service, *db.Queries) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "fork-test.db")
	sqlDB, err := store.OpenSQLite(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	return New(sqlDB), db.New(sqlDB)
}

func TestForkSessionRemapsToolCallIDs(t *testing.T) {
	ctx := context.Background()
	svc, _ := newForkTestService(t)

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
	}, nil)
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

func TestForkSessionResetsContextSummary(t *testing.T) {
	ctx := context.Background()
	svc, q := newForkTestService(t)

	srcWs, err := svc.EnsureWorkspace(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("EnsureWorkspace() error = %v", err)
	}
	src, err := svc.NewSession(ctx, srcWs.ID, "model", "provider")
	if err != nil {
		t.Fatalf("NewSession() error = %v", err)
	}

	if err := q.UpdateSessionContextSummary(ctx, db.UpdateSessionContextSummaryParams{
		ContextSummary:          "prior goals and decisions",
		CompactedUntilMessageID: sql.NullString{String: "msg-old", Valid: true},
		ContextSummaryUpdatedAt: sql.NullString{String: "2026-01-01T00:00:00Z", Valid: true},
		ID:                      src.ID,
	}); err != nil {
		t.Fatalf("UpdateSessionContextSummary() error = %v", err)
	}

	forked, err := svc.ForkSession(ctx, src.ID, t.TempDir())
	if err != nil {
		t.Fatalf("ForkSession() error = %v", err)
	}

	got, err := svc.GetSession(ctx, forked.ID)
	if err != nil {
		t.Fatalf("GetSession() error = %v", err)
	}
	if got.ContextSummary != "" {
		t.Fatalf("ContextSummary = %q, want empty", got.ContextSummary)
	}
	if got.CompactedUntilMessageID != "" {
		t.Fatalf("CompactedUntilMessageID = %q, want empty", got.CompactedUntilMessageID)
	}
	if got.ContextSummaryUpdatedAt != "" {
		t.Fatalf("ContextSummaryUpdatedAt = %q, want empty", got.ContextSummaryUpdatedAt)
	}
}

func TestAppendAssistantMediaCatalogsAndForkCopiesFiles(t *testing.T) {
	t.Setenv("COMETMIND_DATA_DIR", t.TempDir())
	ctx := context.Background()
	svc, q := newForkTestService(t)

	srcWs, err := svc.EnsureWorkspace(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("EnsureWorkspace() error = %v", err)
	}
	src, err := svc.NewSession(ctx, srcWs.ID, "model", "provider")
	if err != nil {
		t.Fatalf("NewSession() error = %v", err)
	}

	png := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00, 0x00, 0x0d}
	ref, err := media.RegisterBytes(src.ID, "image/png", "shot", png)
	if err != nil {
		t.Fatalf("RegisterBytes() error = %v", err)
	}
	if _, err := svc.AppendAssistantMedia(ctx, src.ID, []ContentBlock{{
		Type:      "image",
		ID:        ref.ID,
		MediaType: ref.MediaType,
		Alt:       ref.Alt,
	}}); err != nil {
		t.Fatalf("AppendAssistantMedia() error = %v", err)
	}

	catalog, err := q.ListSessionMediaBySession(ctx, nullSessionID(src.ID))
	if err != nil {
		t.Fatalf("ListSessionMediaBySession() error = %v", err)
	}
	if len(catalog) != 1 || catalog[0].ID != ref.ID || catalog[0].Source != "presented" {
		t.Fatalf("catalog = %#v", catalog)
	}

	forked, err := svc.ForkSession(ctx, src.ID, t.TempDir())
	if err != nil {
		t.Fatalf("ForkSession() error = %v", err)
	}
	copied, err := q.ListSessionMediaBySession(ctx, nullSessionID(forked.ID))
	if err != nil {
		t.Fatalf("ListSessionMediaBySession(fork) error = %v", err)
	}
	if len(copied) != 1 || copied[0].ID == ref.ID || copied[0].SourceMediaID != ref.ID || copied[0].Source != "presented" {
		t.Fatalf("forked catalog = %#v", copied)
	}
	gallery, err := svc.ListMedia(ctx, MediaListFilter{})
	if err != nil {
		t.Fatalf("ListMedia() error = %v", err)
	}
	if len(gallery) != 1 || gallery[0].ID != ref.ID {
		t.Fatalf("gallery after fork = %#v", gallery)
	}
	mt, data, err := media.Read(forked.ID, copied[0].ID)
	if err != nil {
		t.Fatalf("Read forked media: %v", err)
	}
	if mt != "image/png" || string(data) != string(png) {
		t.Fatalf("forked media mt=%q len=%d", mt, len(data))
	}

	items, err := svc.LoadTranscript(ctx, forked.ID)
	if err != nil {
		t.Fatalf("LoadTranscript() error = %v", err)
	}
	found := false
	for _, item := range items {
		for _, block := range item.Images {
			if block.ID == copied[0].ID {
				found = true
			}
			if block.ID == ref.ID {
				t.Fatalf("forked transcript still references source media id %q", ref.ID)
			}
		}
	}
	if !found {
		t.Fatal("forked transcript missing remapped media id")
	}
}

func TestImportMediaCopiesAndDeleteLeavesTombstone(t *testing.T) {
	t.Setenv("COMETMIND_DATA_DIR", t.TempDir())
	ctx := context.Background()
	svc, _ := newForkTestService(t)

	srcWs, err := svc.EnsureWorkspace(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	src, err := svc.NewSession(ctx, srcWs.ID, "model", "provider")
	if err != nil {
		t.Fatal(err)
	}
	dest, err := svc.NewSession(ctx, srcWs.ID, "model", "provider")
	if err != nil {
		t.Fatal(err)
	}
	png := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00, 0x00, 0x0d}
	ref, err := media.RegisterBytes(src.ID, "image/png", "shot", png)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.AppendAssistantMedia(ctx, src.ID, []ContentBlock{{
		Type: "image", ID: ref.ID, MediaType: ref.MediaType, Alt: ref.Alt,
	}}); err != nil {
		t.Fatal(err)
	}

	imported, err := svc.ImportMedia(ctx, dest.ID, ref.ID)
	if err != nil {
		t.Fatalf("ImportMedia: %v", err)
	}
	if imported.ID == ref.ID || imported.Source != "imported" || imported.SourceMediaID != ref.ID {
		t.Fatalf("imported = %#v", imported)
	}
	if _, data, err := media.Read(dest.ID, imported.ID); err != nil || string(data) != string(png) {
		t.Fatalf("copied bytes err=%v", err)
	}
	destItems, err := svc.LoadTranscript(ctx, dest.ID)
	if err != nil {
		t.Fatalf("LoadTranscript dest: %v", err)
	}
	foundImport := false
	for _, item := range destItems {
		for _, block := range item.Images {
			if block.ID == imported.ID {
				foundImport = true
			}
		}
	}
	if !foundImport {
		t.Fatal("imported media missing from destination transcript")
	}

	deleted, err := svc.DeleteMedia(ctx, ref.ID)
	if err != nil {
		t.Fatalf("DeleteMedia: %v", err)
	}
	if deleted.Status != "deleted" {
		t.Fatalf("status = %q", deleted.Status)
	}
	if _, _, err := media.Read(src.ID, ref.ID); !errors.Is(err, media.ErrNotFound) {
		t.Fatalf("source file still present: %v", err)
	}
	if _, data, err := media.Read(dest.ID, imported.ID); err != nil || string(data) != string(png) {
		t.Fatalf("imported copy should survive source delete: %v", err)
	}
	listed, err := svc.ListMedia(ctx, MediaListFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 1 || listed[0].ID != imported.ID {
		t.Fatalf("list = %#v", listed)
	}
}

func TestListMediaBackfillsLegacyDiskFiles(t *testing.T) {
	t.Setenv("COMETMIND_DATA_DIR", t.TempDir())
	ctx := context.Background()
	svc, q := newForkTestService(t)
	ws, err := svc.EnsureWorkspace(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	sess, err := svc.NewSession(ctx, ws.ID, "model", "provider")
	if err != nil {
		t.Fatal(err)
	}
	png := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00, 0x00, 0x0d}
	ref, err := media.RegisterBytes(sess.ID, "image/png", "legacy", png)
	if err != nil {
		t.Fatal(err)
	}
	listed, err := q.ListSessionMediaBySession(ctx, nullSessionID(sess.ID))
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 0 {
		t.Fatalf("catalog should start empty, got %#v", listed)
	}
	got, err := svc.ListMedia(ctx, MediaListFilter{})
	if err != nil {
		t.Fatalf("ListMedia: %v", err)
	}
	if len(got) != 1 || got[0].ID != ref.ID || got[0].Source != "presented" {
		t.Fatalf("backfill = %#v", got)
	}
}

func TestListMediaBackfillsGeneratedSourceFromToolResult(t *testing.T) {
	t.Setenv("COMETMIND_DATA_DIR", t.TempDir())
	ctx := context.Background()
	svc, _ := newForkTestService(t)
	ws, err := svc.EnsureWorkspace(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	sess, err := svc.NewSession(ctx, ws.ID, "model", "provider")
	if err != nil {
		t.Fatal(err)
	}
	png := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00, 0x00, 0x0d}
	ref, err := media.RegisterBytes(sess.ID, "image/png", "lighthouse", png)
	if err != nil {
		t.Fatal(err)
	}
	_, toolIDs, err := svc.AppendAssistantStep(ctx, sess.ID, "", nil, []cometsdk.ToolCallBlock{
		{ID: "gen-1", Name: "generate_image", Input: []byte(`{"prompt":"a lighthouse"}`)},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	output := "generated image id=" + ref.ID + " media_type=image/png"
	if err := svc.UpdateToolCallResult(ctx, toolIDs["gen-1"], output, 10, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.AppendToolResultMessage(ctx, sess.ID, toolIDs["gen-1"], output, false); err != nil {
		t.Fatal(err)
	}
	got, err := svc.ListMedia(ctx, MediaListFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != ref.ID || got[0].Source != "generated" || got[0].Prompt != "a lighthouse" {
		t.Fatalf("backfill = %#v", got)
	}
}

func TestDeleteSessionKeepsGalleryMedia(t *testing.T) {
	t.Setenv("COMETMIND_DATA_DIR", t.TempDir())
	ctx := context.Background()
	svc, _ := newForkTestService(t)
	ws, err := svc.EnsureWorkspace(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	sess, err := svc.NewSession(ctx, ws.ID, "model", "provider")
	if err != nil {
		t.Fatal(err)
	}
	png := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00, 0x00, 0x0d}
	ref, err := media.RegisterBytes(sess.ID, "image/png", "shot", png)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.AppendAssistantMedia(ctx, sess.ID, []ContentBlock{{
		Type: "image", ID: ref.ID, MediaType: ref.MediaType, Alt: ref.Alt,
	}}); err != nil {
		t.Fatal(err)
	}

	if err := svc.DeleteSession(ctx, sess.ID); err != nil {
		t.Fatalf("DeleteSession: %v", err)
	}

	listed, err := svc.ListMedia(ctx, MediaListFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 1 || listed[0].ID != ref.ID {
		t.Fatalf("gallery after session delete = %#v", listed)
	}
	if listed[0].SessionID != "" || listed[0].StorageSessionID != sess.ID {
		t.Fatalf("detached media = %#v", listed[0])
	}
	if _, data, err := media.Read(sess.ID, ref.ID); err != nil || string(data) != string(png) {
		t.Fatalf("file should survive session delete: %v", err)
	}

	if err := svc.DeleteWorkspaceByPath(ctx, ws.Path); err != nil {
		t.Fatalf("DeleteWorkspaceByPath: %v", err)
	}
	listed, err = svc.ListMedia(ctx, MediaListFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 1 || listed[0].ID != ref.ID || listed[0].WorkspaceID != "" {
		t.Fatalf("gallery after workspace delete = %#v", listed)
	}
	if _, data, err := media.Read(sess.ID, ref.ID); err != nil || string(data) != string(png) {
		t.Fatalf("file should survive workspace delete: %v", err)
	}
}

func TestClearSessionTranscriptKeepsGalleryMedia(t *testing.T) {
	t.Setenv("COMETMIND_DATA_DIR", t.TempDir())
	ctx := context.Background()
	svc, _ := newForkTestService(t)
	ws, err := svc.EnsureWorkspace(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	sess, err := svc.NewSession(ctx, ws.ID, "model", "provider")
	if err != nil {
		t.Fatal(err)
	}
	png := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00, 0x00, 0x0d}
	ref, err := media.RegisterBytes(sess.ID, "image/png", "shot", png)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.AppendAssistantMedia(ctx, sess.ID, []ContentBlock{{
		Type: "image", ID: ref.ID, MediaType: ref.MediaType, Alt: ref.Alt,
	}}); err != nil {
		t.Fatal(err)
	}

	if err := svc.ClearSessionTranscript(ctx, sess.ID); err != nil {
		t.Fatalf("ClearSessionTranscript: %v", err)
	}

	listed, err := svc.ListMedia(ctx, MediaListFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 1 || listed[0].ID != ref.ID || listed[0].SessionID != sess.ID {
		t.Fatalf("gallery after clear = %#v", listed)
	}
	if _, data, err := media.Read(sess.ID, ref.ID); err != nil || string(data) != string(png) {
		t.Fatalf("file should survive transcript clear: %v", err)
	}
}
