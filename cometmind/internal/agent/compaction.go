package agent

import (
	"context"
	"fmt"
	"strings"
	"time"

	cometsdk "github.com/cometline/comet-sdk"
	"github.com/cometline/comet-sdk/llm"
	"github.com/cometline/cometmind/internal/config"
	"github.com/cometline/cometmind/internal/event"
	"github.com/cometline/cometmind/internal/logging"
	"github.com/cometline/cometmind/internal/session"
)

const contextSummarySystemPrompt = `You maintain a rolling session context summary for a coding assistant.
Rewrite the summary to preserve:
- user goals and intent
- decisions made
- constraints and preferences
- important tool outcomes and file paths
- unresolved questions and pending work

Drop repetitive chat, verbose tool dumps, status noise, and chain-of-thought.
Output plain text only with short sections when helpful.`

// ContextCompactor performs rolling transcript compaction for long sessions.
type ContextCompactor struct {
	Sessions session.CompactorStore
	Config   *config.Config
}

// MaybeCompact summarizes older history when the prompt budget is exceeded.
// Failures are logged and leave prior summary state untouched.
func (c *ContextCompactor) MaybeCompact(
	ctx context.Context,
	sess session.Session,
	system string,
	tools []cometsdk.Tool,
	provider cometsdk.Provider,
	maxTokens int,
	status func(event.Event),
) (session.Session, error) {
	if c == nil || c.Sessions == nil {
		return sess, nil
	}

	contextWindow := ResolveContextWindow(c.Config)
	rows, err := c.Sessions.ListMessageRows(ctx, sess.ID)
	if err != nil {
		return sess, err
	}

	allMsgs, err := c.Sessions.BuildSDKMessagesAll(ctx, sess.ID)
	if err != nil {
		return sess, err
	}

	estimated := EstimatePromptTokens(PromptBudgetInput{
		System:       system,
		Summary:      sess.ContextSummary,
		Messages:     allMsgs,
		Tools:        tools,
		OutputBudget: maxTokens,
	})
	if !ShouldCompact(estimated, contextWindow, maxTokens) {
		return sess, nil
	}

	allCalls, err := c.Sessions.ListToolCallsForSession(ctx, sess.ID)
	if err != nil {
		return sess, err
	}
	callsByMessage := session.GroupToolCallsByMessage(allCalls)
	recentStart := session.RecentWindowStartForBudget(
		rows,
		callsByMessage,
		recentTurnPreserveCount,
		contextWindow,
		maxTokens,
	)
	prefixStart, prefixEnd := session.CompactionPrefixRange(rows, sess.CompactedUntilMessageID, recentStart)
	if prefixEnd <= prefixStart {
		logging.L().Info("context.compact.skipped", "session", sess.ID, "reason", "no_prefix_messages")
		return sess, nil
	}

	prefixRows := rows[prefixStart:prefixEnd]
	prefixText := session.FormatTranscriptForSummary(prefixRows)
	if prefixText == "" {
		return sess, nil
	}

	if status != nil {
		status(event.TurnStatus(event.PhaseCompactingContext, ""))
	}

	newSummary, err := c.summarize(ctx, provider, sess.ModelID, maxTokens, sess.ContextSummary, prefixText)
	if err != nil {
		logging.L().Error("context.compact.failed", "session", sess.ID, "error", err)
		return sess, nil
	}
	untilID := prefixRows[len(prefixRows)-1].ID
	if err := c.Sessions.UpdateContextSummary(ctx, sess.ID, newSummary, untilID); err != nil {
		logging.L().Error("context.compact.persist_failed", "session", sess.ID, "error", err)
		return sess, nil
	}
	if states, ok := c.Sessions.(interface {
		ClearAssistantProviderState(context.Context, string) error
	}); ok {
		if err := states.ClearAssistantProviderState(ctx, sess.ID); err != nil {
			logging.L().Warn("context.compact.provider_state_clear_failed", "session", sess.ID, "error", err)
		}
	}

	logging.L().Info("context.compact.done", "session", sess.ID, "until_message", untilID, "summary_bytes", len(newSummary), "recent_start", recentStart)
	sess.ContextSummary = newSummary
	sess.CompactedUntilMessageID = untilID
	sess.ContextSummaryUpdatedAt = time.Now().UTC().Format(time.RFC3339)
	return sess, nil
}

func (c *ContextCompactor) summarize(
	ctx context.Context,
	provider cometsdk.Provider,
	modelID string,
	maxTokens int,
	priorSummary, prefixTranscript string,
) (string, error) {
	var userPrompt strings.Builder
	if strings.TrimSpace(priorSummary) != "" {
		userPrompt.WriteString("Prior summary:\n")
		userPrompt.WriteString(strings.TrimSpace(priorSummary))
		userPrompt.WriteString("\n\n")
	}
	userPrompt.WriteString("New transcript to fold in:\n")
	userPrompt.WriteString(prefixTranscript)
	userPrompt.WriteString("\n\nRewrite the session summary.")

	summaryTokens := maxTokens
	if summaryTokens > 4096 {
		summaryTokens = 4096
	}
	req := &cometsdk.Request{
		Model:  modelID,
		System: contextSummarySystemPrompt,
		Messages: []cometsdk.Message{{
			Role:    cometsdk.RoleUser,
			Content: []cometsdk.Block{cometsdk.TextBlock{Text: userPrompt.String()}},
		}},
		MaxTokens: summaryTokens,
	}
	result, err := llm.GenerateText(ctx, provider, req)
	if err != nil {
		return "", fmt.Errorf("summarize context: %w", err)
	}
	out := strings.TrimSpace(result.Text)
	if out == "" {
		return "", fmt.Errorf("summarize context: empty summary")
	}
	return out, nil
}
