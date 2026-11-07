package git

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	DefaultGraphLimit = 80
	MaxGraphLimit     = 200
	MaxGraphLanes     = 12
	graphFieldSep     = "\x1f"
	graphRecordSep    = "\x1e"
)

// GraphQuery selects a page of commits across all refs.
type GraphQuery struct {
	Limit  int
	Offset int
}

// GraphRef is a branch, tag, or remote-tracking ref pointing at a commit.
type GraphRef struct {
	Name    string `json:"name"`
	Kind    string `json:"kind"` // branch | remote | tag
	Current bool   `json:"current,omitempty"`
}

// GraphLane is one column cell in a commit row.
type GraphLane struct {
	Lane int    `json:"lane"`
	Kind string `json:"kind"` // commit | pass | merge
	// To is the column this node joins, for a merge's second parent.
	To *int `json:"to,omitempty"`
}

// GraphCommit is one row of the graph.
type GraphCommit struct {
	SHA        string      `json:"sha"`
	ShortSHA   string      `json:"short_sha"`
	Parents    []string    `json:"parents"`
	Subject    string      `json:"subject"`
	Author     string      `json:"author,omitempty"`
	AuthoredAt int64       `json:"authored_at,omitempty"`
	Lane       int         `json:"lane"`
	Lanes      []GraphLane `json:"lanes"`
	Refs       []GraphRef  `json:"refs"`
}

// GraphResult is a page of the commit graph plus HEAD identity.
type GraphResult struct {
	IsRepo     bool          `json:"is_repo"`
	HeadSHA    string        `json:"head_sha,omitempty"`
	HeadBranch string        `json:"head_branch,omitempty"`
	Commits    []GraphCommit `json:"commits"`
	Truncated  bool          `json:"truncated"`
	Message    string        `json:"message,omitempty"`
}

type rawCommit struct {
	sha     string
	parents []string
	author  string
	when    int64
	subject string
}

type laneAssigner struct {
	active []string // commit sha expected in each lane
}

// Graph returns a date-ordered commit graph for the workspace.
func Graph(ctx context.Context, workspace string, q GraphQuery) (GraphResult, error) {
	limit := q.Limit
	if limit <= 0 {
		limit = DefaultGraphLimit
	}
	if limit > MaxGraphLimit {
		limit = MaxGraphLimit
	}
	offset := q.Offset
	if offset < 0 {
		offset = 0
	}

	empty := GraphResult{Commits: []GraphCommit{}}
	if err := lookGit(); err != nil {
		empty.Message = "git is not installed or not on PATH"
		return empty, nil
	}
	if _, err := resolveGitRoot(ctx, workspace); err != nil {
		if errorsIsNotRepo(err) {
			empty.Message = "This workspace is not a git repository."
			return empty, nil
		}
		return GraphResult{}, err
	}

	headSHA, headBranch := headIdentity(ctx, workspace)
	refsBySHA, err := listRefs(ctx, workspace, headBranch)
	if err != nil {
		return GraphResult{}, err
	}
	markHead(refsBySHA, headSHA, headBranch)

	// One extra commit tells us whether another page exists.
	out, err := runGit(ctx, workspace,
		"log",
		"--date-order",
		"--pretty=format:%H%x1f%P%x1f%an%x1f%at%x1f%s%x1e",
		"-n", strconv.Itoa(limit+1),
		"--skip", strconv.Itoa(offset),
		"--all",
	)
	if err != nil {
		return GraphResult{}, err
	}
	raw := parseGraphLog(string(out))
	truncated := len(raw) > limit
	if truncated {
		raw = raw[:limit]
	}

	assigned := assignLanes(raw)
	commits := make([]GraphCommit, 0, len(assigned))
	for _, row := range assigned {
		commits = append(commits, GraphCommit{
			SHA:        row.sha,
			ShortSHA:   shortSHA(row.sha),
			Parents:    nonNil(row.parents),
			Subject:    row.subject,
			Author:     row.author,
			AuthoredAt: row.when,
			Lane:       row.lane,
			Lanes:      row.lanes,
			Refs:       refsOrEmpty(refsBySHA[row.sha]),
		})
	}
	return GraphResult{
		IsRepo:     true,
		HeadSHA:    headSHA,
		HeadBranch: headBranch,
		Commits:    commits,
		Truncated:  truncated,
	}, nil
}

func lookGit() error {
	if _, err := exec.LookPath("git"); err != nil {
		return fmt.Errorf("git is not installed or not on PATH")
	}
	return nil
}

func headIdentity(ctx context.Context, workspace string) (sha, branch string) {
	if out, err := runGit(ctx, workspace, "rev-parse", "HEAD"); err == nil {
		sha = strings.TrimSpace(string(out))
	}
	if out, err := runGit(ctx, workspace, "rev-parse", "--abbrev-ref", "HEAD"); err == nil {
		name := strings.TrimSpace(string(out))
		if name != "" && name != "HEAD" {
			branch = name
		}
	}
	return sha, branch
}

func listRefs(ctx context.Context, workspace, headBranch string) (map[string][]GraphRef, error) {
	out, err := runGit(ctx, workspace,
		"for-each-ref",
		"--format=%(objectname)%00%(*objectname)%00%(refname)",
		"refs/heads",
		"refs/tags",
		"refs/remotes",
	)
	if err != nil {
		return nil, err
	}
	bySHA := map[string][]GraphRef{}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\x00", 3)
		if len(parts) != 3 || parts[2] == "" {
			continue
		}
		// Annotated tags report the tag object in objectname. *objectname is the
		// commit that object points at, which is the row the chip belongs on.
		sha, name := parts[0], parts[2]
		if parts[1] != "" {
			sha = parts[1]
		}
		if sha == "" {
			continue
		}
		ref, ok := classifyRef(name, headBranch)
		if !ok {
			continue
		}
		bySHA[sha] = append(bySHA[sha], ref)
	}
	return bySHA, nil
}

func markHead(bySHA map[string][]GraphRef, headSHA, headBranch string) {
	if headSHA == "" {
		return
	}
	refs := bySHA[headSHA]
	for i := range refs {
		if headBranch != "" && refs[i].Kind == "branch" && refs[i].Name == headBranch {
			refs[i].Current = true
		}
	}
	label := "HEAD"
	if headBranch != "" {
		label = "HEAD → " + headBranch
	}
	bySHA[headSHA] = append([]GraphRef{{Name: label, Kind: "head", Current: true}}, refs...)
}

func classifyRef(full, headBranch string) (GraphRef, bool) {
	switch {
	case strings.HasPrefix(full, "refs/heads/"):
		name := strings.TrimPrefix(full, "refs/heads/")
		if name == "" || strings.Contains(name, "/") && name == "HEAD" {
			return GraphRef{}, false
		}
		return GraphRef{Name: name, Kind: "branch", Current: name == headBranch}, true
	case strings.HasPrefix(full, "refs/tags/"):
		name := strings.TrimPrefix(full, "refs/tags/")
		if name == "" {
			return GraphRef{}, false
		}
		return GraphRef{Name: name, Kind: "tag"}, true
	case strings.HasPrefix(full, "refs/remotes/"):
		name := strings.TrimPrefix(full, "refs/remotes/")
		if name == "" || strings.HasSuffix(name, "/HEAD") {
			return GraphRef{}, false
		}
		return GraphRef{Name: name, Kind: "remote"}, true
	default:
		return GraphRef{}, false
	}
}

func parseGraphLog(raw string) []rawCommit {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, graphRecordSep)
	out := make([]rawCommit, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		fields := strings.Split(part, graphFieldSep)
		if len(fields) < 5 {
			continue
		}
		var parents []string
		if fields[1] != "" {
			parents = strings.Fields(fields[1])
		}
		when, _ := strconv.ParseInt(strings.TrimSpace(fields[3]), 10, 64)
		out = append(out, rawCommit{
			sha:     fields[0],
			parents: parents,
			author:  fields[2],
			when:    when,
			subject: fields[4],
		})
	}
	return out
}

type assignedCommit struct {
	rawCommit
	lane  int
	lanes []GraphLane
}

// assignLanes places each commit on the column of the first active parent
// slot that was waiting for it, then reserves columns for extra parents.
func assignLanes(commits []rawCommit) []assignedCommit {
	a := &laneAssigner{}
	out := make([]assignedCommit, 0, len(commits))
	for _, c := range commits {
		lane := a.take(c.sha)
		if lane < 0 {
			lane = a.reserve(c.sha)
		}
		if lane >= MaxGraphLanes {
			lane = MaxGraphLanes - 1
		}
		// Drop the consumed slot before inserting parents so the first parent
		// can reuse this column.
		if lane < len(a.active) {
			a.active[lane] = ""
		}
		for i, parent := range c.parents {
			if i == 0 {
				a.place(lane, parent)
				continue
			}
			if existing := indexOf(a.active, parent); existing >= 0 {
				continue
			}
			a.reserve(parent)
		}
		lanes := a.snapshot(lane, len(c.parents) > 1)
		if len(c.parents) > 1 {
			if parentLane := indexOf(a.active, c.parents[1]); parentLane >= 0 && parentLane != lane {
				to := parentLane
				lanes = append(lanes, GraphLane{Lane: lane, Kind: "merge", To: &to})
			}
		}
		out = append(out, assignedCommit{rawCommit: c, lane: lane, lanes: lanes})
		a.compact()
	}
	return out
}

func (a *laneAssigner) take(sha string) int {
	return indexOf(a.active, sha)
}

func (a *laneAssigner) reserve(sha string) int {
	for i, cur := range a.active {
		if cur == "" {
			a.active[i] = sha
			return i
		}
	}
	a.active = append(a.active, sha)
	return len(a.active) - 1
}

func (a *laneAssigner) place(lane int, sha string) {
	for len(a.active) <= lane {
		a.active = append(a.active, "")
	}
	if a.active[lane] == "" || a.active[lane] == sha {
		a.active[lane] = sha
		return
	}
	a.reserve(sha)
}

func (a *laneAssigner) snapshot(commitLane int, merge bool) []GraphLane {
	lanes := make([]GraphLane, 0, len(a.active))
	// Keep every column through the rightmost active lane. Dropping an empty
	// middle column makes the drawn rail break between commits.
	last := len(a.active) - 1
	for last > commitLane && last >= 0 && a.active[last] == "" {
		last--
	}
	for i := 0; i <= last; i++ {
		sha := ""
		if i < len(a.active) {
			sha = a.active[i]
		}
		if sha == "" && i != commitLane {
			lanes = append(lanes, GraphLane{Lane: i, Kind: "pass"})
			continue
		}
		kind := "pass"
		if i == commitLane {
			if merge {
				kind = "merge"
			} else {
				kind = "commit"
			}
		}
		lanes = append(lanes, GraphLane{Lane: i, Kind: kind})
	}
	return lanes
}

func (a *laneAssigner) compact() {
	for len(a.active) > 0 && a.active[len(a.active)-1] == "" {
		a.active = a.active[:len(a.active)-1]
	}
}

func indexOf(items []string, want string) int {
	for i, item := range items {
		if item == want {
			return i
		}
	}
	return -1
}

func shortSHA(sha string) string {
	if len(sha) <= 7 {
		return sha
	}
	return sha[:7]
}

func nonNil(items []string) []string {
	if items == nil {
		return []string{}
	}
	return items
}

func refsOrEmpty(refs []GraphRef) []GraphRef {
	if refs == nil {
		return []GraphRef{}
	}
	return refs
}

// CommitDetail is the expanded card for one commit.
type CommitDetail struct {
	SHA         string           `json:"sha"`
	ShortSHA    string           `json:"short_sha"`
	Parents     []string         `json:"parents"`
	Subject     string           `json:"subject"`
	Body        string           `json:"body,omitempty"`
	Author      string           `json:"author,omitempty"`
	AuthorEmail string           `json:"author_email,omitempty"`
	AuthoredAt  int64            `json:"authored_at,omitempty"`
	Files       []CommitFileStat `json:"files"`
}

// CommitFileStat is one path changed by a commit.
type CommitFileStat struct {
	Path      string `json:"path"`
	Status    string `json:"status"`
	Additions int    `json:"additions"`
	Deletions int    `json:"deletions"`
}

// CommitInfo loads the message, author, and file stats for one commit.
func CommitInfo(ctx context.Context, workspace, sha string) (CommitDetail, error) {
	sha = strings.TrimSpace(sha)
	if !isSafeRevision(sha) {
		return CommitDetail{}, fmt.Errorf("invalid commit: %q", sha)
	}
	if _, err := resolveGitRoot(ctx, workspace); err != nil {
		if errorsIsNotRepo(err) {
			return CommitDetail{}, fmt.Errorf("not a git repository")
		}
		return CommitDetail{}, err
	}
	meta, err := runGit(ctx, workspace, "log", "-1", "--pretty=format:%H%x1f%P%x1f%an%x1f%ae%x1f%at%x1f%s%x1f%b%x1e", sha)
	if err != nil {
		return CommitDetail{}, err
	}
	detail, err := parseCommitMeta(string(meta))
	if err != nil {
		return CommitDetail{}, err
	}
	numstat, err := runGit(ctx, workspace, "show", "--numstat", "--format=", "-m", "--first-parent", sha)
	if err != nil {
		return CommitDetail{}, err
	}
	detail.Files = parseNumstat(string(numstat))
	return detail, nil
}

func parseCommitMeta(raw string) (CommitDetail, error) {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimSuffix(raw, graphRecordSep)
	fields := strings.Split(raw, graphFieldSep)
	if len(fields) < 6 || fields[0] == "" {
		return CommitDetail{}, fmt.Errorf("invalid commit")
	}
	var parents []string
	if fields[1] != "" {
		parents = strings.Fields(fields[1])
	}
	when, _ := strconv.ParseInt(strings.TrimSpace(fields[4]), 10, 64)
	body := ""
	if len(fields) > 6 {
		body = strings.TrimSpace(fields[6])
	}
	return CommitDetail{
		SHA:         fields[0],
		ShortSHA:    shortSHA(fields[0]),
		Parents:     nonNil(parents),
		Subject:     fields[5],
		Body:        body,
		Author:      fields[2],
		AuthorEmail: fields[3],
		AuthoredAt:  when,
		Files:       []CommitFileStat{},
	}, nil
}

// CommitPathDiff returns the unified diff for one path at a commit.
func CommitPathDiff(ctx context.Context, workspace, sha, relPath string) (DiffResult, error) {
	sha = strings.TrimSpace(sha)
	relPath = filepath.ToSlash(filepath.Clean(relPath))
	if !isSafeRevision(sha) {
		return DiffResult{}, fmt.Errorf("invalid commit: %q", sha)
	}
	if relPath == "." || strings.HasPrefix(relPath, "../") || filepath.IsAbs(relPath) {
		return DiffResult{}, fmt.Errorf("invalid path")
	}
	if _, err := resolveGitRoot(ctx, workspace); err != nil {
		if errorsIsNotRepo(err) {
			return DiffResult{Path: relPath, Message: "This workspace is not a git repository."}, nil
		}
		return DiffResult{}, err
	}
	out, err := runGit(ctx, workspace, "show", "--format=", sha, "--", relPath)
	if err != nil {
		return DiffResult{}, err
	}
	body := strings.TrimSpace(string(out))
	if body == "" {
		return DiffResult{Path: relPath, Empty: true, Message: "No diff for this path in that commit."}, nil
	}
	return DiffResult{Path: relPath, Diff: body}, nil
}

func parseNumstat(raw string) []CommitFileStat {
	out := make([]CommitFileStat, 0)
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 3)
		if len(parts) < 3 || parts[2] == "" {
			continue
		}
		add, _ := strconv.Atoi(parts[0])
		del, _ := strconv.Atoi(parts[1])
		path := parts[2]
		status := "modified"
		if strings.Contains(path, " => ") {
			status = "renamed"
		}
		if parts[0] == "-" && parts[1] == "-" {
			status = "binary"
		}
		out = append(out, CommitFileStat{
			Path:      path,
			Status:    status,
			Additions: add,
			Deletions: del,
		})
	}
	return out
}
