package agent

import (
	"context"
	"time"

	cometsdk "github.com/Cometline/cometline/comet-sdk"
	"github.com/Cometline/cometline/cometmind/internal/config"
	"github.com/Cometline/cometline/cometmind/internal/event"
	"github.com/Cometline/cometline/cometmind/internal/session"
	"github.com/Cometline/cometline/cometmind/internal/subagent"
	"github.com/Cometline/cometline/cometmind/internal/tools"
)

// Runner executes the persisted agent loop for one user turn (which may span many tool steps).
type Runner struct {
	Config   *config.Config
	Provider cometsdk.Provider
	Sessions TurnStore
	Memory   MemoryStore
	Registry *tools.Registry
	Jobs     OngoingJobLookup

	MaxSteps               int
	MemoryRetrievalTimeout time.Duration
	// StreamRecoveryBackoff is the first wait between recoverable model-stream
	// retries. Later attempts double it up to eight seconds. Zero uses 2s.
	StreamRecoveryBackoff time.Duration
	SystemPrompt          string
	AgentMode             session.AgentMode
	SkillIndex            string
	JobIndex              string
	SubagentOrchestrator  *subagent.Orchestrator

	// MemorySem is an optional semaphore that bounds the number of
	// extractMemoryAfterTurn calls that may run concurrently across all
	// sessions. When non-nil, each background goroutine acquires one slot
	// before starting and releases it on completion. A nil value means
	// unlimited (the previous behaviour).
	MemorySem          chan struct{}
	Compatibility      cometsdk.CapabilitySource
	CompatibilityScope cometsdk.CapabilityScope

	// Compactor performs rolling context compaction on long sessions. Nil disables it.
	Compactor *ContextCompactor

	// Events publishes background results after the turn SSE channel has closed.
	Events interface{ Publish(event.Event) }
	// ReviewChild runs the hidden skill-review fork. Nil skips the fork.
	ReviewChild ReviewChild
	// ReviewNow overrides the clock used for skill-review cooldown. Nil uses time.Now.
	ReviewNow func() time.Time
}

// Run streams CometMind-native events on ch until the turn completes or ctx is cancelled.
// The caller must receive until the channel closes.
func (r *Runner) Run(ctx context.Context, turn session.AgentTurn, ch chan<- event.Event) error {
	s := &turnState{turn: turn, ch: ch}
	defer s.sendDone()

	if r.MaxSteps <= 0 {
		r.MaxSteps = 100
	}
	r.initTurnState(ctx, s)

	// MaxSteps limits work rounds. If they are exhausted, make one final
	// tool-free request so the user still receives a best-effort answer.
	for s.steps <= r.MaxSteps {
		if ctx.Err() != nil {
			return nil
		}
		p, err := r.prepareStep(ctx, s)
		if err != nil {
			return err
		}
		result, startedToolCalls, done, err := r.streamStep(ctx, s, p)
		if done {
			return err
		}
		outcome, toolIDs, err := r.settleModelStep(ctx, s, p, result, startedToolCalls)
		if err != nil || outcome == stepDone {
			return err
		}
		if outcome == stepAgain {
			continue
		}
		if err := r.executeToolBatch(ctx, s, p, result, toolIDs); err != nil {
			return err
		}
		s.steps++
	}

	return r.completeTurn(ctx, s)
}
