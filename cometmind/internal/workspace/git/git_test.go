package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseScope(t *testing.T) {
	s, err := ParseScope("")
	if err != nil || s != ScopeWorking {
		t.Fatalf("empty: got %q %v", s, err)
	}
	s, err = ParseScope("staged")
	if err != nil || s != ScopeStaged {
		t.Fatalf("staged: got %q %v", s, err)
	}
	if _, err := ParseScope("nope"); err == nil {
		t.Fatal("expected error for invalid scope")
	}
}

func TestStatus_NotARepo(t *testing.T) {
	dir := t.TempDir()
	res, err := Status(context.Background(), dir, ScopeWorking)
	if err != nil {
		t.Fatal(err)
	}
	if res.IsRepo {
		t.Fatal("expected is_repo=false")
	}
	if len(res.Files) != 0 {
		t.Fatalf("files: %v", res.Files)
	}
}

func TestStatusAndDiff_ModifiedAndUntracked(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir := initRepo(t)

	// Modify tracked file + add untracked.
	if err := os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("v2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "new.txt"), []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := Status(context.Background(), dir, ScopeWorking)
	if err != nil {
		t.Fatal(err)
	}
	if !res.IsRepo {
		t.Fatalf("expected repo: %+v", res)
	}
	if res.Branch == "" {
		t.Fatal("expected branch")
	}
	byPath := map[string]FileStatus{}
	for _, f := range res.Files {
		byPath[f.Path] = f
	}
	if f, ok := byPath["readme.txt"]; !ok || f.Status != "modified" {
		t.Fatalf("readme: %+v files=%v", byPath["readme.txt"], res.Files)
	}
	if f, ok := byPath["new.txt"]; !ok || !f.Untracked {
		t.Fatalf("new: %+v", byPath["new.txt"])
	}

	diff, err := Diff(context.Background(), dir, "readme.txt", ScopeWorking)
	if err != nil {
		t.Fatal(err)
	}
	if diff.Empty || diff.Binary || !strings.Contains(diff.Diff, "+v2") {
		t.Fatalf("diff: %+v", diff)
	}

	udiff, err := Diff(context.Background(), dir, "new.txt", ScopeWorking)
	if err != nil {
		t.Fatal(err)
	}
	if udiff.Empty || !strings.Contains(udiff.Diff, "+hello") {
		t.Fatalf("untracked diff: %+v", udiff)
	}
}

func TestStatus_FiltersToWorkspaceSubdir(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	root := initRepo(t)
	sub := filepath.Join(root, "pkg")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "a.go"), []byte("package a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "outside.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(t, root, "git", "add", ".")
	run(t, root, "git", "commit", "-m", "add pkg")

	if err := os.WriteFile(filepath.Join(sub, "a.go"), []byte("package a\n// changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "outside.txt"), []byte("y\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := Status(context.Background(), sub, ScopeWorking)
	if err != nil {
		t.Fatal(err)
	}
	if !res.IsRepo {
		t.Fatalf("expected repo from subdir: %+v", res)
	}
	for _, f := range res.Files {
		if strings.Contains(f.Path, "outside") {
			t.Fatalf("should not list outside path: %v", res.Files)
		}
		if f.Path != "a.go" {
			t.Fatalf("unexpected path %q (want workspace-relative a.go)", f.Path)
		}
	}
}

func TestDiff_PathEscape(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir := initRepo(t)
	_, err := Diff(context.Background(), dir, "../outside", ScopeWorking)
	if err == nil {
		t.Fatal("expected path escape error")
	}
}

func TestParsePorcelain_Scope(t *testing.T) {
	input := "" +
		" M work.txt\n" +
		"M  staged.txt\n" +
		"MM both.txt\n" +
		"?? untracked.txt\n" +
		" D deleted.txt\n"

	working := parsePorcelain(input, ScopeWorking, "")
	paths := map[string]bool{}
	for _, f := range working {
		paths[f.Path] = true
	}
	if !paths["work.txt"] || !paths["untracked.txt"] || !paths["both.txt"] || !paths["deleted.txt"] {
		t.Fatalf("working: %v", working)
	}
	if paths["staged.txt"] {
		t.Fatalf("staged-only should not appear in working: %v", working)
	}

	staged := parsePorcelain(input, ScopeStaged, "")
	paths = map[string]bool{}
	for _, f := range staged {
		paths[f.Path] = true
	}
	if !paths["staged.txt"] || !paths["both.txt"] {
		t.Fatalf("staged: %v", staged)
	}
	if paths["work.txt"] || paths["untracked.txt"] {
		t.Fatalf("unstaged/untracked should not appear in staged: %v", staged)
	}
}

func initRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run(t, dir, "git", "init")
	run(t, dir, "git", "config", "user.email", "test@example.com")
	run(t, dir, "git", "config", "user.name", "Test")
	// Avoid default branch name variance.
	run(t, dir, "git", "checkout", "-b", "main")
	if err := os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("v1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(t, dir, "git", "add", "readme.txt")
	run(t, dir, "git", "commit", "-m", "init")
	return dir
}

func run(t *testing.T, dir string, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v: %v\n%s", name, args, err, out)
	}
}
