package session

import (
	"context"
	"encoding/json"
	"strings"

	cometsdk "github.com/cometline/comet-sdk"
)

// TranscriptKind classifies one UI row in the transcript pane.
type TranscriptKind string

const (
	TranscriptKindUser      TranscriptKind = "user"
	TranscriptKindAssistant TranscriptKind = "assistant"
	TranscriptKindReasoning TranscriptKind = "reasoning"
	TranscriptKindTool      TranscriptKind = "tool"
)

// TranscriptEntry is a persisted message or tool row formatted for chat-style UIs.
type TranscriptEntry struct {
	Kind TranscriptKind

	Text string // user / assistant / reasoning body

	ToolName    string
	ToolInput   string // JSON arguments
	ToolOutput  string
	ToolIsError bool
}

// LoadTranscript rebuilds an ordered transcript from SQLite using sqlc list queries.
func (s *Service) LoadTranscript(ctx context.Context, sessionID string) ([]TranscriptEntry, error) {
	rows, err := s.q.ListMessagesBySession(ctx, sessionID)
	if err != nil {
		return nil, err
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
			out = append(out, TranscriptEntry{
				Kind: TranscriptKindUser,
				Text: m.Content,
			})
		case "assistant":
			blocks, err := unmarshalReasoningContent(m.ReasoningContent)
			if err != nil {
				return nil, err
			}
			var reasoning strings.Builder
			for _, b := range blocks {
				if rb, ok := b.(cometsdk.ReasoningBlock); ok {
					reasoning.WriteString(rb.Text)
				}
			}
			txt := strings.TrimSpace(m.Content)
			rs := strings.TrimSpace(reasoning.String())
			if txt != "" {
				out = append(out, TranscriptEntry{
					Kind: TranscriptKindAssistant,
					Text: txt,
				})
			}
			if rs != "" {
				out = append(out, TranscriptEntry{
					Kind: TranscriptKindReasoning,
					Text: rs,
				})
			}
			tcs, err := s.q.ListToolCallsByMessage(ctx, m.ID)
			if err != nil {
				return nil, err
			}
			for _, tc := range tcs {
				out = append(out, TranscriptEntry{
					Kind:        TranscriptKindTool,
					ToolName:    tc.ToolName,
					ToolInput:   tc.Arguments,
					ToolOutput:  trimTranscriptToolOutput(tc.Result),
					ToolIsError: toolErr[tc.ID],
				})
			}
		case "tool_result", "system":
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
