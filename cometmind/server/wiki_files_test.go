package server

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	cometsdk "github.com/cometline/comet-sdk"
	"github.com/cometline/cometmind/internal/event"
	"github.com/cometline/cometmind/internal/session"
)

func TestListWikiFiles(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("COMETMIND_DATA_DIR", dataDir)
	wikiDir := filepath.Join(dataDir, "wiki")
	if err := os.MkdirAll(filepath.Join(wikiDir, "entities"), 0o700); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(wikiDir, "index.md"), "# index")
	mustWrite(t, filepath.Join(wikiDir, "entities", "foo.md"), "# foo")
	mustWrite(t, filepath.Join(wikiDir, "notes.txt"), "note")

	engine, _, cleanup := newTestEngine(t, func(sess session.Session, workspacePath string, mode session.AgentMode) (Runner, error) {
		return fakeRunner(func(ctx context.Context, turn session.AgentTurn, ch chan<- event.Event) error {
			ch <- event.Done()
			return nil
		}), nil
	})
	defer cleanup()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/wiki/files", nil)
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}

	var got workspaceFileListResponse
	decodeJSON(t, rec.Body.Bytes(), &got)
	want := []string{"entities/", "entities/foo.md", "index.md", "notes.txt"}
	if len(got.Files) != len(want) {
		t.Fatalf("files = %v want %v", got.Files, want)
	}
	for i, w := range want {
		if got.Files[i] != w {
			t.Fatalf("files = %v want %v", got.Files, want)
		}
	}
}

func TestListWikiFileChildren(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("COMETMIND_DATA_DIR", dataDir)
	wikiDir := filepath.Join(dataDir, "wiki")
	mustWrite(t, filepath.Join(wikiDir, "index.md"), "# index")
	mustWrite(t, filepath.Join(wikiDir, "entities", "foo.md"), "# foo")
	mustWrite(t, filepath.Join(wikiDir, "entities", "nested", "bar.md"), "# bar")

	engine, _, cleanup := newTestEngine(t, func(sess session.Session, workspacePath string, mode session.AgentMode) (Runner, error) {
		return fakeRunner(func(ctx context.Context, turn session.AgentTurn, ch chan<- event.Event) error {
			ch <- event.Done()
			return nil
		}), nil
	})
	defer cleanup()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/wiki/files/children?directory=entities", nil)
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}

	var got workspaceFileListResponse
	decodeJSON(t, rec.Body.Bytes(), &got)
	want := []string{"entities/foo.md", "entities/nested/"}
	if len(got.Files) != len(want) {
		t.Fatalf("files = %v want %v", got.Files, want)
	}
	for i, path := range want {
		if got.Files[i] != path {
			t.Fatalf("files = %v want %v", got.Files, want)
		}
	}
}

func TestReadWikiFileContent(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("COMETMIND_DATA_DIR", dataDir)
	wikiDir := filepath.Join(dataDir, "wiki")
	mustWrite(t, filepath.Join(wikiDir, "index.md"), "# wiki")

	engine, _, cleanup := newTestEngine(t, func(sess session.Session, workspacePath string, mode session.AgentMode) (Runner, error) {
		return fakeRunner(func(ctx context.Context, turn session.AgentTurn, ch chan<- event.Event) error {
			ch <- event.Done()
			return nil
		}), nil
	})
	defer cleanup()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/wiki/files/content?path=index.md", nil)
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}

	var got workspaceFileTextContent
	decodeJSON(t, rec.Body.Bytes(), &got)
	if got.Kind != "text" || got.Content != "# wiki" {
		t.Fatalf("got = %+v", got)
	}
}

func TestWriteWikiFileContent(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("COMETMIND_DATA_DIR", dataDir)
	wikiDir := filepath.Join(dataDir, "wiki")
	if err := os.MkdirAll(filepath.Join(wikiDir, "entities"), 0o700); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(wikiDir, "entities", "foo.md"), "old")

	engine, _, cleanup := newTestEngine(t, func(sess session.Session, workspacePath string, mode session.AgentMode) (Runner, error) {
		return fakeRunner(func(ctx context.Context, turn session.AgentTurn, ch chan<- event.Event) error {
			ch <- event.Done()
			return nil
		}), nil
	})
	defer cleanup()

	body := bytes.NewBufferString(`{"path":"entities/foo.md","content":"updated"}`)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/v1/wiki/files/content", body)
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}

	data, err := os.ReadFile(filepath.Join(wikiDir, "entities", "foo.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "updated" {
		t.Fatalf("content = %q", string(data))
	}
}

func TestWriteWikiFileContentRejectsRaw(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("COMETMIND_DATA_DIR", dataDir)
	wikiDir := filepath.Join(dataDir, "wiki")
	if err := os.MkdirAll(filepath.Join(wikiDir, "raw"), 0o700); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(wikiDir, "raw", "note.md"), "immutable")

	engine, _, cleanup := newTestEngine(t, func(sess session.Session, workspacePath string, mode session.AgentMode) (Runner, error) {
		return fakeRunner(func(ctx context.Context, turn session.AgentTurn, ch chan<- event.Event) error {
			ch <- event.Done()
			return nil
		}), nil
	})
	defer cleanup()

	body := bytes.NewBufferString(`{"path":"raw/note.md","content":"changed"}`)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/v1/wiki/files/content", body)
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestWriteWikiFileContentRejectsWikiSchema(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("COMETMIND_DATA_DIR", dataDir)
	mustWrite(t, filepath.Join(dataDir, "wiki", "WIKI.md"), "schema")

	engine, _, cleanup := newTestEngine(t, func(sess session.Session, workspacePath string, mode session.AgentMode) (Runner, error) {
		return fakeRunner(func(ctx context.Context, turn session.AgentTurn, ch chan<- event.Event) error {
			ch <- event.Done()
			return nil
		}), nil
	})
	defer cleanup()

	body := bytes.NewBufferString(`{"path":"WIKI.md","content":"changed"}`)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/v1/wiki/files/content", body)
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestPostMessageInlinesWikiFilePaths(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("COMETMIND_DATA_DIR", dataDir)
	mustWrite(t, filepath.Join(dataDir, "wiki", "index.md"), "# wiki index")

	engine, svc, cleanup := newTestEngine(t, func(sess session.Session, workspacePath string, mode session.AgentMode) (Runner, error) {
		return fakeRunner(func(ctx context.Context, turn session.AgentTurn, ch chan<- event.Event) error {
			ch <- event.Done()
			return nil
		}), nil
	})
	defer cleanup()

	ctx := context.Background()
	workspacePath := t.TempDir()
	ws, err := svc.EnsureWorkspace(ctx, workspacePath)
	if err != nil {
		t.Fatal(err)
	}
	sess, err := svc.NewSession(ctx, ws.ID, "test-model", "test-provider")
	if err != nil {
		t.Fatal(err)
	}

	rec := httptest.NewRecorder()
	body := `{"text":"summarize wiki","file_paths":["@runtime/wiki/index.md"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/sessions/"+sess.ID+"/messages", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}

	msgs, err := svc.BuildSDKMessages(ctx, sess.ID)
	if err != nil {
		t.Fatal(err)
	}
	text := msgs[0].Content[0].(cometsdk.TextBlock).Text
	if !strings.Contains(text, "[Referenced file: @runtime/wiki/index.md —") {
		t.Fatalf("missing wiki file path stub: %q", text)
	}
	if strings.Contains(text, "# wiki index") {
		t.Fatalf("wiki body should not be inlined: %q", text)
	}
}

func TestListWikiFileBacklinks(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("COMETMIND_DATA_DIR", dataDir)
	wikiDir := filepath.Join(dataDir, "wiki")
	if err := os.MkdirAll(filepath.Join(wikiDir, "entities"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(wikiDir, "concepts"), 0o700); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(wikiDir, "entities", "a.md"), "See [[b]]")
	mustWrite(t, filepath.Join(wikiDir, "concepts", "b.md"), "target")

	engine, _, cleanup := newTestEngine(t, func(sess session.Session, workspacePath string, mode session.AgentMode) (Runner, error) {
		return fakeRunner(func(ctx context.Context, turn session.AgentTurn, ch chan<- event.Event) error {
			ch <- event.Done()
			return nil
		}), nil
	})
	defer cleanup()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/wiki/files/backlinks?path=concepts/b.md", nil)
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}

	var got struct {
		Backlinks []string `json:"backlinks"`
	}
	decodeJSON(t, rec.Body.Bytes(), &got)
	if len(got.Backlinks) != 1 || got.Backlinks[0] != "entities/a.md" {
		t.Fatalf("backlinks = %v", got.Backlinks)
	}
}
