package session

import (
	"context"
	"encoding/json"
	"strings"

	cometsdk "github.com/Cometline/cometline/comet-sdk"
	"github.com/Cometline/cometline/cometmind/internal/db"
)

// TranscriptKind classifies one UI row in the transcript pane.
type TranscriptKind string

const (
	TranscriptKindUser      TranscriptKind = "user"
	TranscriptKindAssistant TranscriptKind = "assistant"
	TranscriptKindReasoning TranscriptKind = "reasoning"
	TranscriptKindTool      TranscriptKind = "tool"
	TranscriptKindSystem    TranscriptKind = "system"
	TranscriptKindMemory    TranscriptKind = "memory"
	TranscriptKindError     TranscriptKind = "error"
)

// Default / max page sizes for GET /sessions/:id/messages keyset pagination.
const (
	DefaultTranscriptPageLimit = 50
	MaxTranscriptPageLimit     = 200
)

// TranscriptEntry is a persisted message or tool row formatted for chat-style UIs.
type TranscriptEntry struct {
	Kind TranscriptKind

	Text     string              // user / assistant / reasoning body
	Images   []ContentBlock      // image or video attachments (user inline data or assistant media refs)
	Contexts []MessageContextRef // user-turn UI context chips (no bodies)

	ToolName    string
	ToolInput   string // JSON arguments
	ToolOutput  string
	ToolIsError bool

	Memories []InjectedMemory // memory rows (Kind == TranscriptKindMemory)
}

// TranscriptPage is one keyset window of transcript entries plus the cursor for older history.
type TranscriptPage struct {
	Items      []TranscriptEntry
	HasMore    bool
	NextBefore string // opaque keyset cursor; empty when HasMore is false
}

// LoadTranscript rebuilds an ordered transcript from SQLite using sqlc list queries.
func (s *Service) LoadTranscript(ctx context.Context, sessionID string) ([]TranscriptEntry, error) {
	rows, err := s.q.ListMessagesBySession(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	// Fetch every tool call for the session in one query (avoids an N+1 of
	// ListToolCallsByMessage per assistant message) and group by message id.
	allCalls, err := s.q.ListToolCallsBySession(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	callsByMessage := make(map[string][]db.ToolCall, len(allCalls))
	for _, tc := range allCalls {
		callsByMessage[tc.MessageID] = append(callsByMessage[tc.MessageID], tc)
	}

	return buildTranscriptEntries(rows, callsByMessage)
}

func buildTranscriptEntries(rows []db.Message, callsByMessage map[string][]db.ToolCall) ([]TranscriptEntry, error) {
	toolErr := toolResultErrorFlags(rows)

	var out []TranscriptEntry
	for _, m := range rows {
		switch m.Role {
		case "user":
			out = append(out, userTranscriptEntry(m))
		case "assistant":
			entries, err := assistantTranscriptEntries(m, callsByMessage[m.ID], toolErr)
			if err != nil {
				return nil, err
			}
			out = append(out, entries...)
		case "system":
			out = append(out, systemTranscriptEntry(m))
		}
	}
	return out, nil
}

// toolResultErrorFlags maps tool-call IDs to the is_error flag of their
// persisted tool_result rows. Undecodable rows are skipped.
func toolResultErrorFlags(rows []db.Message) map[string]bool {
	toolErr := map[string]bool{}
	for _, m := range rows {
		if m.Role != "tool_result" {
			continue
		}
		var p toolResultPayload
		if err := json.Unmarshal([]byte(m.Content), &p); err != nil {
			continue
		}
		toolErr[p.ToolCallID] = p.IsError
	}
	return toolErr
}

func userTranscriptEntry(m db.Message) TranscriptEntry {
	blocks, err := DecodeMessageContent(m.Content)
	if err != nil {
		return TranscriptEntry{
			Kind: TranscriptKindUser,
			Text: m.Content,
		}
	}
	return TranscriptEntry{
		Kind:     TranscriptKindUser,
		Text:     DisplayTextFromStoredContent(m.Content),
		Images:   mediaBlocks(blocks),
		Contexts: ContextsFromStoredContent(m.Content),
	}
}

// assistantTranscriptEntries expands one assistant row into its reasoning,
// memory, tool, and text entries, in that display order.
func assistantTranscriptEntries(m db.Message, calls []db.ToolCall, toolErr map[string]bool) ([]TranscriptEntry, error) {
	blocks, err := unmarshalReasoningContent(m.ReasoningContent)
	if err != nil {
		return nil, err
	}
	var out []TranscriptEntry
	for _, b := range blocks {
		if rb, ok := b.(cometsdk.ReasoningBlock); ok {
			rs := strings.TrimSpace(rb.Text)
			if rs != "" {
				out = append(out, TranscriptEntry{
					Kind: TranscriptKindReasoning,
					Text: rs,
				})
			}
		}
	}
	if mems := unmarshalInjectedMemories(m.InjectedMemories); len(mems) > 0 {
		out = append(out, TranscriptEntry{
			Kind:     TranscriptKindMemory,
			Memories: mems,
		})
	}
	for _, tc := range calls {
		out = append(out, TranscriptEntry{
			Kind:        TranscriptKindTool,
			ToolName:    tc.ToolName,
			ToolInput:   tc.Arguments,
			ToolOutput:  trimTranscriptToolOutput(tc.Result),
			ToolIsError: toolErr[tc.ID],
		})
	}
	if entry, ok := assistantTextEntry(m); ok {
		out = append(out, entry)
	}
	return out, nil
}

func assistantTextEntry(m db.Message) (TranscriptEntry, bool) {
	txt := strings.TrimSpace(m.Content)
	if txt == "" {
		return TranscriptEntry{}, false
	}
	entry := TranscriptEntry{Kind: TranscriptKindAssistant}
	if blocks, err := DecodeMessageContent(m.Content); err == nil && strings.HasPrefix(m.Content, contentEnvelopePrefix) {
		entry.Text = PlainTextFromContent(blocks)
		entry.Images = mediaBlocks(blocks)
	} else {
		entry.Text = txt
	}
	return entry, entry.Text != "" || len(entry.Images) > 0
}

func systemTranscriptEntry(m db.Message) TranscriptEntry {
	if text, ok := DecodeErrorMessageContent(m.Content); ok {
		return TranscriptEntry{
			Kind: TranscriptKindError,
			Text: text,
		}
	}
	return TranscriptEntry{
		Kind: TranscriptKindSystem,
		Text: strings.TrimSpace(m.Content),
	}
}

func trimTranscriptToolOutput(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 400 {
		return s[:400] + "…"
	}
	return s
}
