package event

// Kind identifies a CometMind-native runtime event. The same value is the SSE
// "type" discriminator on the wire, so adding a Kind is a wire-contract change.
type Kind string

const (
	KindTextDelta                 Kind = "text_delta"
	KindReasoningStart            Kind = "reasoning_start"
	KindReasoningDelta            Kind = "reasoning_delta"
	KindToolCall                  Kind = "tool_call"
	KindToolResult                Kind = "tool_result"
	KindStepFinish                Kind = "step_finish"
	KindSubagentStarted           Kind = "subagent_started"
	KindSubagentProgress          Kind = "subagent_progress"
	KindSubagentFinished          Kind = "subagent_finished"
	KindMemoryInjected            Kind = "memory_injected"
	KindMemoryUpdated             Kind = "memory_updated"
	KindSkillReviewUpdated        Kind = "skill_review_updated"
	KindWikiReviewUpdated         Kind = "wiki_review_updated"
	KindMemoryCompactionCompleted Kind = "memory_compaction_completed"
	KindContextBudget             Kind = "context_budget"
	KindInboxMessageCreated       Kind = "inbox_message_created"
	KindInboxMessageArchived      Kind = "inbox_message_archived"
	KindRunStarted                Kind = "run_started"
	KindRunFinished               Kind = "run_finished"
	KindSessionCleared            Kind = "session_cleared"
	KindTurnStatus                Kind = "turn_status"
	KindTurnRecover               Kind = "turn_recover"
	KindAssistantImage            Kind = "assistant_image"
	KindAssistantVideo            Kind = "assistant_video"
	KindError                     Kind = "error"
	KindDone                      Kind = "done"
)

// TurnPhase identifies what the agent is doing before visible output streams.
type TurnPhase string

const (
	PhaseRetrievingMemories TurnPhase = "retrieving_memories"
	PhaseCompactingContext  TurnPhase = "compacting_context"
	PhaseContactingModel    TurnPhase = "contacting_model"
	PhaseComposingResponse  TurnPhase = "composing_response"
	PhaseRunningTools       TurnPhase = "running_tools"
	PhaseContinuing         TurnPhase = "continuing"
)

// MemoryWire is the SSE payload for an injected memory.
type MemoryBucket string

const (
	MemoryBucketPreference  MemoryBucket = "preference"
	MemoryBucketTaskOutcome MemoryBucket = "task_outcome"
	MemoryBucketSemantic    MemoryBucket = "semantic"
)

type MemoryWire struct {
	ID              string       `json:"id"`
	Content         string       `json:"content"`
	Kind            string       `json:"kind"`
	Bucket          MemoryBucket `json:"bucket"`
	Similarity      float64      `json:"similarity"`
	EffectiveWeight float64      `json:"effective_weight"`
}

// MemoryChangeWire is the SSE payload for an agent or extractor memory change.
type MemoryChangeWire struct {
	Action  string `json:"action"`
	Kind    string `json:"kind"`
	Content string `json:"content"`
	ID      string `json:"id,omitempty"`
}

// SkillReviewChange is one skill a hidden review fork created or updated.
type SkillReviewChange struct {
	Name        string `json:"name"`
	Action      string `json:"action"`
	Description string `json:"description"`
}

// Usage mirrors the SSE token-usage payload (one source of truth for the wire).
type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
	CacheRead    int `json:"cache_read"`
	CacheWrite   int `json:"cache_write"`
}

// Event is the single runtime event type shared by the agent runner, the SSE
// server, and the CLI. It is also the SSE wire shape: MarshalJSON emits
// exactly the fields each Kind carries, discriminated by "type". Field names are
// per-Kind by contract — reasoning_delta carries "text", text_delta carries
// "delta" — so consumers read the field that matches the Kind.
type Event struct {
	Kind Kind

	// text_delta
	Delta string
	// reasoning_delta
	Text string
	// tool_call / tool_result
	ID      string
	Tool    string
	Input   []byte // tool_call: JSON object bytes; empty marshals as {}
	Output  string // tool_result
	ToolErr string // tool_result: empty if success
	// step_finish
	Usage Usage
	// subagent_*
	ChildSessionID   string
	Purpose          string
	AgentName        string
	ProgressKind     string
	ProgressText     string
	DelegationStatus string
	Summary          string
	// memory_injected
	Memories []MemoryWire
	// memory_updated
	MemoryChanges []MemoryChangeWire
	// skill_review_updated
	SkillReviews []SkillReviewChange
	// wiki_review_updated
	WikiPaths []string
	// memory_compaction_completed
	MemoryCountBefore int64
	MemoryCountAfter  int64
	CompactionTrigger string
	// context_budget
	BudgetEstimated     int
	BudgetAvailable     int
	BudgetContextWindow int
	BudgetCompacted     bool
	// inbox_message_created / inbox_message_archived
	InboxMessageID     string
	InboxOpenCount     int64
	InboxArchiveReason string
	// run_started / run_finished
	SessionID string
	// turn_status
	Phase         TurnPhase
	StatusMessage string
	// turn_recover
	TextChars      int
	ReasoningChars int
	// assistant_image
	ImageID   string
	MediaType string
	Alt       string
	DataURL   string
	// error
	Message string
	Code    string
}
