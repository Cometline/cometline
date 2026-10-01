package event

import cometsdk "github.com/Cometline/cometline/comet-sdk"

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

// MemoryCompactionCompleted builds a global completion event for manual and automatic runs.
func MemoryCompactionCompleted(before, after int64, trigger string) Event {
	return Event{
		Kind:              KindMemoryCompactionCompleted,
		MemoryCountBefore: before,
		MemoryCountAfter:  after,
		CompactionTrigger: trigger,
	}
}

// ContextBudget reports the chars/4 prompt estimate used by context compaction.
func ContextBudget(estimated, available, contextWindow int, compacted bool) Event {
	return Event{
		Kind:                KindContextBudget,
		BudgetEstimated:     estimated,
		BudgetAvailable:     available,
		BudgetContextWindow: contextWindow,
		BudgetCompacted:     compacted,
	}
}

// InboxMessageCreated builds a runtime event when the agent leaves an inbox note.
func InboxMessageCreated(id string, openCount int64) Event {
	return Event{
		Kind:           KindInboxMessageCreated,
		InboxMessageID: id,
		InboxOpenCount: openCount,
	}
}

// InboxMessageArchived builds a runtime event when a user replies or dismisses.
func InboxMessageArchived(id string, openCount int64, archiveReason string) Event {
	return Event{
		Kind:               KindInboxMessageArchived,
		InboxMessageID:     id,
		InboxOpenCount:     openCount,
		InboxArchiveReason: archiveReason,
	}
}

func RunStarted(sessionID string) Event {
	return Event{Kind: KindRunStarted, SessionID: sessionID}
}

func RunFinished(sessionID string) Event {
	return Event{Kind: KindRunFinished, SessionID: sessionID}
}

func SessionCleared(sessionID string) Event {
	return Event{Kind: KindSessionCleared, SessionID: sessionID}
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

// AssistantImage builds an assistant_image event for a presented screenshot or image.
func AssistantImage(id, mediaType, alt, dataURL string) Event {
	return Event{
		Kind:      KindAssistantImage,
		ImageID:   id,
		MediaType: mediaType,
		Alt:       alt,
		DataURL:   dataURL,
	}
}

// AssistantVideo builds an assistant_video event. Bytes are fetched from the media API.
func AssistantVideo(id, mediaType, alt string) Event {
	return Event{
		Kind:      KindAssistantVideo,
		ImageID:   id,
		MediaType: mediaType,
		Alt:       alt,
	}
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
