package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cometline/cometmind/internal/config"
	"github.com/cometline/cometmind/internal/event"
	"github.com/cometline/cometmind/internal/inbox"
	"github.com/cometline/cometmind/internal/session"
	"github.com/cometline/cometmind/internal/store"
)

func TestInboxHandlersReplyAndDismiss(t *testing.T) {
	ctx := context.Background()
	sqlDB, err := store.OpenSQLite(ctx, ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()

	sessions := session.New(sqlDB)
	inboxSvc := inbox.NewService(sqlDB)
	hub := event.NewHub()
	engine, err := New(Deps{
		Config:    config.Defaults(),
		Sessions:  sessions,
		Inbox:     inboxSvc,
		Events:    hub,
		NewRunner: func(session.Session, string, session.AgentMode) (Runner, error) { return nil, nil },
	})
	if err != nil {
		t.Fatal(err)
	}

	msg, err := inboxSvc.Create(ctx, inbox.CreateInput{Title: "Hello", Body: "World"})
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/inbox/summary", nil)
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("summary status=%d body=%s", w.Code, w.Body.String())
	}
	var summary struct {
		OpenCount int64 `json:"open_count"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &summary); err != nil {
		t.Fatal(err)
	}
	if summary.OpenCount != 1 {
		t.Fatalf("open_count=%d", summary.OpenCount)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/inbox/messages?status=open", nil)
	w = httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("list status=%d", w.Code)
	}

	replyBody := `{"content":"got it"}`
	req = httptest.NewRequest(http.MethodPost, "/api/v1/inbox/messages/"+msg.ID+"/replies", bytes.NewBufferString(replyBody))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("reply status=%d body=%s", w.Code, w.Body.String())
	}

	msg2, err := inboxSvc.Create(ctx, inbox.CreateInput{Title: "Other", Body: "note"})
	if err != nil {
		t.Fatal(err)
	}
	req = httptest.NewRequest(http.MethodPost, "/api/v1/inbox/messages/"+msg2.ID+"/dismissals", nil)
	w = httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("dismiss status=%d body=%s", w.Code, w.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/inbox/summary", nil)
	w = httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	if err := json.Unmarshal(w.Body.Bytes(), &summary); err != nil {
		t.Fatal(err)
	}
	if summary.OpenCount != 0 {
		t.Fatalf("open_count after archive=%d", summary.OpenCount)
	}
}
