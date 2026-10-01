package agent

import (
	"context"
	"errors"
	"time"

	cometsdk "github.com/Cometline/cometline/comet-sdk"
	"github.com/Cometline/cometline/cometmind/internal/event"
	"github.com/Cometline/cometline/cometmind/internal/logging"
	"github.com/Cometline/cometline/cometmind/internal/memory"
	"github.com/Cometline/cometline/cometmind/internal/provider"
	"github.com/Cometline/cometline/cometmind/internal/session"
)

const memoryRetrievalTimeout = 3 * time.Second

// injectTurnMemories retrieves memories for the first step of a turn, records
// them for the first persisted assistant message, and returns the system
// prompt suffix that carries them.
func (r *Runner) injectTurnMemories(ctx context.Context, s *turnState, baseSystem string, msgs []cometsdk.Message) string {
	if r.Memory == nil || !r.Memory.Enabled() || s.steps != 0 {
		return ""
	}
	turnID := s.turn.ID
	decision := memory.DecideRetrieval(msgs)
	logging.L().Info("memory.retrieve.policy", "session", turnID, "retrieve", decision.Retrieve, "reason", decision.Reason, "score", decision.Score, "text_bytes", decision.TextBytes)
	if !decision.Retrieve {
		logging.L().Info("memory.retrieve.skipped", "session", turnID, "reason", decision.Reason, "score", decision.Score, "text_bytes", decision.TextBytes)
		return ""
	}
	s.emitStatus(event.PhaseRetrievingMemories)
	query := memory.BuildRetrievalQuery(memory.RetrievalQueryInput{
		Messages: msgs,
	})
	allowance := memoryTokenAllowance(s.budget, baseSystem, msgs, r.Registry.CometSDK())
	retrieveCtx, cancel := context.WithTimeout(ctx, s.retrievalTimeout)
	promptMemories, memErr := r.Memory.RetrieveForTurn(retrieveCtx, turnID, query, allowance)
	cancel()
	if memErr != nil {
		if errors.Is(memErr, context.DeadlineExceeded) {
			logging.L().Warn("memory.retrieve.timeout", "session", turnID, "budget_ms", s.retrievalTimeout.Milliseconds())
		} else {
			logging.L().Error("memory.retrieve.failed", "session", turnID, "error", memErr)
			s.ch <- event.Errorf(memErr.Error(), "memory")
		}
	}
	if len(promptMemories.Records) == 0 {
		return ""
	}
	logging.L().Info("memory.injected", "session", turnID, "preferences", promptMemories.Count(memory.BucketPreference), "task_outcomes", promptMemories.Count(memory.BucketTaskOutcome), "semantic", promptMemories.Count(memory.BucketSemantic), "token_allowance", allowance)
	suffix := memory.FormatPromptMemories(promptMemories)
	wire, injected := promptMemoriesToWire(promptMemories.Records)
	s.pendingMemories = injected
	s.ch <- event.MemoryInjected(wire)
	return suffix
}

func promptMemoriesToWire(records []memory.PromptMemory) ([]event.MemoryWire, []session.InjectedMemory) {
	wire := make([]event.MemoryWire, len(records))
	injected := make([]session.InjectedMemory, len(records))
	for i, m := range records {
		wire[i] = event.MemoryWire{
			ID:              m.ID,
			Content:         m.Content,
			Kind:            m.Kind,
			Bucket:          event.MemoryBucket(m.Bucket),
			Similarity:      m.Similarity,
			EffectiveWeight: m.EffectiveWeight,
		}
		injected[i] = session.InjectedMemory{
			ID:              m.ID,
			Content:         m.Content,
			Kind:            m.Kind,
			Bucket:          session.MemoryBucket(m.Bucket),
			Similarity:      m.Similarity,
			EffectiveWeight: m.EffectiveWeight,
		}
	}
	return wire, injected
}

func memoryTokenAllowance(budget SessionBudget, system string, messages []cometsdk.Message, tools []cometsdk.Tool) int {
	modelShare := budget.Available / 20
	if modelShare > 4096 {
		modelShare = 4096
	}
	if modelShare < 0 {
		modelShare = 0
	}
	remaining := budget.Available - EstimatePromptTokens(PromptBudgetInput{System: system, Messages: messages, Tools: tools})
	if remaining < modelShare {
		modelShare = remaining
	}
	if modelShare < 0 {
		return 0
	}
	return modelShare
}

func memoryChangesToWire(changes []memory.Change) []event.MemoryChangeWire {
	if len(changes) == 0 {
		return nil
	}
	wire := make([]event.MemoryChangeWire, 0, len(changes))
	for _, change := range changes {
		wire = append(wire, event.MemoryChangeWire{
			Action:  change.Action,
			Kind:    change.Kind,
			Content: change.Content,
			ID:      change.ID,
		})
	}
	return wire
}

func (r *Runner) extractMemoryAfterTurn(ctx context.Context, turn session.AgentTurn, ch chan<- event.Event) {
	if r.Memory == nil || !r.Memory.Enabled() {
		return
	}
	// Honour the optional concurrency cap: acquire a slot before doing any
	// work and release it when done. This prevents N simultaneous session
	// completions from spawning N unbounded LLM API calls and SQLite writes.
	if r.MemorySem != nil {
		select {
		case r.MemorySem <- struct{}{}:
			defer func() { <-r.MemorySem }()
		case <-ctx.Done():
			return
		}
	}
	providerID, model := turn.ProviderID, turn.ModelID
	llmProvider := r.Provider
	if r.Config != nil {
		providerID, model = r.Config.ExtractionLLM()
		if providerID == "" || model == "" {
			logging.L().Warn("memory.extract.skipped_no_model", "session", turn.ID)
			return
		}
		if p, err := provider.NewForModel(r.Config, providerID, model); err == nil {
			llmProvider = p
		} else {
			logging.L().Warn("memory.extract.provider_failed", "session", turn.ID, "provider", providerID, "error", err)
			return
		}
	}
	changes, err := r.Memory.ExtractAfterTurn(ctx, turn.ID, model, llmProvider)
	if err != nil {
		logging.L().Warn("memory.extract.after_turn_failed", "session", turn.ID, "provider", providerID, "error", err)
		return
	}
	if ch != nil {
		if wire := memoryChangesToWire(changes); len(wire) > 0 {
			ch <- event.MemoryUpdated(wire)
		}
	}
}
