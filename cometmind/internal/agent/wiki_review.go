package agent

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/Cometline/cometline/cometmind/internal/event"
	"github.com/Cometline/cometline/cometmind/internal/logging"
	"github.com/Cometline/cometline/cometmind/internal/session"
	"github.com/Cometline/cometline/cometmind/internal/skills"
	"github.com/Cometline/cometline/cometmind/internal/tools"
	"go.uber.org/zap"
)

type wikiReviewInput struct {
	UserChat bool
	HasModel bool
	Turns    int
}

type wikiReviewDecision struct {
	Start   bool
	LogSkip string
}

func decideWikiReview(in wikiReviewInput) wikiReviewDecision {
	var decision wikiReviewDecision
	if !in.UserChat || in.Turns < reviewTurnThreshold {
		return decision
	}
	if !in.HasModel {
		decision.LogSkip = "wiki.review.skipped_no_model"
		return decision
	}
	decision.Start = true
	return decision
}

func (r *Runner) reviewAfterTurn(ctx context.Context, turn session.AgentTurn) {
	if r == nil || strings.TrimSpace(turn.ID) == "" {
		return
	}
	store, ok := r.Sessions.(skillReviewStore)
	if !ok {
		return
	}
	sess, err := store.GetSession(ctx, turn.ID)
	if err != nil {
		logging.L().Warn("review.session_failed", zap.String("session", turn.ID), zap.Error(err))
		return
	}
	if sess.ParentSessionID != "" || sess.Origin != "user" {
		return
	}
	rows, err := store.ListMessageRows(ctx, turn.ID)
	if err != nil {
		logging.L().Warn("review.messages_failed", zap.String("session", turn.ID), zap.Error(err))
		return
	}
	calls, err := store.ListToolCallsForSession(ctx, turn.ID)
	if err != nil {
		logging.L().Warn("review.tools_failed", zap.String("session", turn.ID), zap.Error(err))
		return
	}
	turns, err := store.AddSkillReviewMutatingCount(ctx, turn.ID, 1)
	if err != nil {
		logging.L().Warn("review.count_failed", zap.String("session", turn.ID), zap.Error(err))
		turns = sess.SkillReviewMutatingCount + 1
	}
	_, _, hasModel := reviewExtraction(r.Config)
	skillDecision := decideSkillReview(skillReviewInput{
		UserChat: true,
		HasModel: hasModel,
		Turns:    int(turns),
	})
	wikiDecision := decideWikiReview(wikiReviewInput{
		UserChat: true,
		HasModel: hasModel,
		Turns:    int(turns),
	})
	if err := store.SetSkillReviewLastTargets(ctx, turn.ID, encodeTargets(skillDecision.Targets)); err != nil {
		logging.L().Warn("skills.review.targets_failed", zap.String("session", turn.ID), zap.Error(err))
	}
	if skillDecision.LogSkip != "" {
		logging.L().Warn(skillDecision.LogSkip, zap.String("session", turn.ID))
	}
	if wikiDecision.LogSkip != "" {
		logging.L().Warn(wikiDecision.LogSkip, zap.String("session", turn.ID))
	}
	if !skillDecision.Start && !wikiDecision.Start {
		return
	}
	workspace, err := store.WorkspacePath(ctx, sess.WorkspaceID)
	if err != nil {
		logging.L().Warn("review.workspace_failed", zap.String("session", turn.ID), zap.Error(err))
		return
	}
	cfg := skills.Config{Enabled: true}
	if r.Config != nil {
		cfg = r.Config.SkillSettings()
	}
	catalog := skills.Discover(workspace, cfg)
	transcript := formatTurnTranscript(rows, calls)
	if skillDecision.WideWindow {
		transcript = formatTranscriptSince(rows, calls, sess.SkillReviewCountResetAt)
	}
	r.enqueueTurnReview(ctx, store, sess, workspace, transcript, catalog, skillDecision.Start, wikiDecision.Start)
}

type turnReviewJob struct {
	ctx          context.Context
	store        skillReviewStore
	parent       session.Session
	workspace    string
	transcript   string
	catalog      skills.Registry
	reviewSkills bool
	reviewWiki   bool
}

func (r *Runner) enqueueTurnReview(ctx context.Context, store skillReviewStore, parent session.Session, workspace, transcript string, catalog skills.Registry, reviewSkills, reviewWiki bool) {
	job := turnReviewJob{ctx, store, parent, workspace, transcript, catalog, reviewSkills, reviewWiki}
	r.reviewMu.Lock()
	if r.reviewRunning {
		r.reviewPending = &job
		r.reviewMu.Unlock()
		return
	}
	r.reviewRunning = true
	r.reviewMu.Unlock()
	r.drainTurnReviews(job)
}

func (r *Runner) drainTurnReviews(job turnReviewJob) {
	for {
		r.startTurnReview(job.ctx, job.store, job.parent, job.workspace, job.transcript, job.catalog, job.reviewSkills, job.reviewWiki)
		r.reviewMu.Lock()
		next := r.reviewPending
		r.reviewPending = nil
		if next == nil {
			r.reviewRunning = false
			r.reviewMu.Unlock()
			return
		}
		r.reviewMu.Unlock()
		job = *next
	}
}

func (r *Runner) startTurnReview(ctx context.Context, store skillReviewStore, parent session.Session, workspace, transcript string, catalog skills.Registry, reviewSkills, reviewWiki bool) {
	if r.ReviewChild == nil {
		logging.L().Warn("review.skipped_no_runner", zap.String("session", parent.ID))
		return
	}
	providerID, modelID, ok := reviewExtraction(r.Config)
	if !ok {
		return
	}
	child, err := store.NewChildSession(ctx, parent, "turn review", reviewKind)
	if err != nil {
		logging.L().Warn("review.child_failed", zap.String("session", parent.ID), zap.Error(err))
		return
	}
	updated, err := store.UpdateSessionModel(ctx, child.ID, modelID, providerID)
	if err != nil {
		logging.L().Warn("review.model_failed", zap.String("session", parent.ID), zap.Error(err))
		_ = store.DeleteSession(ctx, child.ID)
		return
	}
	child = updated
	if _, err := store.AppendUserMessage(ctx, child.ID, skillReviewUserPrompt(transcript, catalog)); err != nil {
		logging.L().Warn("review.prompt_failed", zap.String("session", parent.ID), zap.Error(err))
		_ = store.DeleteSession(ctx, child.ID)
		return
	}
	registry := tools.NewTurnReviewRegistry(workspace, &catalog, r.SkillUsed)
	started := r.reviewNow()
	if reviewSkills {
		if err := store.ResetSkillReviewAfterStart(ctx, parent.ID, started.UnixMilli()); err != nil {
			logging.L().Warn("skills.review.reset_failed", zap.String("session", parent.ID), zap.Error(err))
			_ = store.DeleteSession(ctx, child.ID)
			return
		}
	}
	if reviewWiki {
		if err := store.SetWikiReviewStartedAt(ctx, parent.ID, started.UnixMilli()); err != nil {
			logging.L().Warn("wiki.review.mark_failed", zap.String("session", parent.ID), zap.Error(err))
		}
	}
	runErr := r.ReviewChild(ctx, child, registry, reviewMaxSteps, turnReviewSystemPrompt(reviewSkills, reviewWiki))
	if runErr != nil {
		logging.L().Warn("review.child_run_failed", zap.String("session", parent.ID), zap.String("child", child.ID), zap.Error(runErr))
	}
	if reviewSkills {
		written := collectSkillWrites(ctx, store, child.ID)
		if len(written) > 0 && r.Events != nil {
			r.Events.Publish(event.SkillReviewUpdated(parent.ID, written))
		}
	}
	if reviewWiki && r.Events != nil {
		if paths := collectWikiPaths(ctx, store, child.ID); len(paths) > 0 {
			r.Events.Publish(event.WikiReviewUpdated(parent.ID, paths))
		}
	}
	if err := store.DeleteSession(ctx, child.ID); err != nil {
		logging.L().Warn("review.delete_failed", zap.String("session", parent.ID), zap.String("child", child.ID), zap.Error(err))
	}
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
