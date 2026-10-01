package agent

import (
	"context"
	"time"

	cometsdk "github.com/Cometline/cometline/comet-sdk"
	"github.com/Cometline/cometline/cometmind/internal/event"
	"github.com/Cometline/cometline/cometmind/internal/logging"
	"github.com/Cometline/cometline/cometmind/internal/memory"
	"github.com/Cometline/cometline/cometmind/internal/provider"
	"github.com/Cometline/cometline/cometmind/internal/session"
)

const memoryRetrievalTimeout = 3 * time.Second

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
