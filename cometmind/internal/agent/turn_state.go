package agent

import (
	"context"
	"time"

	cometsdk "github.com/Cometline/cometline/comet-sdk"
	"github.com/Cometline/cometline/cometmind/internal/event"
	"github.com/Cometline/cometline/cometmind/internal/session"
)

// turnState holds the bookkeeping that survives across the steps of one turn.
type turnState struct {
	turn session.AgentTurn
	ch   chan<- event.Event
	sess session.Session

	budget           SessionBudget
	maxTokens        int
	retrievalTimeout time.Duration
	jobTracker       *JobProgressTracker

	steps                                 int
	outputTruncationContinuations         int
	incompleteToolTruncationContinuations int
	invalidToolInputStreak                int
	doomLoopHalt                          bool
	degradationsReported                  bool
	doneSent                              bool

	// Injected memories belong to the first assistant message of the turn. They
	// are captured when retrieved (step 0) and attached to the first
	// AppendAssistantStep call so they persist and rebuild on reload.
	pendingMemories []session.InjectedMemory

	nudges continueNudges
}

// continueNudges are decided while one step finishes and consumed when the
// next step builds its request.
type continueNudges struct {
	truncation               bool
	incompleteToolTruncation bool
	jobProgress              bool
	jobCompletionGate        bool
	subagentWait             bool
	subagentResults          string
}

func (n continueNudges) messages(jobID string) []cometsdk.Message {
	return ContinueUserNudgeMessages(
		n.truncation,
		n.incompleteToolTruncation,
		n.jobProgress,
		n.jobCompletionGate,
		jobID,
		n.subagentWait,
		n.subagentResults,
	)
}

func (r *Runner) initTurnState(ctx context.Context, s *turnState) {
	s.retrievalTimeout = r.MemoryRetrievalTimeout
	if s.retrievalTimeout <= 0 {
		s.retrievalTimeout = memoryRetrievalTimeout
	}
	s.jobTracker = newJobProgressTracker(ctx, r.Jobs, s.turn.ID)
	if svc, ok := r.Sessions.(sessionLoader); ok {
		if loaded, err := svc.GetSession(ctx, s.turn.ID); err == nil {
			s.sess = loaded
		}
	}
	if s.sess.ID == "" {
		s.sess.ID = s.turn.ID
	}
	// Output ceiling is min(turn model output, 32k), computed in ResolveSessionBudget.
	s.budget = ResolveSessionBudget(r.Config, s.turn.ProviderID, s.turn.ModelID)
	s.maxTokens = s.budget.EffectiveMaxTokens
}

// emit forwards a turn event. Compaction passes this method as its status callback.
func (s *turnState) emit(ev event.Event) {
	s.ch <- ev
}

func (s *turnState) emitStatus(phase event.TurnPhase) {
	s.ch <- event.TurnStatus(phase, "")
}

// fail reports err on the turn stream with the given source and returns it.
func (s *turnState) fail(err error, source string) error {
	s.ch <- event.Errorf(err.Error(), source)
	return err
}

func (s *turnState) sendDone() {
	if s.doneSent {
		return
	}
	s.ch <- event.Done()
	s.doneSent = true
}
