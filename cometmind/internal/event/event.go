package event

import (
	"encoding/json"
	"strings"

	cometsdk "github.com/cometline/comet-sdk"
)

// Kind identifies a CometMind-native runtime event. The same value is the SSE
// "type" discriminator on the wire, so adding a Kind is a wire-contract change.
type Kind string

const (
	KindTextDelta        Kind = "text_delta"
	KindReasoningStart   Kind = "reasoning_start"
	KindReasoningDelta   Kind = "reasoning_delta"
	KindToolCall         Kind = "tool_call"
	KindToolResult       Kind = "tool_result"
	KindStepFinish       Kind = "step_finish"
	KindSubagentStarted  Kind = "subagent_started"
	KindSubagentProgress Kind = "subagent_progress"
	KindSubagentFinished Kind = "subagent_finished"
	KindMemoryInjected   Kind = "memory_injected"
	KindMemoryUpdated    Kind = "memory_updated"
	KindTurnStatus       Kind = "turn_status"
	KindTurnRecover      Kind = "turn_recover"
	KindError            Kind = "error"
	KindDone             Kind = "done"
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
type MemoryWire struct {
	ID              string  `json:"id"`
	Content         string  `json:"content"`
	Kind            string  `json:"kind"`
	Similarity      float64 `json:"similarity"`
	EffectiveWeight float64 `json:"effective_weight"`
}

// MemoryChangeWire is the SSE payload for an agent or extractor memory change.
type MemoryChangeWire struct {
	Action  string `json:"action"`
	Kind    string `json:"kind"`
	Content string `json:"content"`
	ID      string `json:"id,omitempty"`
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
	// turn_status
	Phase         TurnPhase
	StatusMessage string
	// turn_recover
	TextChars      int
	ReasoningChars int
	// error
	Message string
	Code    string
}

// MarshalJSON renders the SSE wire frame body for this event. The output is the
// authoritative wire contract consumed by the cometline frontend. Each Kind
// marshals through a small typed struct so "type" stays first and field order is
// deterministic (a map would sort keys alphabetically and reorder the wire).
func (e Event) MarshalJSON() ([]byte, error) {
	t := string(e.Kind)
	switch e.Kind {
	case KindTextDelta:
		return json.Marshal(struct {
			Type  string `json:"type"`
			Delta string `json:"delta"`
		}{t, e.Delta})
	case KindReasoningDelta:
		return json.Marshal(struct {
			Type string `json:"type"`
			Text string `json:"text"`
		}{t, e.Text})
	case KindToolCall:
		input := json.RawMessage(e.Input)
		if len(input) == 0 {
			input = json.RawMessage("{}")
		}
		return json.Marshal(struct {
			Type  string          `json:"type"`
			ID    string          `json:"id"`
			Tool  string          `json:"tool"`
			Input json.RawMessage `json:"input"`
		}{t, e.ID, e.Tool, input})
	case KindToolResult:
		return json.Marshal(struct {
			Type   string `json:"type"`
			ID     string `json:"id"`
			Tool   string `json:"tool"`
			Output string `json:"output"`
			Error  string `json:"error,omitempty"`
		}{t, e.ID, e.Tool, e.Output, e.ToolErr})
	case KindStepFinish:
		return json.Marshal(struct {
			Type  string `json:"type"`
			Usage Usage  `json:"usage"`
		}{t, e.Usage})
	case KindSubagentStarted:
		return json.Marshal(struct {
			Type           string `json:"type"`
			ChildSessionID string `json:"child_session_id"`
			Purpose        string `json:"purpose"`
			AgentName      string `json:"agent_name"`
		}{t, e.ChildSessionID, e.Purpose, e.AgentName})
	case KindSubagentProgress:
		return json.Marshal(struct {
			Type           string `json:"type"`
			ChildSessionID string `json:"child_session_id"`
			ProgressKind   string `json:"progress_kind"`
			ProgressText   string `json:"progress_text"`
		}{t, e.ChildSessionID, e.ProgressKind, e.ProgressText})
	case KindSubagentFinished:
		return json.Marshal(struct {
			Type             string `json:"type"`
			ChildSessionID   string `json:"child_session_id"`
			DelegationStatus string `json:"delegation_status"`
			Summary          string `json:"summary"`
		}{t, e.ChildSessionID, e.DelegationStatus, e.Summary})
	case KindMemoryInjected:
		return json.Marshal(struct {
			Type     string       `json:"type"`
			Memories []MemoryWire `json:"memories"`
		}{t, e.Memories})
	case KindMemoryUpdated:
		return json.Marshal(struct {
			Type    string             `json:"type"`
			Changes []MemoryChangeWire `json:"changes"`
		}{t, e.MemoryChanges})
	case KindTurnStatus:
		out := struct {
			Type    string `json:"type"`
			Phase   string `json:"phase"`
			Message string `json:"message,omitempty"`
		}{Type: t, Phase: string(e.Phase)}
		if strings.TrimSpace(e.StatusMessage) != "" {
			out.Message = e.StatusMessage
		}
		return json.Marshal(out)
	case KindTurnRecover:
		return json.Marshal(struct {
			Type           string `json:"type"`
			TextChars      int    `json:"text_chars"`
			ReasoningChars int    `json:"reasoning_chars"`
		}{t, e.TextChars, e.ReasoningChars})
	case KindError:
		return json.Marshal(struct {
			Type    string `json:"type"`
			Message string `json:"message"`
			Code    string `json:"code,omitempty"`
		}{t, e.Message, e.Code})
	default:
		return json.Marshal(struct {
			Type string `json:"type"`
		}{t})
	}
}

// TextDelta builds a text_delta event.
func TextDelta(delta string) Event { return Event{Kind: KindTextDelta, Delta: delta} }

// ReasoningStart builds a reasoning_start event.
func ReasoningStart() Event { return Event{Kind: KindReasoningStart} }

// ReasoningDelta builds a reasoning_delta event.
func ReasoningDelta(text string) Event { return Event{Kind: KindReasoningDelta, Text: text} }

// ToolCall builds a tool_call event. input is JSON object bytes.
func ToolCall(id, tool string, input []byte) Event {
	return Event{Kind: KindToolCall, ID: id, Tool: tool, Input: input}
}

// ToolResult builds a tool_result event. toolErr is empty on success.
func ToolResult(id, tool, output, toolErr string) Event {
	return Event{Kind: KindToolResult, ID: id, Tool: tool, Output: output, ToolErr: toolErr}
}

// StepFinish builds a step_finish event from SDK token usage.
func StepFinish(u cometsdk.TokenUsage) Event {
	return Event{Kind: KindStepFinish, Usage: Usage{
		InputTokens:  u.InputTokens,
		OutputTokens: u.OutputTokens,
		CacheRead:    u.CacheRead,
		CacheWrite:   u.CacheWrite,
	}}
}

// Errorf builds an error event.
func Errorf(message, code string) Event {
	return Event{Kind: KindError, Message: message, Code: code}
}

// Done builds the terminal done event.
func Done() Event { return Event{Kind: KindDone} }

// SubagentStarted builds a subagent_started event.
func SubagentStarted(childSessionID, purpose, agentName string) Event {
	return Event{
		Kind:           KindSubagentStarted,
		ChildSessionID: childSessionID,
		Purpose:        purpose,
		AgentName:      agentName,
	}
}

// SubagentProgress builds a subagent_progress event.
func SubagentProgress(childSessionID, progressKind, progressText string) Event {
	return Event{
		Kind:           KindSubagentProgress,
		ChildSessionID: childSessionID,
		ProgressKind:   progressKind,
		ProgressText:   progressText,
	}
}

// MemoryInjected builds a memory_injected event.
func MemoryInjected(wire []MemoryWire) Event {
	return Event{Kind: KindMemoryInjected, Memories: wire}
}

// MemoryUpdated builds a memory_updated event.
func MemoryUpdated(changes []MemoryChangeWire) Event {
	return Event{Kind: KindMemoryUpdated, MemoryChanges: changes}
}

// TurnStatus builds a turn_status event for pre-stream activity feedback.
func TurnStatus(phase TurnPhase, message string) Event {
	return Event{Kind: KindTurnStatus, Phase: phase, StatusMessage: message}
}

// TurnRecover tells clients to discard partial rendering from a failed stream
// attempt before the same logical assistant step is retried.
func TurnRecover(textChars, reasoningChars int) Event {
	return Event{Kind: KindTurnRecover, TextChars: textChars, ReasoningChars: reasoningChars}
}

// SubagentFinished builds a subagent_finished event.
func SubagentFinished(childSessionID, status, summary string) Event {
	return Event{
		Kind:             KindSubagentFinished,
		ChildSessionID:   childSessionID,
		DelegationStatus: status,
		Summary:          summary,
	}
}
