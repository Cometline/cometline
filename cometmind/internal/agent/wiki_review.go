package agent

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/Cometline/cometline/cometmind/internal/event"
	"github.com/Cometline/cometline/cometmind/internal/logging"
	"github.com/Cometline/cometline/cometmind/internal/session"
	"github.com/Cometline/cometline/cometmind/internal/tools"
	"go.uber.org/zap"
)

const (
	wikiReviewCooldown = 15 * time.Minute
	wikiReviewMaxSteps = 12
	wikiReviewKind     = "wiki_review"
)

type wikiReviewInput struct {
	UserChat    bool
	HasModel    bool
	Now         time.Time
	LastStarted time.Time
	Calls       []skillCall
}

type wikiReviewDecision struct {
	Start   bool
	LogSkip string
}

func decideWikiReview(in wikiReviewInput) wikiReviewDecision {
	var decision wikiReviewDecision
	if !in.UserChat || !wikiReviewEvidence(in.Calls) || wikiAlreadyWritten(in.Calls) {
		return decision
	}
	if cooldownActive(in.Now, in.LastStarted) {
		return decision
	}
	if !in.HasModel {
		decision.LogSkip = "wiki.review.skipped_no_model"
		return decision
	}
	decision.Start = true
	return decision
}

func wikiReviewEvidence(calls []skillCall) bool {
	for _, call := range calls {
		if call.OK {
			return true
		}
	}
	return false
}

func wikiWriteTool(name string) bool {
	switch name {
	case "write_file", "edit_file":
		return true
	default:
		return false
	}
}

func wikiAlreadyWritten(calls []skillCall) bool {
	for _, call := range calls {
		if !call.OK || !wikiWriteTool(call.Name) {
			continue
		}
		if strings.Contains(call.Path, "@runtime/wiki/") || strings.Contains(call.Path, "/.cometmind/wiki/") {
			return true
		}
	}
	return false
}

func (r *Runner) reviewWikiAfterTurn(ctx context.Context, turn session.AgentTurn) {
	if r == nil || strings.TrimSpace(turn.ID) == "" {
		return
	}
	store, ok := r.Sessions.(skillReviewStore)
	if !ok {
		return
	}
	sess, err := store.GetSession(ctx, turn.ID)
	if err != nil {
		logging.L().Warn("wiki.review.session_failed", zap.String("session", turn.ID), zap.Error(err))
		return
	}
	if sess.ParentSessionID != "" || sess.Origin != "user" {
		return
	}
	rows, err := store.ListMessageRows(ctx, turn.ID)
	if err != nil {
		logging.L().Warn("wiki.review.messages_failed", zap.String("session", turn.ID), zap.Error(err))
		return
	}
	calls, err := store.ListToolCallsForSession(ctx, turn.ID)
	if err != nil {
		logging.L().Warn("wiki.review.tools_failed", zap.String("session", turn.ID), zap.Error(err))
		return
	}
	_, turnCalls := turnMutations(rows, calls)
	_, _, hasModel := pinnedExtraction(r.Config)
	decision := decideWikiReview(wikiReviewInput{
		UserChat:    true,
		HasModel:    hasModel,
		Now:         r.reviewNow(),
		LastStarted: unixMilliTime(sess.WikiReviewStartedAt),
		Calls:       turnCalls,
	})
	if decision.LogSkip != "" {
		logging.L().Warn(decision.LogSkip, zap.String("session", turn.ID))
	}
	if !decision.Start {
		return
	}
	providerID, modelID, _ := pinnedExtraction(r.Config)
	workspace, err := store.WorkspacePath(ctx, sess.WorkspaceID)
	if err != nil {
		logging.L().Warn("wiki.review.workspace_failed", zap.String("session", turn.ID), zap.Error(err))
		return
	}
	r.startWikiReview(ctx, store, sess, providerID, modelID, workspace, formatTurnTranscript(rows, calls))
}

func (r *Runner) startWikiReview(ctx context.Context, store skillReviewStore, parent session.Session, providerID, modelID, workspace, transcript string) {
	if r.WikiChild == nil {
		logging.L().Warn("wiki.review.skipped_no_runner", zap.String("session", parent.ID))
		return
	}
	child, err := store.NewChildSession(ctx, parent, "wiki review", wikiReviewKind)
	if err != nil {
		logging.L().Warn("wiki.review.child_failed", zap.String("session", parent.ID), zap.Error(err))
		return
	}
	updated, err := store.UpdateSessionModel(ctx, child.ID, modelID, providerID)
	if err != nil {
		logging.L().Warn("wiki.review.model_failed", zap.String("session", parent.ID), zap.Error(err))
		_ = store.DeleteSession(ctx, child.ID)
		return
	}
	child = updated
	if _, err := store.AppendUserMessage(ctx, child.ID, wikiReviewUserPrompt(transcript)); err != nil {
		logging.L().Warn("wiki.review.prompt_failed", zap.String("session", parent.ID), zap.Error(err))
		_ = store.DeleteSession(ctx, child.ID)
		return
	}
	registry := tools.NewWikiReviewRegistry(workspace)
	started := r.reviewNow()
	if err := store.SetWikiReviewStartedAt(ctx, parent.ID, started.UnixMilli()); err != nil {
		logging.L().Warn("wiki.review.cooldown_failed", zap.String("session", parent.ID), zap.Error(err))
		_ = store.DeleteSession(ctx, child.ID)
		return
	}
	if runErr := r.WikiChild(ctx, child, registry, wikiReviewMaxSteps, wikiReviewSystemPrompt()); runErr != nil {
		logging.L().Warn("wiki.review.child_run_failed", zap.String("session", parent.ID), zap.String("child", child.ID), zap.Error(runErr))
	}
	paths := collectWikiPaths(ctx, store, child.ID)
	if len(paths) > 0 && r.Events != nil {
		r.Events.Publish(event.WikiReviewUpdated(parent.ID, paths))
	}
	if err := store.DeleteSession(ctx, child.ID); err != nil {
		logging.L().Warn("wiki.review.delete_failed", zap.String("session", parent.ID), zap.String("child", child.ID), zap.Error(err))
	}
}

func wikiReviewSystemPrompt() string {
	return strings.TrimSpace(`
You compile reusable knowledge from this turn into the user's LLM wiki at @runtime/wiki/.
Save a conclusion only when it is useful again and grounded in a tool result from this turn.
Route other takeaways elsewhere:
- Preferences and personal facts belong to memory. Do not write them here.
- Repeated procedures belong to skills. Do not write them here.
- One-off bug fixes, commands, and transient task logs are not wiki pages.
Update an existing page when the conclusion belongs there. If the page is a dated or commit-stamped snapshot and this turn has a newer source, update it; do not leave a stale current claim.
If no existing page fits and the conclusion is narrow, stop without writing. Do not create a page for a single fix.
Follow llm-wiki capture-then-compile when a source is worth keeping: new raw file, update existing entity/concept/synthesis pages, update index.md, append log.md.
Never edit an existing file under raw/.
Read @runtime/wiki/index.md before writing.
`)
}

func wikiReviewUserPrompt(transcript string) string {
	var b strings.Builder
	b.WriteString("Parent turn transcript:\n\n")
	if strings.TrimSpace(transcript) == "" {
		b.WriteString("(empty)\n")
	} else {
		b.WriteString(transcript)
	}
	b.WriteString("\nRead @runtime/wiki/index.md before writing. Write only under @runtime/wiki/.\n")
	return b.String()
}

func collectWikiPaths(ctx context.Context, store skillReviewStore, childID string) []string {
	calls, err := store.ListToolCallsForSession(ctx, childID)
	if err != nil {
		return nil
	}
	rows, err := store.ListMessageRows(ctx, childID)
	if err != nil {
		return nil
	}
	seen, failed := toolResultFlags(rows)
	var out []string
	used := map[string]bool{}
	for _, call := range calls {
		if (call.ToolName != "edit_file" && call.ToolName != "write_file") || !seen[call.ID] || failed[call.ID] {
			continue
		}
		var args struct {
			Path string `json:"path"`
		}
		if err := json.Unmarshal([]byte(call.Arguments), &args); err != nil {
			continue
		}
		path := wikiDisplayPath(args.Path)
		if path == "" || used[path] {
			continue
		}
		used[path] = true
		out = append(out, path)
	}
	return out
}

func wikiDisplayPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	slash := strings.ReplaceAll(path, "\\", "/")
	if strings.HasPrefix(slash, "@runtime/wiki/") || slash == "@runtime/wiki" {
		return slash
	}
	slash = strings.TrimPrefix(slash, "/")
	return "@runtime/wiki/" + slash
}
