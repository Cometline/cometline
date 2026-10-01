package session

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

func TestLoadTranscriptPageKeyset(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	svc, _ := newForkTestService(t)

	ws, err := svc.EnsureWorkspace(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("EnsureWorkspace: %v", err)
	}
	sess, err := svc.NewSession(ctx, ws.ID, "test-model", "test-provider")
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}

	const turns = 12
	for i := 0; i < turns; i++ {
		if _, err := svc.AppendUserMessage(ctx, sess.ID, fmt.Sprintf("user-%d", i)); err != nil {
			t.Fatalf("AppendUserMessage %d: %v", i, err)
		}
		if _, _, err := svc.AppendAssistantStep(ctx, sess.ID, fmt.Sprintf("assistant-%d", i), nil, nil, nil); err != nil {
			t.Fatalf("AppendAssistantStep %d: %v", i, err)
		}
	}

	full, err := svc.LoadTranscript(ctx, sess.ID)
	if err != nil {
		t.Fatalf("LoadTranscript: %v", err)
	}
	if len(full) != turns*2 {
		t.Fatalf("full len = %d, want %d", len(full), turns*2)
	}

	recent, err := svc.LoadTranscriptPage(ctx, sess.ID, 4, "")
	if err != nil {
		t.Fatalf("LoadTranscriptPage recent: %v", err)
	}
	// limit counts DB messages (user+assistant), so 4 messages → 4 transcript items here.
	if len(recent.Items) != 4 {
		t.Fatalf("recent items = %d, want 4 (%+v)", len(recent.Items), recent.Items)
	}
	if !recent.HasMore || recent.NextBefore == "" {
		t.Fatalf("recent has_more/next_before = %v %q", recent.HasMore, recent.NextBefore)
	}
	last := recent.Items[len(recent.Items)-1]
	if last.Kind != TranscriptKindAssistant || last.Text != "assistant-11" {
		t.Fatalf("recent last = %+v, want assistant-11", last)
	}
	first := recent.Items[0]
	if first.Kind != TranscriptKindUser || first.Text != "user-10" {
		t.Fatalf("recent first = %+v, want user-10 (4 msgs: u10,a10,u11,a11)", first)
	}

	older, err := svc.LoadTranscriptPage(ctx, sess.ID, 4, recent.NextBefore)
	if err != nil {
		t.Fatalf("LoadTranscriptPage older: %v", err)
	}
	if len(older.Items) != 4 {
		t.Fatalf("older items = %d, want 4", len(older.Items))
	}
	if older.Items[0].Text != "user-8" || older.Items[3].Text != "assistant-9" {
		t.Fatalf("older window = %+v", older.Items)
	}
	for _, a := range older.Items {
		for _, b := range recent.Items {
			if a.Kind == b.Kind && a.Text == b.Text {
				t.Fatalf("overlap item %+v", a)
			}
		}
	}

	var walked []TranscriptEntry
	walked = append(walked, recent.Items...)
	cursor := recent.NextBefore
	for i := 0; i < 20 && cursor != ""; i++ {
		page, err := svc.LoadTranscriptPage(ctx, sess.ID, 4, cursor)
		if err != nil {
			t.Fatalf("walk page: %v", err)
		}
		walked = append(page.Items, walked...)
		if !page.HasMore {
			break
		}
		cursor = page.NextBefore
	}
	if len(walked) != len(full) {
		t.Fatalf("walked len = %d, want %d", len(walked), len(full))
	}
	for i := range full {
		if walked[i].Kind != full[i].Kind || walked[i].Text != full[i].Text {
			t.Fatalf("walked[%d] = %+v, full = %+v", i, walked[i], full[i])
		}
	}
}

func TestLoadTranscriptPageInvalidCursor(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	svc, _ := newForkTestService(t)

	ws, err := svc.EnsureWorkspace(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("EnsureWorkspace: %v", err)
	}
	sess, err := svc.NewSession(ctx, ws.ID, "test-model", "test-provider")
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}

	_, err = svc.LoadTranscriptPage(ctx, sess.ID, 10, "not-a-cursor")
	if !errors.Is(err, ErrInvalidTranscriptCursor) {
		t.Fatalf("err = %v, want ErrInvalidTranscriptCursor", err)
	}
}

func TestDecodeTranscriptCursor(t *testing.T) {
	t.Parallel()
	createdAt, id, err := decodeTranscriptCursor("1710000000000:01ABCDEF")
	if err != nil || createdAt != 1710000000000 || id != "01ABCDEF" {
		t.Fatalf("got %d %q %v", createdAt, id, err)
	}
	if _, _, err := decodeTranscriptCursor(""); err == nil {
		t.Fatal("expected error")
	}
	if _, _, err := decodeTranscriptCursor("nocolon"); err == nil {
		t.Fatal("expected error")
	}
}

func TestLoadTranscriptPageSnapsToAnchoringUser(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	svc, _ := newForkTestService(t)

	ws, err := svc.EnsureWorkspace(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("EnsureWorkspace: %v", err)
	}
	sess, err := svc.NewSession(ctx, ws.ID, "test-model", "test-provider")
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}

	// Older complete turn, then a mega-like turn with many assistant steps, then "hi".
	if _, err := svc.AppendUserMessage(ctx, sess.ID, "older"); err != nil {
		t.Fatalf("AppendUserMessage older: %v", err)
	}
	if _, _, err := svc.AppendAssistantStep(ctx, sess.ID, "older-reply", nil, nil, nil); err != nil {
		t.Fatalf("AppendAssistantStep older: %v", err)
	}
	if _, err := svc.AppendUserMessage(ctx, sess.ID, "mega-prompt"); err != nil {
		t.Fatalf("AppendUserMessage mega: %v", err)
	}
	const megaSteps = 8
	for i := 0; i < megaSteps; i++ {
		if _, _, err := svc.AppendAssistantStep(ctx, sess.ID, fmt.Sprintf("mega-step-%d", i), nil, nil, nil); err != nil {
			t.Fatalf("AppendAssistantStep mega %d: %v", i, err)
		}
	}
	if _, err := svc.AppendUserMessage(ctx, sess.ID, "hi"); err != nil {
		t.Fatalf("AppendUserMessage hi: %v", err)
	}
	if _, _, err := svc.AppendAssistantStep(ctx, sess.ID, "hello", nil, nil, nil); err != nil {
		t.Fatalf("AppendAssistantStep hello: %v", err)
	}

	// limit=4 on the recent page without snap would be: mega-step-6,7, hi, hello
	// (4 newest messages) — mid mega turn. Snap must pull back to mega-prompt.
	page, err := svc.LoadTranscriptPage(ctx, sess.ID, 4, "")
	if err != nil {
		t.Fatalf("LoadTranscriptPage: %v", err)
	}
	if len(page.Items) == 0 {
		t.Fatal("empty page")
	}
	if page.Items[0].Kind != TranscriptKindUser || page.Items[0].Text != "mega-prompt" {
		t.Fatalf("page first = %+v, want user mega-prompt (turn-boundary snap)", page.Items[0])
	}
	last := page.Items[len(page.Items)-1]
	if last.Kind != TranscriptKindAssistant || last.Text != "hello" {
		t.Fatalf("page last = %+v, want assistant hello", last)
	}
	// Older history ("older" turn) must remain behind has_more.
	if !page.HasMore || page.NextBefore == "" {
		t.Fatalf("has_more/next_before = %v %q, want true after snap", page.HasMore, page.NextBefore)
	}
	var sawMegaBody bool
	for _, it := range page.Items {
		if it.Kind == TranscriptKindAssistant && it.Text == "mega-step-7" {
			sawMegaBody = true
		}
	}
	if !sawMegaBody {
		t.Fatalf("mega body missing from snapped page: %+v", page.Items)
	}

	older, err := svc.LoadTranscriptPage(ctx, sess.ID, 10, page.NextBefore)
	if err != nil {
		t.Fatalf("older page: %v", err)
	}
	if len(older.Items) == 0 || older.Items[0].Text != "older" {
		t.Fatalf("older page = %+v, want to start at older turn", older.Items)
	}
}
