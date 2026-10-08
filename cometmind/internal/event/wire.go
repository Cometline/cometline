package event

import (
	"encoding/json"
	"fmt"
	"strings"
)

// MarshalJSON renders the SSE wire frame body for this event. The output is the
// authoritative wire contract consumed by the cometline frontend.
func (e Event) MarshalJSON() ([]byte, error) {
	if payload, ok := wirePayloads[e.Kind]; ok {
		return json.Marshal(payload(e))
	}
	return json.Marshal(struct {
		Type string `json:"type"`
	}{string(e.Kind)})
}

// wirePayloads maps each Kind to the small typed struct it marshals through, so
// "type" stays first and field order is deterministic (a map would sort keys
// alphabetically and reorder the wire). Kinds without an entry carry only "type".
var wirePayloads = map[Kind]func(Event) any{
	KindTextDelta: func(e Event) any {
		return struct {
			Type  string `json:"type"`
			Delta string `json:"delta"`
		}{string(e.Kind), e.Delta}
	},
	KindReasoningDelta: func(e Event) any {
		return struct {
			Type string `json:"type"`
			Text string `json:"text"`
		}{string(e.Kind), e.Text}
	},
	KindToolCall:   toolCallPayload,
	KindToolResult: toolResultPayload,
	KindStepFinish: func(e Event) any {
		return struct {
			Type  string `json:"type"`
			Usage Usage  `json:"usage"`
		}{string(e.Kind), e.Usage}
	},
	KindSubagentStarted:           subagentStartedPayload,
	KindSubagentProgress:          subagentProgressPayload,
	KindSubagentFinished:          subagentFinishedPayload,
	KindMemoryInjected:            memoryInjectedPayload,
	KindMemoryUpdated:             memoryUpdatedPayload,
	KindSkillReviewUpdated:        skillReviewUpdatedPayload,
	KindWikiReviewUpdated:         wikiReviewUpdatedPayload,
	KindMemoryCompactionCompleted: memoryCompactionPayload,
	KindContextBudget:             contextBudgetPayload,
	KindInboxMessageCreated:       inboxCreatedPayload,
	KindInboxMessageArchived:      inboxArchivedPayload,
	KindRunStarted:                sessionPayload,
	KindRunFinished:               sessionPayload,
	KindSessionCleared:            sessionPayload,
	KindTurnStatus:                turnStatusPayload,
	KindTurnRecover:               turnRecoverPayload,
	KindAssistantImage:            assistantMediaPayload,
	KindAssistantVideo:            assistantMediaPayload,
	KindError:                     errorPayload,
}

func toolCallPayload(e Event) any {
	input := json.RawMessage(e.Input)
	if len(input) == 0 {
		input = json.RawMessage("{}")
	}
	return struct {
		Type  string          `json:"type"`
		ID    string          `json:"id"`
		Tool  string          `json:"tool"`
		Input json.RawMessage `json:"input"`
	}{string(e.Kind), e.ID, e.Tool, input}
}

func toolResultPayload(e Event) any {
	return struct {
		Type   string `json:"type"`
		ID     string `json:"id"`
		Tool   string `json:"tool"`
		Output string `json:"output"`
		Error  string `json:"error,omitempty"`
	}{string(e.Kind), e.ID, e.Tool, e.Output, e.ToolErr}
}

func subagentStartedPayload(e Event) any {
	return struct {
		Type           string `json:"type"`
		ChildSessionID string `json:"child_session_id"`
		Purpose        string `json:"purpose"`
		AgentName      string `json:"agent_name"`
	}{string(e.Kind), e.ChildSessionID, e.Purpose, e.AgentName}
}

func subagentProgressPayload(e Event) any {
	return struct {
		Type           string `json:"type"`
		ChildSessionID string `json:"child_session_id"`
		ProgressKind   string `json:"progress_kind"`
		ProgressText   string `json:"progress_text"`
	}{string(e.Kind), e.ChildSessionID, e.ProgressKind, e.ProgressText}
}

func subagentFinishedPayload(e Event) any {
	return struct {
		Type             string `json:"type"`
		ChildSessionID   string `json:"child_session_id"`
		DelegationStatus string `json:"delegation_status"`
		Summary          string `json:"summary"`
	}{string(e.Kind), e.ChildSessionID, e.DelegationStatus, e.Summary}
}

func memoryInjectedPayload(e Event) any {
	return struct {
		Type     string       `json:"type"`
		Memories []MemoryWire `json:"memories"`
	}{string(e.Kind), e.Memories}
}

func memoryUpdatedPayload(e Event) any {
	return struct {
		Type    string             `json:"type"`
		Changes []MemoryChangeWire `json:"changes"`
	}{string(e.Kind), e.MemoryChanges}
}

func wikiReviewUpdatedPayload(e Event) any {
	paths := e.WikiPaths
	if paths == nil {
		paths = []string{}
	}
	return struct {
		Type      string   `json:"type"`
		SessionID string   `json:"session_id"`
		Paths     []string `json:"paths"`
	}{string(e.Kind), e.SessionID, paths}
}

func skillReviewUpdatedPayload(e Event) any {
	skills := e.SkillReviews
	if skills == nil {
		skills = []SkillReviewChange{}
	}
	return struct {
		Type      string              `json:"type"`
		SessionID string              `json:"session_id"`
		Skills    []SkillReviewChange `json:"skills"`
	}{string(e.Kind), e.SessionID, skills}
}

func memoryCompactionPayload(e Event) any {
	return struct {
		Type    string `json:"type"`
		Before  int64  `json:"before"`
		After   int64  `json:"after"`
		Trigger string `json:"trigger"`
	}{string(e.Kind), e.MemoryCountBefore, e.MemoryCountAfter, e.CompactionTrigger}
}

func contextBudgetPayload(e Event) any {
	return struct {
		Type          string `json:"type"`
		Estimated     int    `json:"estimated"`
		Available     int    `json:"available"`
		ContextWindow int    `json:"context_window"`
		Compacted     bool   `json:"compacted,omitempty"`
	}{string(e.Kind), e.BudgetEstimated, e.BudgetAvailable, e.BudgetContextWindow, e.BudgetCompacted}
}

func inboxCreatedPayload(e Event) any {
	return struct {
		Type      string `json:"type"`
		ID        string `json:"id"`
		OpenCount int64  `json:"open_count"`
	}{string(e.Kind), e.InboxMessageID, e.InboxOpenCount}
}

func inboxArchivedPayload(e Event) any {
	return struct {
		Type          string `json:"type"`
		ID            string `json:"id"`
		OpenCount     int64  `json:"open_count"`
		ArchiveReason string `json:"archive_reason"`
	}{string(e.Kind), e.InboxMessageID, e.InboxOpenCount, e.InboxArchiveReason}
}

func sessionPayload(e Event) any {
	return struct {
		Type      string `json:"type"`
		SessionID string `json:"session_id"`
	}{string(e.Kind), e.SessionID}
}

func turnStatusPayload(e Event) any {
	out := struct {
		Type    string `json:"type"`
		Phase   string `json:"phase"`
		Message string `json:"message,omitempty"`
	}{Type: string(e.Kind), Phase: string(e.Phase)}
	if strings.TrimSpace(e.StatusMessage) != "" {
		out.Message = e.StatusMessage
	}
	return out
}

func turnRecoverPayload(e Event) any {
	return struct {
		Type           string `json:"type"`
		TextChars      int    `json:"text_chars"`
		ReasoningChars int    `json:"reasoning_chars"`
	}{string(e.Kind), e.TextChars, e.ReasoningChars}
}

func assistantMediaPayload(e Event) any {
	return struct {
		Type      string `json:"type"`
		ID        string `json:"id"`
		MediaType string `json:"media_type"`
		Alt       string `json:"alt,omitempty"`
		DataURL   string `json:"data_url,omitempty"`
	}{string(e.Kind), e.ImageID, e.MediaType, e.Alt, e.DataURL}
}

func errorPayload(e Event) any {
	return struct {
		Type    string `json:"type"`
		Message string `json:"message"`
		Code    string `json:"code,omitempty"`
	}{string(e.Kind), e.Message, e.Code}
}

// eventWire is the union of every field any Kind puts on the wire. Several
// Kinds share a JSON name (e.g. "id", "message"), so decoding fans each shared
// field out to every Event field that marshals from it.
type eventWire struct {
	Type             string              `json:"type"`
	Delta            string              `json:"delta"`
	Text             string              `json:"text"`
	ID               string              `json:"id"`
	Tool             string              `json:"tool"`
	Input            json.RawMessage     `json:"input"`
	Output           string              `json:"output"`
	Error            string              `json:"error"`
	Usage            Usage               `json:"usage"`
	ChildSessionID   string              `json:"child_session_id"`
	Purpose          string              `json:"purpose"`
	AgentName        string              `json:"agent_name"`
	ProgressKind     string              `json:"progress_kind"`
	ProgressText     string              `json:"progress_text"`
	DelegationStatus string              `json:"delegation_status"`
	Summary          string              `json:"summary"`
	Memories         []MemoryWire        `json:"memories"`
	Changes          []MemoryChangeWire  `json:"changes"`
	Skills           []SkillReviewChange `json:"skills"`
	Paths            []string            `json:"paths"`
	Before           int64               `json:"before"`
	After            int64               `json:"after"`
	Trigger          string              `json:"trigger"`
	Estimated        int                 `json:"estimated"`
	Available        int                 `json:"available"`
	ContextWindow    int                 `json:"context_window"`
	Compacted        bool                `json:"compacted"`
	OpenCount        int64               `json:"open_count"`
	ArchiveReason    string              `json:"archive_reason"`
	SessionID        string              `json:"session_id"`
	Phase            TurnPhase           `json:"phase"`
	Message          string              `json:"message"`
	TextChars        int                 `json:"text_chars"`
	ReasoningChars   int                 `json:"reasoning_chars"`
	MediaType        string              `json:"media_type"`
	Alt              string              `json:"alt"`
	DataURL          string              `json:"data_url"`
	Code             string              `json:"code"`
}

// UnmarshalJSON decodes the same discriminated wire shape emitted by MarshalJSON.
// It is used by the localhost gateway-to-serve event bridge.
func (e *Event) UnmarshalJSON(data []byte) error {
	var wire eventWire
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	if strings.TrimSpace(wire.Type) == "" {
		return fmt.Errorf("event type is required")
	}
	*e = wire.event()
	return nil
}

func (w eventWire) event() Event {
	return Event{
		Kind:                Kind(w.Type),
		Delta:               w.Delta,
		Text:                w.Text,
		ID:                  w.ID,
		Tool:                w.Tool,
		Input:               append([]byte(nil), w.Input...),
		Output:              w.Output,
		ToolErr:             w.Error,
		Usage:               w.Usage,
		ChildSessionID:      w.ChildSessionID,
		Purpose:             w.Purpose,
		AgentName:           w.AgentName,
		ProgressKind:        w.ProgressKind,
		ProgressText:        w.ProgressText,
		DelegationStatus:    w.DelegationStatus,
		Summary:             w.Summary,
		Memories:            w.Memories,
		MemoryChanges:       w.Changes,
		SkillReviews:        w.Skills,
		WikiPaths:           w.Paths,
		MemoryCountBefore:   w.Before,
		MemoryCountAfter:    w.After,
		CompactionTrigger:   w.Trigger,
		BudgetEstimated:     w.Estimated,
		BudgetAvailable:     w.Available,
		BudgetContextWindow: w.ContextWindow,
		BudgetCompacted:     w.Compacted,
		InboxMessageID:      w.ID,
		InboxOpenCount:      w.OpenCount,
		InboxArchiveReason:  w.ArchiveReason,
		SessionID:           w.SessionID,
		Phase:               w.Phase,
		StatusMessage:       w.Message,
		TextChars:           w.TextChars,
		ReasoningChars:      w.ReasoningChars,
		ImageID:             w.ID,
		MediaType:           w.MediaType,
		Alt:                 w.Alt,
		DataURL:             w.DataURL,
		Message:             w.Message,
		Code:                w.Code,
	}
}
