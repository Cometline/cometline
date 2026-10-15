package agent

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	cometsdk "github.com/Cometline/cometline/comet-sdk"
	"github.com/Cometline/cometline/comet-sdk/llm"
	"github.com/Cometline/cometline/cometmind/internal/config"
	"github.com/Cometline/cometline/cometmind/internal/event"
	"github.com/Cometline/cometline/cometmind/internal/logging"
	"github.com/Cometline/cometline/cometmind/internal/session"
	"go.uber.org/zap"
)

// errSoftCompact marks summarize and persist failures that automatic compaction
// logs and ignores. Manual compaction surfaces them.
var errSoftCompact = errors.New("soft compact")

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

// PromptBudget is the chars/4 estimate used to gate context compaction.
type PromptBudget struct {
	Estimated     int
	Available     int
	ContextWindow int
	Reserve       int
	EffectiveMax  int
}

// EstimatePromptBudget computes the same budget numbers MaybeCompact uses.
func (c *ContextCompactor) EstimatePromptBudget(
	ctx context.Context,
	sessionID string,
	system string,
	tools []cometsdk.Tool,
	providerID, modelID string,
) (PromptBudget, error) {
	budget := PromptBudget{}
	if c == nil {
		return budget, nil
	}
	sb := ResolveSessionBudget(c.Config, providerID, modelID)
	budget.ContextWindow = sb.Context
	budget.Available = sb.Available
	budget.Reserve = sb.Reserve
	budget.EffectiveMax = sb.EffectiveMaxTokens
	if c.Sessions == nil || sessionID == "" {
		return budget, nil
	}
	msgs, err := c.Sessions.BuildSDKMessages(ctx, sessionID)
	if err != nil {
		return budget, err
	}
	budget.Estimated = EstimatePromptTokens(PromptBudgetInput{
		System:       system,
		Messages:     msgs,
		Tools:        tools,
		OutputBudget: sb.EffectiveMaxTokens,
	})
	return budget, nil
}

// CompactStatus is the outcome of a manual context compaction.
type CompactStatus string

const (
	CompactStatusCompacted CompactStatus = "compacted"
	CompactStatusSkipped   CompactStatus = "skipped"
)

// CompactOutcome is the result of ForceCompact.
type CompactOutcome struct {
	Session       session.Session
	Status        CompactStatus
	Reason        string
	ProviderID    string
	ModelID       string
	Estimated     int
	Available     int
	ContextWindow int
}

// ModelOpener opens the provider used to summarize a manual compaction.
type ModelOpener func(providerID, modelID string) (cometsdk.Provider, error)

// MaybeCompact summarizes older history when the prompt budget is exceeded.
// When force is true, compaction runs even if the estimate is under the threshold
// (used for provider context-overflow recovery).
// Failures are logged and leave prior summary state untouched.
func (c *ContextCompactor) MaybeCompact(
	ctx context.Context,
	sess session.Session,
	system string,
	tools []cometsdk.Tool,
	provider cometsdk.Provider,
	providerID, modelID string,
	force bool,
	status func(event.Event),
) (session.Session, error) {
	result := c.runCompact(ctx, sess, system, tools, provider, nil, providerID, modelID, force, status)
	if result.err != nil {
		if errors.Is(result.err, errSoftCompact) {
			return sess, nil
		}
		return sess, result.err
	}
	return result.sess, nil
}

// ForceCompact summarizes older history even when the prompt is under the
// compaction threshold. Summarize and persist failures are returned.
func (c *ContextCompactor) ForceCompact(ctx context.Context, sess session.Session, open ModelOpener) (CompactOutcome, error) {
	outcome := CompactOutcome{Session: sess, Status: CompactStatusSkipped}
	if c == nil || c.Sessions == nil {
		outcome.Reason = "no_prefix"
		return outcome, nil
	}
	providerID, modelID := c.resolveModel(sess)
	outcome.ProviderID = providerID
	outcome.ModelID = modelID
	result := c.runCompact(ctx, sess, "", nil, nil, open, providerID, modelID, true, nil)
	outcome.Session = result.sess
	if result.err != nil {
		if errors.Is(result.err, context.Canceled) || errors.Is(result.err, context.DeadlineExceeded) {
			outcome.Reason = "cancelled"
			return outcome, nil
		}
		return outcome, result.err
	}
	if result.reason != "" {
		outcome.Reason = result.reason
		return outcome, nil
	}
	outcome.Status = CompactStatusCompacted
	outcome.Session = result.sess
	return outcome, nil
}

type compactRun struct {
	sess   session.Session
	reason string
	err    error
}

func (c *ContextCompactor) runCompact(
	ctx context.Context,
	sess session.Session,
	system string,
	tools []cometsdk.Tool,
	provider cometsdk.Provider,
	open ModelOpener,
	providerID, modelID string,
	force bool,
	status func(event.Event),
) compactRun {
	if c == nil || c.Sessions == nil {
		return compactRun{sess: sess, reason: "no_prefix"}
	}

	sb := ResolveSessionBudget(c.Config, providerID, modelID)
	rows, err := c.Sessions.ListMessageRows(ctx, sess.ID)
	if err != nil {
		return compactRun{sess: sess, err: err}
	}

	msgs, err := c.Sessions.BuildSDKMessages(ctx, sess.ID)
	if err != nil {
		return compactRun{sess: sess, err: err}
	}

	estimated := EstimatePromptTokens(PromptBudgetInput{
		System:       system,
		Messages:     msgs,
		Tools:        tools,
		OutputBudget: sb.EffectiveMaxTokens,
	})
	if !force && !ShouldCompact(estimated, sb.Available) {
		return compactRun{sess: sess}
	}

	allCalls, err := c.Sessions.ListToolCallsForSession(ctx, sess.ID)
	if err != nil {
		return compactRun{sess: sess, err: err}
	}
	callsByMessage := session.GroupToolCallsByMessage(allCalls)
	activeRows := session.FilterMessagesAfterCompacted(rows, sess.CompactedUntilMessageID)
	recentStart := session.RecentWindowStartForKeep(activeRows, callsByMessage, compactKeepTokens(sb.Available))
	if recentStart <= 0 {
		logging.L().Info("context.compact.skipped", zap.String("session", sess.ID), zap.String("reason", "no_prefix_messages"), zap.Bool("force", force))
		return compactRun{sess: sess, reason: "no_prefix"}
	}

	prefixRows := activeRows[:recentStart]
	prefixText := session.FormatTranscriptForSummary(prefixRows)
	if prefixText == "" {
		return compactRun{sess: sess, reason: "empty_prefix"}
	}

	if provider == nil {
		if open == nil {
			return compactRun{sess: sess, err: fmt.Errorf("context compaction requires a model provider")}
		}
		provider, err = open(providerID, modelID)
		if err != nil {
			return compactRun{sess: sess, err: err}
		}
	}
	summaryModel := modelID
	if strings.TrimSpace(sess.ModelID) != "" {
		summaryModel = sess.ModelID
	}

	if status != nil {
		status(event.TurnStatus(event.PhaseCompactingContext, ""))
	}

	newSummary, err := c.summarize(ctx, provider, summaryModel, sb.EffectiveMaxTokens, sess.ContextSummary, prefixText)
	if err != nil {
		logging.L().Error("context.compact.failed", zap.String("session", sess.ID), zap.Error(err))
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return compactRun{sess: sess, err: err}
		}
		return compactRun{sess: sess, err: fmt.Errorf("%w: %w", errSoftCompact, err)}
	}
	untilID := prefixRows[len(prefixRows)-1].ID
	if err := c.Sessions.UpdateContextSummary(ctx, sess.ID, newSummary, untilID); err != nil {
		logging.L().Error("context.compact.persist_failed", zap.String("session", sess.ID), zap.Error(err))
		return compactRun{sess: sess, err: fmt.Errorf("%w: %w", errSoftCompact, err)}
	}
	if states, ok := c.Sessions.(interface {
		ClearAssistantProviderState(context.Context, string) error
	}); ok {
		if err := states.ClearAssistantProviderState(ctx, sess.ID); err != nil {
			logging.L().Warn("context.compact.provider_state_clear_failed", zap.String("session", sess.ID), zap.Error(err))
		}
	}

	logging.L().Info("context.compact.done", zap.String("session", sess.ID), zap.String("until_message", untilID), zap.Int("summary_bytes", len(newSummary)), zap.Int("recent_start", recentStart), zap.Bool("force", force))
	sess.ContextSummary = newSummary
	sess.CompactedUntilMessageID = untilID
	sess.ContextSummaryUpdatedAt = time.Now().UTC().Format(time.RFC3339)
	return compactRun{sess: sess}
}

func compactKeepTokens(available int) int {
	keep := session.CompactKeepTokens
	if available > 0 {
		ceiling := int(float64(available) * session.RecentWindowMaxRatio)
		if ceiling > 0 && keep > ceiling {
			keep = ceiling
		}
	}
	return keep
}

func (c *ContextCompactor) resolveModel(sess session.Session) (string, string) {
	providerID := strings.TrimSpace(sess.ProviderID)
	modelID := strings.TrimSpace(sess.ModelID)
	if c != nil && c.Config != nil {
		if providerID == "" {
			providerID = c.Config.DefaultProviderID
		}
		if modelID == "" {
			modelID = c.Config.DefaultModelID
		}
	}
	return providerID, modelID
}

// Prune stubs old tool outputs in future model prompts without deleting UI copies.
func (c *ContextCompactor) Prune(ctx context.Context, sessionID string) error {
	if c == nil || c.Sessions == nil || sessionID == "" {
		return nil
	}
	rows, err := c.Sessions.ListMessageRows(ctx, sessionID)
	if err != nil {
		return err
	}
	allCalls, err := c.Sessions.ListToolCallsForSession(ctx, sessionID)
	if err != nil {
		return err
	}
	ids := session.SelectToolCallsToPrune(rows, session.GroupToolCallsByMessage(allCalls))
	if len(ids) == 0 {
		return nil
	}
	if err := c.Sessions.MarkToolCallsCompacted(ctx, ids, time.Now().UTC().UnixMilli()); err != nil {
		return err
	}
	logging.L().Info("context.prune.done", zap.String("session", sessionID), zap.Int("count", len(ids)))
	return nil
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
	prompt := userPrompt.String()
	out, err := c.summarizeOnce(ctx, provider, modelID, summaryTokens, prompt, "")
	if err == nil && strings.TrimSpace(out) != "" {
		return strings.TrimSpace(out), nil
	}
	// Reasoning models such as Grok sometimes return only hidden thinking.
	// A low-effort retry asks for the visible summary instead of failing the run.
	retried, retryErr := c.summarizeOnce(ctx, provider, modelID, summaryTokens, prompt, "low")
	if retryErr == nil && strings.TrimSpace(retried) != "" {
		return strings.TrimSpace(retried), nil
	}
	if err != nil {
		return "", err
	}
	if retryErr != nil {
		return "", retryErr
	}
	return "", fmt.Errorf("summarize context: empty summary")
}

func (c *ContextCompactor) summarizeOnce(
	ctx context.Context,
	provider cometsdk.Provider,
	modelID string,
	maxTokens int,
	prompt, reasoningEffort string,
) (string, error) {
	req := &cometsdk.Request{
		Model:  modelID,
		System: contextSummarySystemPrompt,
		Messages: []cometsdk.Message{{
			Role:    cometsdk.RoleUser,
			Content: []cometsdk.Block{cometsdk.TextBlock{Text: prompt}},
		}},
		MaxTokens:       maxTokens,
		ReasoningEffort: reasoningEffort,
	}
	result, err := llm.Collect(ctx, provider, req)
	if err != nil {
		return "", fmt.Errorf("summarize context: %w", err)
	}
	out := strings.TrimSpace(result.Text)
	if out == "" {
		logging.L().Warn(
			"context.compact.empty_summary",
			zap.String("model", modelID),
			zap.String("finish", result.FinishReason),
			zap.Int("reasoning_bytes", len(result.ReasoningText)),
			zap.String("effort", reasoningEffort),
		)
	}
	return out, nil
}
