package server

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Cometline/cometline/cometmind/internal/event"
	"github.com/Cometline/cometline/cometmind/internal/session"
)

func TestCreateWorkspaceRegistersPath(t *testing.T) {
	t.Parallel()

	engine, svc, cleanup := newTestEngine(t, func(sess session.Session, workspacePath string, mode session.AgentMode) (Runner, error) {
		return fakeRunner(func(ctx context.Context, turn session.AgentTurn, ch chan<- event.Event) error {
			ch <- event.Done()
			return nil
		}), nil
	})
	defer cleanup()

	workspacePath := t.TempDir()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workspaces", bytes.NewBufferString(`{"workspace_path":`+mustJSON(workspacePath)+`}`))
	req.Header.Set("Content-Type", "application/json")

	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d body=%s", rec.Code, http.StatusCreated, rec.Body.String())
	}

	var got workspaceResource
	decodeJSON(t, rec.Body.Bytes(), &got)
	if got.Path != filepath.Clean(workspacePath) {
		t.Fatalf("workspace path = %q, want %q", got.Path, filepath.Clean(workspacePath))
	}
	if got.ID == "" {
		t.Fatalf("expected workspace id to be populated: %+v", got)
	}

	ws, err := svc.LookupWorkspaceByPath(context.Background(), workspacePath)
	if err != nil {
		t.Fatalf("LookupWorkspaceByPath() error = %v", err)
	}
	if ws.ID != got.ID {
		t.Fatalf("lookup workspace id = %q, want %q", ws.ID, got.ID)
	}
}

func TestListWorkspaceFiles(t *testing.T) {
	t.Parallel()

	engine, svc, cleanup := newTestEngine(t, func(sess session.Session, workspacePath string, mode session.AgentMode) (Runner, error) {
		return fakeRunner(func(ctx context.Context, turn session.AgentTurn, ch chan<- event.Event) error {
			ch <- event.Done()
			return nil
		}), nil
	})
	defer cleanup()

	workspacePath := t.TempDir()
	mustWrite(t, filepath.Join(workspacePath, "main.go"), "package main")
	mustWrite(t, filepath.Join(workspacePath, "README.md"), "# readme")
	mustWrite(t, filepath.Join(workspacePath, ".hidden", "secret.go"), "package hidden")
	mustWrite(t, filepath.Join(workspacePath, "node_modules", "x", "index.js"), "x")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/files?workspace_path="+workspacePath, nil)
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var got workspaceFileListResponse
	decodeJSON(t, rec.Body.Bytes(), &got)
	want := []string{
		".hidden/",
		".hidden/secret.go",
		"README.md",
		"main.go",
	}
	if len(got.Files) != len(want) {
		t.Fatalf("files = %v, want %v", got.Files, want)
	}
	for i, w := range want {
		if got.Files[i] != w {
			t.Fatalf("files = %v, want %v", got.Files, want)
		}
	}

	// Verify the workspace was registered.
	_, err := svc.LookupWorkspaceByPath(context.Background(), workspacePath)
	if err != nil {
		t.Fatalf("LookupWorkspaceByPath() error = %v", err)
	}
}

func TestListWorkspaceFileChildren(t *testing.T) {
	t.Parallel()

	engine, _, cleanup := newTestEngine(t, func(sess session.Session, workspacePath string, mode session.AgentMode) (Runner, error) {
		return fakeRunner(func(ctx context.Context, turn session.AgentTurn, ch chan<- event.Event) error {
			ch <- event.Done()
			return nil
		}), nil
	})
	defer cleanup()

	workspacePath := t.TempDir()
	mustWrite(t, filepath.Join(workspacePath, "README.md"), "# readme")
	mustWrite(t, filepath.Join(workspacePath, "src", "main.go"), "package main")
	mustWrite(t, filepath.Join(workspacePath, "src", "internal", "helper.go"), "package internal")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/files/children?workspace_path="+workspacePath, nil)
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("root status = %d body=%s", rec.Code, rec.Body.String())
	}

	var root workspaceFileListResponse
	decodeJSON(t, rec.Body.Bytes(), &root)
	if want := []string{"README.md", "src/"}; !reflect.DeepEqual(root.Files, want) {
		t.Fatalf("root files = %v want %v", root.Files, want)
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/files/children?workspace_path="+workspacePath+"&directory=src", nil)
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("src status = %d body=%s", rec.Code, rec.Body.String())
	}

	var src workspaceFileListResponse
	decodeJSON(t, rec.Body.Bytes(), &src)
	if want := []string{"src/internal/", "src/main.go"}; !reflect.DeepEqual(src.Files, want) {
		t.Fatalf("src files = %v want %v", src.Files, want)
	}
}

func TestReadWorkspaceFileContent(t *testing.T) {
	t.Parallel()

	engine, _, cleanup := newTestEngine(t, func(sess session.Session, workspacePath string, mode session.AgentMode) (Runner, error) {
		return fakeRunner(func(ctx context.Context, turn session.AgentTurn, ch chan<- event.Event) error {
			ch <- event.Done()
			return nil
		}), nil
	})
	defer cleanup()

	workspacePath := t.TempDir()
	mustWrite(t, filepath.Join(workspacePath, "main.go"), "package main")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/files/content?workspace_path="+workspacePath+"&path=main.go", nil)
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var got workspaceFileTextContent
	decodeJSON(t, rec.Body.Bytes(), &got)
	if got.Kind != "text" || got.Content != "package main" || got.Extension != ".go" {
		t.Fatalf("got = %+v", got)
	}
}

func TestWriteWorkspaceFileContent(t *testing.T) {
	t.Parallel()

	engine, _, cleanup := newTestEngine(t, func(sess session.Session, workspacePath string, mode session.AgentMode) (Runner, error) {
		return fakeRunner(func(ctx context.Context, turn session.AgentTurn, ch chan<- event.Event) error {
			ch <- event.Done()
			return nil
		}), nil
	})
	defer cleanup()

	workspacePath := t.TempDir()
	filePath := filepath.Join(workspacePath, "main.go")
	mustWrite(t, filePath, "package main")

	body := bytes.NewBufferString(fmt.Sprintf(`{"workspace_path":%q,"path":"main.go","content":"package main\n\nfunc main() {}"}`, workspacePath))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/v1/workspaces/files/content", body)
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d body=%s", rec.Code, http.StatusNoContent, rec.Body.String())
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("read updated file: %v", err)
	}
	if string(data) != "package main\n\nfunc main() {}" {
		t.Fatalf("file content = %q", string(data))
	}

	readRec := httptest.NewRecorder()
	readReq := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/files/content?workspace_path="+workspacePath+"&path=main.go", nil)
	engine.ServeHTTP(readRec, readReq)
	if readRec.Code != http.StatusOK {
		t.Fatalf("read status = %d, want %d body=%s", readRec.Code, http.StatusOK, readRec.Body.String())
	}

	var got workspaceFileTextContent
	decodeJSON(t, readRec.Body.Bytes(), &got)
	if got.Content != "package main\n\nfunc main() {}" {
		t.Fatalf("round trip content = %q", got.Content)
	}
}

func TestWriteWorkspaceFileContentRejectsInvalidRequests(t *testing.T) {
	t.Parallel()

	engine, _, cleanup := newTestEngine(t, func(sess session.Session, workspacePath string, mode session.AgentMode) (Runner, error) {
		return fakeRunner(func(ctx context.Context, turn session.AgentTurn, ch chan<- event.Event) error {
			ch <- event.Done()
			return nil
		}), nil
	})
	defer cleanup()

	workspacePath := t.TempDir()
	mustWrite(t, filepath.Join(workspacePath, "main.go"), "package main")

	t.Run("path escape", func(t *testing.T) {
		body := bytes.NewBufferString(fmt.Sprintf(`{"workspace_path":%q,"path":"../outside.go","content":"package main"}`, workspacePath))
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPut, "/api/v1/workspaces/files/content", body)
		req.Header.Set("Content-Type", "application/json")
		engine.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
		}
	})

	t.Run("oversized content", func(t *testing.T) {
		content := strings.Repeat("a", maxMessageFileBytes+1)
		body := bytes.NewBufferString(fmt.Sprintf(`{"workspace_path":%q,"path":"main.go","content":%q}`, workspacePath, content))
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPut, "/api/v1/workspaces/files/content", body)
		req.Header.Set("Content-Type", "application/json")
		engine.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
		}
	})

	t.Run("missing file", func(t *testing.T) {
		body := bytes.NewBufferString(fmt.Sprintf(`{"workspace_path":%q,"path":"missing.go","content":"package main"}`, workspacePath))
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPut, "/api/v1/workspaces/files/content", body)
		req.Header.Set("Content-Type", "application/json")
		engine.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
		}
	})
}

func TestListWorkspaceFilesFiltersByQuery(t *testing.T) {
	t.Parallel()

	engine, _, cleanup := newTestEngine(t, func(sess session.Session, workspacePath string, mode session.AgentMode) (Runner, error) {
		return fakeRunner(func(ctx context.Context, turn session.AgentTurn, ch chan<- event.Event) error {
			ch <- event.Done()
			return nil
		}), nil
	})
	defer cleanup()

	workspacePath := t.TempDir()
	mustWrite(t, filepath.Join(workspacePath, "main.go"), "package main")
	mustWrite(t, filepath.Join(workspacePath, "README.md"), "# readme")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/files?workspace_path="+workspacePath+"&q=go", nil)
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var got workspaceFileListResponse
	decodeJSON(t, rec.Body.Bytes(), &got)
	want := []string{"main.go"}
	assertSlice(t, got.Files, want)
}

func TestListWorkspaceFilesIndexSkipsIgnoredPaths(t *testing.T) {
	t.Parallel()

	engine, _, cleanup := newTestEngine(t, func(sess session.Session, workspacePath string, mode session.AgentMode) (Runner, error) {
		return fakeRunner(func(ctx context.Context, turn session.AgentTurn, ch chan<- event.Event) error {
			ch <- event.Done()
			return nil
		}), nil
	})
	defer cleanup()

	workspacePath := t.TempDir()
	mustWrite(t, filepath.Join(workspacePath, "keep.go"), "package main")
	mustWrite(t, filepath.Join(workspacePath, "dist", "bundle.js"), "bundle")
	mustWrite(t, filepath.Join(workspacePath, ".gitignore"), "dist/\n")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/files?index=true&workspace_path="+workspacePath, nil)
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}

	var got workspaceFileListResponse
	decodeJSON(t, rec.Body.Bytes(), &got)
	assertSlice(t, got.Files, []string{".gitignore", "keep.go"})
}

func TestListWorkspaceFilesMissingWorkspace(t *testing.T) {
	t.Parallel()

	engine, _, cleanup := newTestEngine(t, func(sess session.Session, workspacePath string, mode session.AgentMode) (Runner, error) {
		return fakeRunner(func(ctx context.Context, turn session.AgentTurn, ch chan<- event.Event) error {
			ch <- event.Done()
			return nil
		}), nil
	})
	defer cleanup()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/files", nil)
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
}

func assertSlice(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}
