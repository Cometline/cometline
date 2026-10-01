package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Cometline/cometline/cometmind/internal/event"
	"github.com/Cometline/cometline/cometmind/internal/session"
)

func TestWorkspaceGitStatusAndDiff(t *testing.T) {
	t.Parallel()

	engine, _, cleanup := newTestEngine(t, func(sess session.Session, workspacePath string, mode session.AgentMode) (Runner, error) {
		return fakeRunner(func(ctx context.Context, turn session.AgentTurn, ch chan<- event.Event) error {
			ch <- event.Done()
			return nil
		}), nil
	})
	defer cleanup()

	// Non-repo workspace.
	plain := t.TempDir()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/git/status?workspace_path="+url.QueryEscape(plain), nil)
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var notRepo struct {
		IsRepo bool `json:"is_repo"`
	}
	decodeJSON(t, rec.Body.Bytes(), &notRepo)
	if notRepo.IsRepo {
		t.Fatal("expected is_repo=false for plain dir")
	}

	// Repo with a local modification.
	repo := t.TempDir()
	runGit(t, repo, "init")
	runGit(t, repo, "config", "user.email", "test@example.com")
	runGit(t, repo, "config", "user.name", "Test")
	runGit(t, repo, "checkout", "-b", "main")
	mustWrite(t, filepath.Join(repo, "only-in-test.go"), "package only\n")
	runGit(t, repo, "add", "only-in-test.go")
	runGit(t, repo, "commit", "-m", "init")
	mustWrite(t, filepath.Join(repo, "only-in-test.go"), "package only\n// edit\n")

	// Prefer raw absolute path like other workspace tests (no QueryEscape).
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/git/status?workspace_path="+repo+"&scope=working", nil)
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var status struct {
		IsRepo bool   `json:"is_repo"`
		Branch string `json:"branch"`
		Files  []struct {
			Path   string `json:"path"`
			Status string `json:"status"`
		} `json:"files"`
	}
	decodeJSON(t, rec.Body.Bytes(), &status)
	if !status.IsRepo {
		t.Fatalf("expected is_repo=true for %s: %+v", repo, status)
	}
	found := false
	for _, f := range status.Files {
		if f.Path == "only-in-test.go" && f.Status == "modified" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected only-in-test.go modified in %s; status = %+v", repo, status)
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/git/diff?workspace_path="+repo+"&path=only-in-test.go", nil)
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("diff status = %d body=%s", rec.Code, rec.Body.String())
	}
	var diff struct {
		Path string `json:"path"`
		Diff string `json:"diff"`
	}
	decodeJSON(t, rec.Body.Bytes(), &diff)
	if diff.Path != "only-in-test.go" || !strings.Contains(diff.Diff, "+// edit") {
		t.Fatalf("diff = %+v", diff)
	}

	// Path escape rejected.
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/git/diff?workspace_path="+repo+"&path=../secret", nil)
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("escape status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}
