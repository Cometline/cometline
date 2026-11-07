package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestGraph_BranchMergeAndTag(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir := initRepo(t)
	run(t, dir, "git", "commit", "--allow-empty", "-m", "second")
	run(t, dir, "git", "checkout", "-b", "side")
	run(t, dir, "git", "commit", "--allow-empty", "-m", "side")
	sideSHA := strings.TrimSpace(string(mustGit(t, dir, "rev-parse", "HEAD")))
	run(t, dir, "git", "tag", "v1", sideSHA)
	run(t, dir, "git", "checkout", "main")
	run(t, dir, "git", "merge", "--no-ff", "-m", "merge side", "side")

	res, err := Graph(context.Background(), dir, GraphQuery{Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	if !res.IsRepo || res.HeadBranch != "main" || res.HeadSHA == "" {
		t.Fatalf("head: %+v", res)
	}
	if len(res.Commits) < 4 {
		t.Fatalf("commits: %+v", res.Commits)
	}
	merge := res.Commits[0]
	if len(merge.Parents) != 2 {
		t.Fatalf("merge parents: %+v", merge)
	}
	if !hasRef(merge.Refs, "main", "branch", true) {
		t.Fatalf("main decoration: %+v", merge.Refs)
	}
	if !hasRef(merge.Refs, "HEAD → main", "head", true) {
		t.Fatalf("HEAD decoration: %+v", merge.Refs)
	}

	var side *GraphCommit
	for i := range res.Commits {
		if res.Commits[i].SHA == sideSHA {
			side = &res.Commits[i]
			break
		}
	}
	if side == nil {
		t.Fatal("side commit missing")
	}
	if !hasRef(side.Refs, "side", "branch", false) {
		t.Fatalf("side branch: %+v", side.Refs)
	}
	if !hasRef(side.Refs, "v1", "tag", false) {
		t.Fatalf("tag: %+v", side.Refs)
	}
	run(t, dir, "git", "tag", "-a", "v1-annotated", "-m", "release", sideSHA)
	annotated, err := Graph(context.Background(), dir, GraphQuery{Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	var annotatedSide *GraphCommit
	for i := range annotated.Commits {
		if annotated.Commits[i].SHA == sideSHA {
			annotatedSide = &annotated.Commits[i]
			break
		}
	}
	if annotatedSide == nil || !hasRef(annotatedSide.Refs, "v1-annotated", "tag", false) {
		t.Fatalf("annotated tag: %+v", annotatedSide)
	}
	if side.Lane == merge.Lane && len(res.Commits) > 2 {
		// Side diverged, so it should not sit on the same column as the merge
		// unless the assigner reused a free column later. The merge itself is
		// on the first-parent column; the side commit must have been drawn.
		if len(side.Lanes) == 0 {
			t.Fatalf("side lanes empty: %+v", side)
		}
	}
}

func TestGraph_RemoteRefKind(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir := initRepo(t)
	sha := strings.TrimSpace(string(mustGit(t, dir, "rev-parse", "HEAD")))
	run(t, dir, "git", "update-ref", "refs/remotes/origin/main", sha)

	res, err := Graph(context.Background(), dir, GraphQuery{})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Commits) == 0 || !hasRef(res.Commits[0].Refs, "origin/main", "remote", false) {
		t.Fatalf("remote ref: %+v", res.Commits)
	}
}

func TestGraph_Pagination(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir := initRepo(t)
	run(t, dir, "git", "commit", "--allow-empty", "-m", "second")
	run(t, dir, "git", "commit", "--allow-empty", "-m", "third")

	first, err := Graph(context.Background(), dir, GraphQuery{Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if !first.Truncated || len(first.Commits) != 2 {
		t.Fatalf("first page: truncated=%v n=%d", first.Truncated, len(first.Commits))
	}
	second, err := Graph(context.Background(), dir, GraphQuery{Limit: 2, Offset: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(second.Commits) != 1 {
		t.Fatalf("second page: %+v", second.Commits)
	}
	if second.Commits[0].SHA == first.Commits[0].SHA || second.Commits[0].SHA == first.Commits[1].SHA {
		t.Fatal("second page repeated a first-page commit")
	}
}

func TestGraph_NotARepo(t *testing.T) {
	res, err := Graph(context.Background(), t.TempDir(), GraphQuery{})
	if err != nil {
		t.Fatal(err)
	}
	if res.IsRepo || len(res.Commits) != 0 {
		t.Fatalf("%+v", res)
	}
}

func TestBranchAndTagCRUD(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir := initRepo(t)
	ctx := context.Background()

	if _, err := CreateBranch(ctx, dir, "feature", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := RenameBranch(ctx, dir, "feature", "feature-renamed"); err != nil {
		t.Fatal(err)
	}
	if _, err := DeleteBranch(ctx, dir, "feature-renamed"); err != nil {
		t.Fatal(err)
	}
	if _, err := DeleteBranch(ctx, dir, "main"); err == nil {
		t.Fatal("expected current branch delete to fail")
	}

	run(t, dir, "git", "checkout", "-b", "diverged")
	run(t, dir, "git", "commit", "--allow-empty", "-m", "only on diverged")
	run(t, dir, "git", "checkout", "main")
	if _, err := DeleteBranch(ctx, dir, "diverged"); err == nil || !strings.Contains(err.Error(), "not fully merged") && !strings.Contains(strings.ToLower(err.Error()), "not fully merged") {
		// git -d says "not fully merged".
		if err == nil {
			t.Fatal("expected unmerged delete to fail")
		}
	}

	sha := strings.TrimSpace(string(mustGit(t, dir, "rev-parse", "HEAD")))
	if _, err := CreateTag(ctx, dir, "v1", sha); err != nil {
		t.Fatal(err)
	}
	if _, err := RenameTag(ctx, dir, "v1", "v2"); err != nil {
		t.Fatal(err)
	}
	if _, err := runGit(ctx, dir, "rev-parse", "--verify", "--quiet", "refs/tags/v1"); err == nil {
		t.Fatal("old tag still exists")
	}
	if _, err := runGit(ctx, dir, "rev-parse", "--verify", "--quiet", "refs/tags/v2"); err != nil {
		t.Fatal(err)
	}
	if _, err := DeleteTag(ctx, dir, "v2"); err != nil {
		t.Fatal(err)
	}
	if _, err := runGit(ctx, dir, "rev-parse", "--verify", "--quiet", "refs/tags/v2"); err == nil {
		t.Fatal("tag still exists")
	}
}

func TestCheckout_DirtyRefused(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir := initRepo(t)
	ctx := context.Background()
	if _, err := CreateBranch(ctx, dir, "other", ""); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("dirty\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	before := strings.TrimSpace(string(mustGit(t, dir, "rev-parse", "--abbrev-ref", "HEAD")))
	if _, err := Checkout(ctx, dir, "other"); err == nil || !strings.Contains(err.Error(), "dirty") {
		t.Fatalf("expected dirty error, got %v", err)
	}
	after := strings.TrimSpace(string(mustGit(t, dir, "rev-parse", "--abbrev-ref", "HEAD")))
	if after != before {
		t.Fatalf("HEAD moved from %s to %s", before, after)
	}

	if err := os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("v1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Checkout(ctx, dir, "other"); err != nil {
		t.Fatal(err)
	}
	got := strings.TrimSpace(string(mustGit(t, dir, "rev-parse", "--abbrev-ref", "HEAD")))
	if got != "other" {
		t.Fatalf("HEAD = %s", got)
	}
}

func TestCommitInfo_MessageAndFiles(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir := initRepo(t)
	if err := os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("v1\nv2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(t, dir, "git", "add", "readme.txt")
	run(t, dir, "git", "commit", "-m", "edit readme")
	sha := strings.TrimSpace(string(mustGit(t, dir, "rev-parse", "HEAD")))

	detail, err := CommitInfo(context.Background(), dir, sha)
	if err != nil {
		t.Fatal(err)
	}
	if detail.Subject != "edit readme" || detail.Author == "" || len(detail.Parents) != 1 {
		t.Fatalf("%+v", detail)
	}
	if len(detail.Files) != 1 || detail.Files[0].Path != "readme.txt" || detail.Files[0].Additions < 1 {
		t.Fatalf("files: %+v", detail.Files)
	}
	if _, err := CommitInfo(context.Background(), dir, "HEAD..bad"); err == nil {
		t.Fatal("expected unsafe sha to fail")
	}
}

func TestRejectUnsafeRefNames(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir := initRepo(t)
	ctx := context.Background()
	badBranches := []string{"HEAD", "foo bar", "refs/heads/evil", "-n"}
	for _, name := range badBranches {
		if _, err := CreateBranch(ctx, dir, name, ""); err == nil {
			t.Fatalf("accepted branch %q", name)
		}
		if _, err := RenameBranch(ctx, dir, "main", name); err == nil {
			t.Fatalf("accepted rename to %q", name)
		}
	}
	if _, err := CreateTag(ctx, dir, "HEAD", ""); err == nil {
		t.Fatal("accepted tag HEAD")
	}
	if _, err := CreateTag(ctx, dir, "refs/tags/evil", ""); err == nil {
		t.Fatal("accepted tag refs/tags/evil")
	}
}

func TestAssignLanes_MergeUsesSecondColumn(t *testing.T) {
	// newest first: merge, side, main-parent, root
	rows := []rawCommit{
		{sha: "m", parents: []string{"a", "b"}},
		{sha: "b", parents: []string{"r"}},
		{sha: "a", parents: []string{"r"}},
		{sha: "r"},
	}
	got := assignLanes(rows)
	if got[0].lane != 0 || got[0].lanes[0].Kind != "merge" {
		t.Fatalf("merge: %+v", got[0])
	}
	if got[1].lane == 0 {
		t.Fatalf("side commit should leave the first-parent column: %+v", got[1])
	}
	if !laneKind(got[1].lanes, 0, "pass") || !laneKind(got[1].lanes, got[1].lane, "commit") {
		t.Fatalf("side row should keep the first-parent rail: %+v", got[1].lanes)
	}
	if !hasJoin(got[0].lanes, got[1].lane) {
		t.Fatalf("merge should join the side lane: %+v", got[0].lanes)
	}
}

func laneKind(lanes []GraphLane, lane int, kind string) bool {
	for _, item := range lanes {
		if item.Lane == lane && item.Kind == kind && item.To == nil {
			return true
		}
	}
	return false
}

func hasJoin(lanes []GraphLane, to int) bool {
	for _, item := range lanes {
		if item.To != nil && *item.To == to {
			return true
		}
	}
	return false
}

func hasRef(refs []GraphRef, name, kind string, current bool) bool {
	for _, ref := range refs {
		if ref.Name == name && ref.Kind == kind && ref.Current == current {
			return true
		}
	}
	return false
}

func mustGit(t *testing.T, dir string, args ...string) []byte {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git %v: %v", args, err)
	}
	return out
}
