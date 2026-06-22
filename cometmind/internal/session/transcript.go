package session

import (
	"context"
	"encoding/json"
	"strings"

	cometsdk "github.com/cometline/comet-sdk"
	"github.com/cometline/cometmind/internal/db"
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
)

// TranscriptEntry is a persisted message or tool row formatted for chat-style UIs.
type TranscriptEntry struct {
	Kind TranscriptKind

	Text   string         // user / assistant / reasoning body
	Images []ContentBlock // user image attachments (decoded from content envelope)

	ToolName    string
	ToolInput   string // JSON arguments
	ToolOutput  string
	ToolIsError bool

	Memories []InjectedMemory // memory rows (Kind == TranscriptKindMemory)
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

	var out []TranscriptEntry
	for _, m := range rows {
		switch m.Role {
		case "user":
			blocks, err := DecodeMessageContent(m.Content)
			if err != nil {
				out = append(out, TranscriptEntry{
					Kind: TranscriptKindUser,
					Text: m.Content,
				})
				continue
			}
			var images []ContentBlock
			for _, block := range blocks {
				if block.Type == "image" {
					images = append(images, block)
				}
			}
			out = append(out, TranscriptEntry{
				Kind:   TranscriptKindUser,
				Text:   DisplayTextFromStoredContent(m.Content),
				Images: images,
			})
		case "assistant":
			blocks, err := unmarshalReasoningContent(m.ReasoningContent)
			if err != nil {
				return nil, err
			}
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
			for _, tc := range callsByMessage[m.ID] {
				out = append(out, TranscriptEntry{
					Kind:        TranscriptKindTool,
					ToolName:    tc.ToolName,
					ToolInput:   tc.Arguments,
					ToolOutput:  trimTranscriptToolOutput(tc.Result),
					ToolIsError: toolErr[tc.ID],
				})
			}
			txt := strings.TrimSpace(m.Content)
			if txt != "" {
				out = append(out, TranscriptEntry{
					Kind: TranscriptKindAssistant,
					Text: txt,
				})
			}
		case "system":
			out = append(out, TranscriptEntry{
				Kind: TranscriptKindSystem,
				Text: strings.TrimSpace(m.Content),
			})
		case "tool_result":
			continue
		default:
			continue
		}
	}
	return out, nil
}

func trimTranscriptToolOutput(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 400 {
		return s[:400] + "…"
	}
	return s
}
