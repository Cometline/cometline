package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cometline/cometmind/internal/event"
	"github.com/cometline/cometmind/internal/media"
	"github.com/cometline/cometmind/internal/session"
)

func TestMediaHandlersListAndDelete(t *testing.T) {
	t.Setenv("COMETMIND_DATA_DIR", t.TempDir())
	engine, svc, cleanup := newTestEngine(t, func(sess session.Session, workspacePath string, mode session.AgentMode) (Runner, error) {
		return fakeRunner(func(ctx context.Context, turn session.AgentTurn, ch chan<- event.Event) error {
			ch <- event.Done()
			return nil
		}), nil
	})
	defer cleanup()

	ws, err := svc.EnsureWorkspace(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	sess, err := svc.NewSession(context.Background(), ws.ID, "model", "provider")
	if err != nil {
		t.Fatal(err)
	}
	png := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00, 0x00, 0x0d}
	ref, err := media.RegisterBytes(sess.ID, "image/png", "shot", png)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.AppendAssistantMedia(context.Background(), sess.ID, []session.ContentBlock{{
		Type: "image", ID: ref.ID, MediaType: ref.MediaType, Alt: ref.Alt,
	}}); err != nil {
		t.Fatal(err)
	}

	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/media", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("list status = %d body=%s", rec.Code, rec.Body.String())
	}
	var listed struct {
		Items []sessionMediaResource `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &listed); err != nil {
		t.Fatal(err)
	}
	if len(listed.Items) != 1 || listed.Items[0].ID != ref.ID {
		t.Fatalf("list = %#v", listed.Items)
	}

	rec = httptest.NewRecorder()
	engine.ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/api/v1/media/"+ref.ID, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("delete status = %d body=%s", rec.Code, rec.Body.String())
	}
	var deleted sessionMediaResource
	if err := json.Unmarshal(rec.Body.Bytes(), &deleted); err != nil {
		t.Fatal(err)
	}
	if deleted.Status != "deleted" {
		t.Fatalf("deleted = %#v", deleted)
	}

	rec = httptest.NewRecorder()
	engine.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/media", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("list after delete status = %d", rec.Code)
	}
	listed.Items = nil
	if err := json.Unmarshal(rec.Body.Bytes(), &listed); err != nil {
		t.Fatal(err)
	}
	if len(listed.Items) != 0 {
		t.Fatalf("gallery still contains deleted item: %#v", listed.Items)
	}
}

func TestGetSessionMediaSupportsRange(t *testing.T) {
	t.Setenv("COMETMIND_DATA_DIR", t.TempDir())
	engine, svc, cleanup := newTestEngine(t, func(sess session.Session, workspacePath string, mode session.AgentMode) (Runner, error) {
		return fakeRunner(func(ctx context.Context, turn session.AgentTurn, ch chan<- event.Event) error {
			ch <- event.Done()
			return nil
		}), nil
	})
	defer cleanup()

	ws, err := svc.EnsureWorkspace(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	sess, err := svc.NewSession(context.Background(), ws.ID, "model", "provider")
	if err != nil {
		t.Fatal(err)
	}
	png := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00, 0x00, 0x0d}
	ref, err := media.RegisterBytes(sess.ID, "image/png", "shot", png)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/sessions/"+sess.ID+"/media/"+ref.ID, nil)
	req.Header.Set("Range", "bytes=0-3")
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusPartialContent {
		t.Fatalf("range status = %d body=%s", rec.Code, rec.Body.String())
	}
	if rec.Body.Len() != 4 || rec.Body.String() != string(png[:4]) {
		t.Fatalf("range body = %q", rec.Body.String())
	}
}

func TestImportMediaHandlerAppendsToDestination(t *testing.T) {
	t.Setenv("COMETMIND_DATA_DIR", t.TempDir())
	engine, svc, cleanup := newTestEngine(t, func(sess session.Session, workspacePath string, mode session.AgentMode) (Runner, error) {
		return fakeRunner(func(ctx context.Context, turn session.AgentTurn, ch chan<- event.Event) error {
			ch <- event.Done()
			return nil
		}), nil
	})
	defer cleanup()

	ws, err := svc.EnsureWorkspace(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	src, err := svc.NewSession(context.Background(), ws.ID, "model", "provider")
	if err != nil {
		t.Fatal(err)
	}
	dest, err := svc.NewSession(context.Background(), ws.ID, "model", "provider")
	if err != nil {
		t.Fatal(err)
	}
	png := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00, 0x00, 0x0d}
	ref, err := media.RegisterBytes(src.ID, "image/png", "shot", png)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.AppendAssistantMedia(context.Background(), src.ID, []session.ContentBlock{{
		Type: "image", ID: ref.ID, MediaType: ref.MediaType, Alt: ref.Alt,
	}}); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/media/"+ref.ID+"/imports", strings.NewReader(`{"session_id":"`+dest.ID+`"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("import status = %d body=%s", rec.Code, rec.Body.String())
	}
	var imported sessionMediaResource
	if err := json.Unmarshal(rec.Body.Bytes(), &imported); err != nil {
		t.Fatal(err)
	}
	if imported.ID == ref.ID || imported.Source != "imported" || imported.SessionID != dest.ID {
		t.Fatalf("imported = %#v", imported)
	}
	items, err := svc.LoadTranscript(context.Background(), dest.ID)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, item := range items {
		for _, block := range item.Images {
			if block.ID == imported.ID {
				found = true
			}
		}
	}
	if !found {
		t.Fatal("imported media missing from destination transcript")
	}
}

func TestMediaContentSurvivesSessionDelete(t *testing.T) {
	t.Setenv("COMETMIND_DATA_DIR", t.TempDir())
	engine, svc, cleanup := newTestEngine(t, func(sess session.Session, workspacePath string, mode session.AgentMode) (Runner, error) {
		return fakeRunner(func(ctx context.Context, turn session.AgentTurn, ch chan<- event.Event) error {
			ch <- event.Done()
			return nil
		}), nil
	})
	defer cleanup()

	ws, err := svc.EnsureWorkspace(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	sess, err := svc.NewSession(context.Background(), ws.ID, "model", "provider")
	if err != nil {
		t.Fatal(err)
	}
	png := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00, 0x00, 0x0d}
	ref, err := media.RegisterBytes(sess.ID, "image/png", "shot", png)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.AppendAssistantMedia(context.Background(), sess.ID, []session.ContentBlock{{
		Type: "image", ID: ref.ID, MediaType: ref.MediaType, Alt: ref.Alt,
	}}); err != nil {
		t.Fatal(err)
	}

	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/api/v1/sessions/"+sess.ID, nil))
	if rec.Code != http.StatusNoContent && rec.Code != http.StatusOK {
		t.Fatalf("delete session status = %d body=%s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	engine.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/media", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("list status = %d body=%s", rec.Code, rec.Body.String())
	}
	var listed struct {
		Items []sessionMediaResource `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &listed); err != nil {
		t.Fatal(err)
	}
	if len(listed.Items) != 1 || listed.Items[0].ID != ref.ID {
		t.Fatalf("list = %#v", listed.Items)
	}
	if !listed.Items[0].SessionDeleted || listed.Items[0].SessionID != "" || listed.Items[0].StorageSessionID != sess.ID {
		t.Fatalf("detached resource = %#v", listed.Items[0])
	}
	if listed.Items[0].URL != "/api/v1/media/"+ref.ID+"/content" {
		t.Fatalf("url = %q", listed.Items[0].URL)
	}

	rec = httptest.NewRecorder()
	engine.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/media/"+ref.ID+"/content", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("content status = %d body=%s", rec.Code, rec.Body.String())
	}
	if rec.Body.String() != string(png) {
		t.Fatalf("content body = %q", rec.Body.String())
	}

	rec = httptest.NewRecorder()
	engine.ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/api/v1/media/"+ref.ID, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("delete media status = %d body=%s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	engine.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/media", nil))
	if rec.Code != http.StatusOK {
		t.Fatal(rec.Body.String())
	}
	listed.Items = nil
	if err := json.Unmarshal(rec.Body.Bytes(), &listed); err != nil {
		t.Fatal(err)
	}
	if len(listed.Items) != 0 {
		t.Fatalf("gallery still contains deleted item: %#v", listed.Items)
	}
}
