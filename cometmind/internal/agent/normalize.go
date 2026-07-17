package agent

import (
	cometsdk "github.com/cometline/comet-sdk"
)

// HistoryDegradation describes a single class of change normalization applied to
// the session history before replaying it to a provider. It is surfaced to
// callers (and ultimately the UI) so users understand why a switched provider
// may behave differently than the one that produced the history.
type HistoryDegradation struct {
	// Kind is a stable machine-readable identifier (e.g. "empty_assistant_dropped").
	Kind string
	// Count is how many history messages were affected.
	Count int
	// Message is a short human-readable explanation.
	Message string
}

// NormalizeHistory adapts session history so it can be safely replayed to any
// provider. It returns a possibly-rewritten copy of messages plus a list of
// degradations describing any lossy adaptations. The input slice is never
// mutated; messages that need no change are shared by reference.
func NormalizeHistory(messages []cometsdk.Message) ([]cometsdk.Message, []HistoryDegradation) {
	sanitized, droppedEmpty := dropEmptyAssistantMessages(messages)
	return sanitized, historyDegradations(droppedEmpty)
}

func historyDegradations(droppedEmpty int) []HistoryDegradation {
	var degradations []HistoryDegradation
	if droppedEmpty > 0 {
		degradations = append(degradations, HistoryDegradation{
			Kind:    "empty_assistant_dropped",
			Count:   droppedEmpty,
			Message: "Empty assistant turns were skipped because some providers reject replaying assistant messages without text or tool calls.",
		})
	}
	return degradations
}

func dropEmptyAssistantMessages(messages []cometsdk.Message) ([]cometsdk.Message, int) {
	dropped := 0
	for _, m := range messages {
		if isEmptyAssistantMessage(m) {
			// A prior provider may have ended the stream without yielding text,
			// reasoning, or tool calls. Replaying that row into OpenAI-compatible
			// APIs triggers 400s because assistant history must contain content.
			dropped++
		}
	}
	if dropped == 0 {
		return messages, 0
	}
	out := make([]cometsdk.Message, 0, len(messages)-dropped)
	for _, m := range messages {
		if isEmptyAssistantMessage(m) {
			continue
		}
		out = append(out, m)
	}
	return out, dropped
}

func isEmptyAssistantMessage(m cometsdk.Message) bool {
	return m.Role == cometsdk.RoleAssistant && len(m.Content) == 0 && len(m.ReasoningContent) == 0 && len(m.ProviderState) == 0
}
