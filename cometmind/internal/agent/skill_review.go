package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Cometline/cometline/cometmind/internal/config"
	"github.com/Cometline/cometline/cometmind/internal/db"
	"github.com/Cometline/cometline/cometmind/internal/event"
	"github.com/Cometline/cometline/cometmind/internal/logging"
	"github.com/Cometline/cometline/cometmind/internal/session"
	"github.com/Cometline/cometline/cometmind/internal/skills"
	"github.com/Cometline/cometline/cometmind/internal/tools"
	"go.uber.org/zap"
)

// ReviewChild runs one hidden skill-review child. The parent launch deletes
// the child after this returns.
type ReviewChild func(ctx context.Context, child session.Session, registry *tools.Registry, maxSteps int, systemPrompt string) error

type skillReviewStore interface {
	GetSession(ctx context.Context, sessionID string) (session.Session, error)
	ListMessageRows(ctx context.Context, sessionID string) ([]db.Message, error)
	ListToolCallsForSession(ctx context.Context, sessionID string) ([]db.ToolCall, error)
	NewChildSession(ctx context.Context, parent session.Session, purpose, subagentKind string) (session.Session, error)
	UpdateSessionModel(ctx context.Context, sessionID, modelID, providerID string) (session.Session, error)
	DeleteSession(ctx context.Context, sessionID string) error
	AppendUserMessage(ctx context.Context, sessionID, text string) (session.Message, error)
	WorkspacePath(ctx context.Context, workspaceID string) (string, error)
	SetSkillReviewLastTargets(ctx context.Context, sessionID, targets string) error
	AddSkillReviewMutatingCount(ctx context.Context, sessionID string, n int64) (int64, error)
	ResetSkillReviewAfterStart(ctx context.Context, sessionID string, at int64) error
	SetWikiReviewStartedAt(ctx context.Context, sessionID string, at int64) error
}

func (r *Runner) reviewNow() time.Time {
	if r != nil && r.ReviewNow != nil {
		return r.ReviewNow()
	}
	return time.Now()
}

func (r *Runner) reviewSkillsAfterTurn(ctx context.Context, turn session.AgentTurn) {
	if r == nil || strings.TrimSpace(turn.ID) == "" {
		return
	}
	store, ok := r.Sessions.(skillReviewStore)
	if !ok {
		return
	}
	sess, err := store.GetSession(ctx, turn.ID)
	if err != nil {
		logging.L().Warn("skills.review.session_failed", zap.String("session", turn.ID), zap.Error(err))
		return
	}
	if sess.ParentSessionID != "" || sess.Origin != "user" {
		return
	}
	rows, err := store.ListMessageRows(ctx, turn.ID)
	if err != nil {
		logging.L().Warn("skills.review.messages_failed", zap.String("session", turn.ID), zap.Error(err))
		return
	}
	calls, err := store.ListToolCallsForSession(ctx, turn.ID)
	if err != nil {
		logging.L().Warn("skills.review.tools_failed", zap.String("session", turn.ID), zap.Error(err))
		return
	}
	userText, turnCalls := turnMutations(rows, calls)
	accumulated := int(sess.SkillReviewMutatingCount) + countMutating(turnCalls)
	if added := countMutating(turnCalls); added > 0 {
		total, err := store.AddSkillReviewMutatingCount(ctx, turn.ID, int64(added))
		if err != nil {
			logging.L().Warn("skills.review.count_failed", zap.String("session", turn.ID), zap.Error(err))
		} else {
			accumulated = int(total)
		}
	}
	providerID, modelID, hasModel := pinnedExtraction(r.Config)
	decision := decideSkillReview(skillReviewInput{
		UserChat:    true,
		HasModel:    hasModel,
		Now:         r.reviewNow(),
		LastStarted: unixMilliTime(sess.SkillReviewStartedAt),
		UserText:    userText,
		Previous:    decodeTargets(sess.SkillReviewLastTargets),
		Calls:       turnCalls,
		Accumulated: accumulated,
	})
	if err := store.SetSkillReviewLastTargets(ctx, turn.ID, encodeTargets(decision.Targets)); err != nil {
		logging.L().Warn("skills.review.targets_failed", zap.String("session", turn.ID), zap.Error(err))
	}
	if decision.LogSkip != "" {
		logging.L().Warn(decision.LogSkip, zap.String("session", turn.ID))
	}
	if !decision.Start {
		return
	}
	workspace, err := store.WorkspacePath(ctx, sess.WorkspaceID)
	if err != nil {
		logging.L().Warn("skills.review.workspace_failed", zap.String("session", turn.ID), zap.Error(err))
		return
	}
	cfg := skills.Config{Enabled: true}
	if r.Config != nil {
		cfg = r.Config.SkillSettings()
	}
	catalog := skills.Discover(workspace, cfg)
	transcript := formatTurnTranscript(rows, calls)
	if decision.WideWindow {
		transcript = formatTranscriptSince(rows, calls, sess.SkillReviewCountResetAt)
	}
	r.startSkillReview(ctx, store, sess, providerID, modelID, workspace, transcript, catalog)
}

func (r *Runner) startSkillReview(ctx context.Context, store skillReviewStore, parent session.Session, providerID, modelID, workspace, transcript string, catalog skills.Registry) {
	if r.ReviewChild == nil {
		logging.L().Warn("skills.review.skipped_no_runner", zap.String("session", parent.ID))
		return
	}
	child, err := store.NewChildSession(ctx, parent, "skill review", skillReviewKind)
	if err != nil {
		logging.L().Warn("skills.review.child_failed", zap.String("session", parent.ID), zap.Error(err))
		return
	}
	updated, err := store.UpdateSessionModel(ctx, child.ID, modelID, providerID)
	if err != nil {
		logging.L().Warn("skills.review.model_failed", zap.String("session", parent.ID), zap.Error(err))
		_ = store.DeleteSession(ctx, child.ID)
		return
	}
	child = updated
	if _, err := store.AppendUserMessage(ctx, child.ID, skillReviewUserPrompt(transcript, catalog)); err != nil {
		logging.L().Warn("skills.review.prompt_failed", zap.String("session", parent.ID), zap.Error(err))
		_ = store.DeleteSession(ctx, child.ID)
		return
	}
	registry := tools.NewSkillReviewRegistry(workspace, &catalog, r.SkillUsed)
	started := r.reviewNow()
	if err := store.ResetSkillReviewAfterStart(ctx, parent.ID, started.UnixMilli()); err != nil {
		logging.L().Warn("skills.review.cooldown_failed", zap.String("session", parent.ID), zap.Error(err))
		_ = store.DeleteSession(ctx, child.ID)
		return
	}
	runErr := r.ReviewChild(ctx, child, registry, skillReviewMaxSteps, skillReviewSystemPrompt())
	if runErr != nil {
		logging.L().Warn("skills.review.child_run_failed", zap.String("session", parent.ID), zap.String("child", child.ID), zap.Error(runErr))
	}
	written := collectSkillWrites(ctx, store, child.ID)
	if len(written) > 0 && r.Events != nil {
		r.Events.Publish(event.SkillReviewUpdated(parent.ID, written))
	}
	if err := store.DeleteSession(ctx, child.ID); err != nil {
		logging.L().Warn("skills.review.delete_failed", zap.String("session", parent.ID), zap.String("child", child.ID), zap.Error(err))
	}
}

func skillReviewSystemPrompt() string {
	return strings.TrimSpace(`
You review one completed user turn and may save a reusable workflow as a live Agent Skill.
Save only a workflow that was demonstrated and verified in the parent transcript.
Skip one-off fixes, guesses, and procedures that failed.
Create a new skill only when no existing self-improvement skill covers it.
If an existing self-improvement skill overlaps, overwrite that skill with overwrite=true and preserve its origin.
Never try to edit a skill whose origin is not self-improvement.
If nothing is worth saving, stop without calling write_skill.
`)
}

func skillReviewUserPrompt(transcript string, catalog skills.Registry) string {
	var b strings.Builder
	b.WriteString("Parent turn transcript:\n\n")
	if strings.TrimSpace(transcript) == "" {
		b.WriteString("(empty)\n")
	} else {
		b.WriteString(transcript)
		if !strings.HasSuffix(transcript, "\n") {
			b.WriteByte('\n')
		}
	}
	b.WriteString("\nCurrent skills:\n")
	listed := 0
	for _, skill := range catalog.Skills {
		if skill.Internal {
			continue
		}
		origin := skill.Origin
		if origin == "" {
			origin = "other"
		}
		fmt.Fprintf(&b, "- %s (%s): %s\n", skill.Name, origin, skill.Description)
		listed++
	}
	if listed == 0 {
		b.WriteString("(none)\n")
	}
	return b.String()
}

func pinnedExtraction(cfg *config.Config) (string, string, bool) {
	if cfg == nil {
		return "", "", false
	}
	providerID := strings.TrimSpace(cfg.Memory.ExtractionProvider)
	modelID := strings.TrimSpace(cfg.Memory.ExtractionModel)
	if providerID == "" || modelID == "" {
		return "", "", false
	}
	return providerID, modelID, true
}

func turnMutations(rows []db.Message, calls []db.ToolCall) (string, []skillCall) {
	userIdx := -1
	for i, row := range rows {
		if row.Role == "user" {
			userIdx = i
		}
	}
	if userIdx < 0 {
		return "", nil
	}
	userText := session.DisplayTextFromStoredContent(rows[userIdx].Content)
	after := map[string]bool{}
	var resultRows []db.Message
	for _, row := range rows[userIdx+1:] {
		after[row.ID] = true
		if row.Role == "tool_result" {
			resultRows = append(resultRows, row)
		}
	}
	seen, failed := toolResultFlags(resultRows)
	out := make([]skillCall, 0, len(calls))
	for _, call := range calls {
		if !after[call.MessageID] {
			continue
		}
		path, command := callTarget(call.ToolName, call.Arguments)
		out = append(out, skillCall{
			Name:    call.ToolName,
			Path:    path,
			Command: command,
			OK:      seen[call.ID] && !failed[call.ID],
		})
	}
	return userText, out
}

func toolResultFlags(rows []db.Message) (seen, failed map[string]bool) {
	seen = map[string]bool{}
	failed = map[string]bool{}
	for _, row := range rows {
		var payload struct {
			ToolCallID string `json:"tool_call_id"`
			IsError    bool   `json:"is_error"`
		}
		if err := json.Unmarshal([]byte(row.Content), &payload); err != nil || payload.ToolCallID == "" {
			continue
		}
		seen[payload.ToolCallID] = true
		if payload.IsError {
			failed[payload.ToolCallID] = true
		}
	}
	return seen, failed
}

func callTarget(name, args string) (path, command string) {
	var in struct {
		Path    string `json:"path"`
		Command string `json:"command"`
	}
	_ = json.Unmarshal([]byte(args), &in)
	switch name {
	case "edit_file", "write_file":
		return in.Path, ""
	case "run_command":
		return "", in.Command
	default:
		return "", ""
	}
}

func formatTurnTranscript(rows []db.Message, calls []db.ToolCall) string {
	userIdx := -1
	for i, row := range rows {
		if row.Role == "user" {
			userIdx = i
		}
	}
	if userIdx < 0 {
		return ""
	}
	return formatTranscriptRows(rows[userIdx:], calls)
}

func formatTranscriptSince(rows []db.Message, calls []db.ToolCall, since int64) string {
	if since <= 0 {
		return formatTranscriptRows(rows, calls)
	}
	start := len(rows)
	for i, row := range rows {
		if row.CreatedAt > since {
			start = i
			break
		}
	}
	return formatTranscriptRows(rows[start:], calls)
}

func formatTranscriptRows(rows []db.Message, calls []db.ToolCall) string {
	byMessage := map[string][]db.ToolCall{}
	for _, call := range calls {
		byMessage[call.MessageID] = append(byMessage[call.MessageID], call)
	}
	var b strings.Builder
	for _, row := range rows {
		switch row.Role {
		case "user":
			fmt.Fprintf(&b, "User: %s\n", session.DisplayTextFromStoredContent(row.Content))
		case "assistant":
			text := strings.TrimSpace(session.DisplayTextFromStoredContent(row.Content))
			if text != "" {
				fmt.Fprintf(&b, "Assistant: %s\n", text)
			}
			for _, call := range byMessage[row.ID] {
				fmt.Fprintf(&b, "Tool %s %s\n", call.ToolName, strings.TrimSpace(call.Arguments))
				if strings.TrimSpace(call.Result) != "" {
					fmt.Fprintf(&b, "Result: %s\n", strings.TrimSpace(call.Result))
				}
			}
		case "tool_result":
			var payload struct {
				ToolCallID string `json:"tool_call_id"`
				Content    string `json:"content"`
				IsError    bool   `json:"is_error"`
			}
			if err := json.Unmarshal([]byte(row.Content), &payload); err != nil {
				continue
			}
			label := "ok"
			if payload.IsError {
				label = "error"
			}
			fmt.Fprintf(&b, "Tool result %s (%s): %s\n", payload.ToolCallID, label, strings.TrimSpace(payload.Content))
		}
	}
	return b.String()
}

func collectSkillWrites(ctx context.Context, store skillReviewStore, childID string) []event.SkillReviewChange {
	calls, err := store.ListToolCallsForSession(ctx, childID)
	if err != nil {
		return nil
	}
	rows, err := store.ListMessageRows(ctx, childID)
	if err != nil {
		return nil
	}
	seen, failed := toolResultFlags(rows)
	var out []event.SkillReviewChange
	for _, call := range calls {
		if call.ToolName != "write_skill" || !seen[call.ID] || failed[call.ID] {
			continue
		}
		var args struct {
			Name      string `json:"name"`
			Content   string `json:"content"`
			Overwrite bool   `json:"overwrite"`
		}
		if err := json.Unmarshal([]byte(call.Arguments), &args); err != nil || strings.TrimSpace(args.Name) == "" {
			continue
		}
		action := "created"
		if args.Overwrite {
			action = "updated"
		}
		out = append(out, event.SkillReviewChange{
			Name:        strings.TrimSpace(args.Name),
			Action:      action,
			Description: skills.SkillMarkdownDescription(args.Content),
		})
	}
	return out
}

func encodeTargets(targets skillTargets) string {
	if len(targets.Paths) == 0 && len(targets.Commands) == 0 {
		return ""
	}
	raw, err := json.Marshal(targets)
	if err != nil {
		return ""
	}
	return string(raw)
}

func decodeTargets(raw string) skillTargets {
	var targets skillTargets
	if strings.TrimSpace(raw) == "" {
		return targets
	}
	_ = json.Unmarshal([]byte(raw), &targets)
	return targets
}

func unixMilliTime(ms int64) time.Time {
	if ms <= 0 {
		return time.Time{}
	}
	return time.UnixMilli(ms)
}
