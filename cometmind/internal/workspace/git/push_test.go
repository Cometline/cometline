package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestPushBranchSetsUpstream(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir := initRepo(t)
	bare := filepath.Join(t.TempDir(), "origin.git")
	if err := exec.Command("git", "init", "--bare", "-b", "main", bare).Run(); err != nil {
		t.Fatal(err)
	}
	run(t, dir, "git", "remote", "add", "origin", bare)
	result, err := PushBranch(context.Background(), dir, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK {
		t.Fatalf("result = %+v", result)
	}
	upstream, err := runGit(context.Background(), dir, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(upstream), "origin/main") {
		t.Fatalf("upstream = %q", upstream)
	}
}

func TestPushBranchRejectsNonFastForward(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir := initRepo(t)
	bare := filepath.Join(t.TempDir(), "origin.git")
	if err := exec.Command("git", "init", "--bare", "-b", "main", bare).Run(); err != nil {
		t.Fatal(err)
	}
	run(t, dir, "git", "remote", "add", "origin", bare)
	if _, err := PushBranch(context.Background(), dir, "origin", "main"); err != nil {
		t.Fatal(err)
	}
	// Advance remote without the local repo knowing.
	parent := t.TempDir()
	clone := filepath.Join(parent, "clone")
	run(t, parent, "git", "clone", bare, clone)
	run(t, clone, "git", "config", "user.email", "test@example.com")
	run(t, clone, "git", "config", "user.name", "Test")
	if err := os.WriteFile(filepath.Join(clone, "readme.txt"), []byte("remote\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(t, clone, "git", "add", "readme.txt")
	run(t, clone, "git", "commit", "-m", "remote")
	run(t, clone, "git", "push", "origin", "main")
	_, err := PushBranch(context.Background(), dir, "origin", "main")
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "fetch") {
		t.Fatalf("expected non-fast-forward message, got %v", err)
	}
}

func TestPushTags(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir := initRepo(t)
	bare := filepath.Join(t.TempDir(), "origin.git")
	if err := exec.Command("git", "init", "--bare", "-b", "main", bare).Run(); err != nil {
		t.Fatal(err)
	}
	run(t, dir, "git", "remote", "add", "origin", bare)
	if _, err := PushBranch(context.Background(), dir, "origin", "main"); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateTag(context.Background(), dir, "v1", "", "release"); err != nil {
		t.Fatal(err)
	}
	if _, err := PushTags(context.Background(), dir, "origin", "v1"); err != nil {
		t.Fatal(err)
	}
}
