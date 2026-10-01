package server

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Cometline/cometline/cometmind/internal/event"
	"github.com/Cometline/cometline/cometmind/internal/session"
)

func TestGetMessagesKeysetPagination(t *testing.T) {
	t.Parallel()

	engine, svc, cleanup := newTestEngine(t, func(sess session.Session, workspacePath string, mode session.AgentMode) (Runner, error) {
		return fakeRunner(func(ctx context.Context, turn session.AgentTurn, ch chan<- event.Event) error {
			return nil
		}), nil
	})
	defer cleanup()

	ctx := context.Background()
	ws, err := svc.EnsureWorkspace(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("EnsureWorkspace: %v", err)
	}
	sess, err := svc.NewSession(ctx, ws.ID, "test-model", "test-provider")
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	for i := 0; i < 6; i++ {
		if _, err := svc.AppendUserMessage(ctx, sess.ID, fmt.Sprintf("u-%d", i)); err != nil {
			t.Fatalf("user %d: %v", i, err)
		}
		if _, _, err := svc.AppendAssistantStep(ctx, sess.ID, fmt.Sprintf("a-%d", i), nil, nil, nil); err != nil {
			t.Fatalf("assistant %d: %v", i, err)
		}
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sessions/"+sess.ID+"/messages?limit=4", nil)
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var page transcriptResponse
	decodeJSON(t, rec.Body.Bytes(), &page)
	if len(page.Items) != 4 || !page.HasMore || page.NextBefore == "" {
		t.Fatalf("page = %+v", page)
	}
	if page.Items[0].Text != "u-4" || page.Items[3].Text != "a-5" {
		t.Fatalf("recent window texts = %+v", page.Items)
	}

	rec2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/sessions/"+sess.ID+"/messages?limit=4&before="+page.NextBefore, nil)
	engine.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("older status = %d body=%s", rec2.Code, rec2.Body.String())
	}
	var older transcriptResponse
	decodeJSON(t, rec2.Body.Bytes(), &older)
	if len(older.Items) != 4 {
		t.Fatalf("older items = %d", len(older.Items))
	}
	if older.Items[0].Text != "u-2" || older.Items[3].Text != "a-3" {
		t.Fatalf("older window = %+v", older.Items)
	}

	rec3 := httptest.NewRecorder()
	req3 := httptest.NewRequest(http.MethodGet, "/api/v1/sessions/"+sess.ID+"/messages?before=bad", nil)
	engine.ServeHTTP(rec3, req3)
	if rec3.Code != http.StatusBadRequest {
		t.Fatalf("bad cursor status = %d, want 400", rec3.Code)
	}
}
