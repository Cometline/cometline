package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestStageUnstageCommitDiscard(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir := initRepo(t)

	// Modify tracked + add untracked.
	if err := os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("v2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "new.txt"), []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := Stage(context.Background(), dir, []string{"readme.txt", "new.txt"}); err != nil {
		t.Fatal(err)
	}

	staged, err := Status(context.Background(), dir, ScopeStaged)
	if err != nil {
		t.Fatal(err)
	}
	if len(staged.Files) != 2 {
		t.Fatalf("staged files = %+v", staged.Files)
	}

	commit, err := Commit(context.Background(), dir, "add changes")
	if err != nil {
		t.Fatal(err)
	}
	if !commit.OK || commit.SHA == "" {
		t.Fatalf("commit = %+v", commit)
	}

	// Our committed paths should no longer appear as changes.
	all, err := Status(context.Background(), dir, ScopeAll)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range all.Files {
		if f.Path == "readme.txt" || f.Path == "new.txt" {
			t.Fatalf("expected clean tracked files, still have %s in %+v", f.Path, all.Files)
		}
	}

	// New modification then discard.
	if err := os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("v3\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Discard(context.Background(), dir, []string{"readme.txt"}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "readme.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "v2\n" {
		t.Fatalf("after discard content = %q", data)
	}

	// Untracked discard deletes the file.
	if err := os.WriteFile(filepath.Join(dir, "tmp.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Discard(context.Background(), dir, []string{"tmp.txt"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "tmp.txt")); !os.IsNotExist(err) {
		t.Fatalf("expected tmp.txt removed, err=%v", err)
	}
}

func TestUnstage(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir := initRepo(t)
	if err := os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("v2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Stage(context.Background(), dir, []string{"readme.txt"}); err != nil {
		t.Fatal(err)
	}
	if _, err := Unstage(context.Background(), dir, []string{"readme.txt"}); err != nil {
		t.Fatal(err)
	}
	working, err := Status(context.Background(), dir, ScopeWorking)
	if err != nil {
		t.Fatal(err)
	}
	var readme *FileStatus
	for i := range working.Files {
		if working.Files[i].Path == "readme.txt" {
			readme = &working.Files[i]
			break
		}
	}
	if readme == nil || readme.Staged {
		t.Fatalf("expected unstaged readme.txt, working = %+v", working.Files)
	}
}

func TestCommit_NothingStaged(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir := initRepo(t)
	_, err := Commit(context.Background(), dir, "nope")
	if err == nil || !strings.Contains(err.Error(), "nothing staged") {
		t.Fatalf("expected nothing staged error, got %v", err)
	}
}

func TestStage_PathEscape(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir := initRepo(t)
	_, err := Stage(context.Background(), dir, []string{"../outside"})
	if err == nil {
		t.Fatal("expected path escape error")
	}
}
