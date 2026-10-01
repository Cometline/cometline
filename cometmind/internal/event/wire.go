package event

import (
	"encoding/json"
	"fmt"
	"strings"
)

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
	case KindMemoryCompactionCompleted:
		return json.Marshal(struct {
			Type    string `json:"type"`
			Before  int64  `json:"before"`
			After   int64  `json:"after"`
			Trigger string `json:"trigger"`
		}{t, e.MemoryCountBefore, e.MemoryCountAfter, e.CompactionTrigger})
	case KindContextBudget:
		out := struct {
			Type          string `json:"type"`
			Estimated     int    `json:"estimated"`
			Available     int    `json:"available"`
			ContextWindow int    `json:"context_window"`
			Compacted     bool   `json:"compacted,omitempty"`
		}{
			Type:          t,
			Estimated:     e.BudgetEstimated,
			Available:     e.BudgetAvailable,
			ContextWindow: e.BudgetContextWindow,
		}
		if e.BudgetCompacted {
			out.Compacted = true
		}
		return json.Marshal(out)
	case KindInboxMessageCreated:
		return json.Marshal(struct {
			Type      string `json:"type"`
			ID        string `json:"id"`
			OpenCount int64  `json:"open_count"`
		}{t, e.InboxMessageID, e.InboxOpenCount})
	case KindInboxMessageArchived:
		return json.Marshal(struct {
			Type          string `json:"type"`
			ID            string `json:"id"`
			OpenCount     int64  `json:"open_count"`
			ArchiveReason string `json:"archive_reason"`
		}{t, e.InboxMessageID, e.InboxOpenCount, e.InboxArchiveReason})
	case KindRunStarted, KindRunFinished, KindSessionCleared:
		return json.Marshal(struct {
			Type      string `json:"type"`
			SessionID string `json:"session_id"`
		}{t, e.SessionID})
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
	case KindAssistantImage, KindAssistantVideo:
		return json.Marshal(struct {
			Type      string `json:"type"`
			ID        string `json:"id"`
			MediaType string `json:"media_type"`
			Alt       string `json:"alt,omitempty"`
			DataURL   string `json:"data_url,omitempty"`
		}{t, e.ImageID, e.MediaType, e.Alt, e.DataURL})
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

// UnmarshalJSON decodes the same discriminated wire shape emitted by MarshalJSON.
// It is used by the localhost gateway-to-serve event bridge.
func (e *Event) UnmarshalJSON(data []byte) error {
	var wire struct {
		Type             string             `json:"type"`
		Delta            string             `json:"delta"`
		Text             string             `json:"text"`
		ID               string             `json:"id"`
		Tool             string             `json:"tool"`
		Input            json.RawMessage    `json:"input"`
		Output           string             `json:"output"`
		Error            string             `json:"error"`
		Usage            Usage              `json:"usage"`
		ChildSessionID   string             `json:"child_session_id"`
		Purpose          string             `json:"purpose"`
		AgentName        string             `json:"agent_name"`
		ProgressKind     string             `json:"progress_kind"`
		ProgressText     string             `json:"progress_text"`
		DelegationStatus string             `json:"delegation_status"`
		Summary          string             `json:"summary"`
		Memories         []MemoryWire       `json:"memories"`
		Changes          []MemoryChangeWire `json:"changes"`
		Before           int64              `json:"before"`
		After            int64              `json:"after"`
		Trigger          string             `json:"trigger"`
		Estimated        int                `json:"estimated"`
		Available        int                `json:"available"`
		ContextWindow    int                `json:"context_window"`
		Compacted        bool               `json:"compacted"`
		OpenCount        int64              `json:"open_count"`
		ArchiveReason    string             `json:"archive_reason"`
		SessionID        string             `json:"session_id"`
		Phase            TurnPhase          `json:"phase"`
		Message          string             `json:"message"`
		TextChars        int                `json:"text_chars"`
		ReasoningChars   int                `json:"reasoning_chars"`
		MediaType        string             `json:"media_type"`
		Alt              string             `json:"alt"`
		DataURL          string             `json:"data_url"`
		Code             string             `json:"code"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	if strings.TrimSpace(wire.Type) == "" {
		return fmt.Errorf("event type is required")
	}
	*e = Event{
		Kind:                Kind(wire.Type),
		Delta:               wire.Delta,
		Text:                wire.Text,
		ID:                  wire.ID,
		Tool:                wire.Tool,
		Input:               append([]byte(nil), wire.Input...),
		Output:              wire.Output,
		ToolErr:             wire.Error,
		Usage:               wire.Usage,
		ChildSessionID:      wire.ChildSessionID,
		Purpose:             wire.Purpose,
		AgentName:           wire.AgentName,
		ProgressKind:        wire.ProgressKind,
		ProgressText:        wire.ProgressText,
		DelegationStatus:    wire.DelegationStatus,
		Summary:             wire.Summary,
		Memories:            wire.Memories,
		MemoryChanges:       wire.Changes,
		MemoryCountBefore:   wire.Before,
		MemoryCountAfter:    wire.After,
		CompactionTrigger:   wire.Trigger,
		BudgetEstimated:     wire.Estimated,
		BudgetAvailable:     wire.Available,
		BudgetContextWindow: wire.ContextWindow,
		BudgetCompacted:     wire.Compacted,
		InboxMessageID:      wire.ID,
		InboxOpenCount:      wire.OpenCount,
		InboxArchiveReason:  wire.ArchiveReason,
		SessionID:           wire.SessionID,
		Phase:               wire.Phase,
		StatusMessage:       wire.Message,
		TextChars:           wire.TextChars,
		ReasoningChars:      wire.ReasoningChars,
		ImageID:             wire.ID,
		MediaType:           wire.MediaType,
		Alt:                 wire.Alt,
		DataURL:             wire.DataURL,
		Message:             wire.Message,
		Code:                wire.Code,
	}
	return nil
}
