package session

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
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

// LoadTranscriptPage returns a recent (or older-than-before) window of transcript
// entries using keyset pagination on (created_at, id). limit is clamped to
// [1, MaxTranscriptPageLimit]; zero/negative uses DefaultTranscriptPageLimit.
// before is an opaque cursor from a prior page's NextBefore; empty means the
// most recent page.
func (s *Service) LoadTranscriptPage(ctx context.Context, sessionID string, limit int, before string) (TranscriptPage, error) {
	if limit <= 0 {
		limit = DefaultTranscriptPageLimit
	}
	if limit > MaxTranscriptPageLimit {
		limit = MaxTranscriptPageLimit
	}
	before = strings.TrimSpace(before)

	fetch := int64(limit) + 1 // one extra to detect HasMore
	var (
		descRows []db.Message
		err      error
		beforeCreatedAt int64
		beforeID        string
	)
	if before == "" {
		descRows, err = s.q.ListTranscriptMessagesRecent(ctx, db.ListTranscriptMessagesRecentParams{
			SessionID: sessionID,
			RowLimit:  fetch,
		})
	} else {
		var decErr error
		beforeCreatedAt, beforeID, decErr = decodeTranscriptCursor(before)
		if decErr != nil {
			return TranscriptPage{}, fmt.Errorf("%w: %v", ErrInvalidTranscriptCursor, decErr)
		}
		descRows, err = s.q.ListTranscriptMessagesBefore(ctx, db.ListTranscriptMessagesBeforeParams{
			SessionID:       sessionID,
			BeforeCreatedAt: beforeCreatedAt,
			BeforeID:        beforeID,
			RowLimit:        fetch,
		})
	}
	if err != nil {
		return TranscriptPage{}, err
	}

	hasMore := len(descRows) > limit
	if hasMore {
		descRows = descRows[:limit]
	}
	if len(descRows) == 0 {
		return TranscriptPage{Items: []TranscriptEntry{}, HasMore: false}, nil
	}

	// descRows are newest-first; reverse to chronological ASC for the UI.
	rows := reverseMessages(descRows)

	messageIDs := make([]string, 0, len(rows))
	for _, m := range rows {
		messageIDs = append(messageIDs, m.ID)
	}

	var allCalls []db.ToolCall
	if len(messageIDs) > 0 {
		allCalls, err = s.q.ListToolCallsByMessageIDs(ctx, messageIDs)
		if err != nil {
			return TranscriptPage{}, err
		}
	}
	callsByMessage := make(map[string][]db.ToolCall, len(allCalls))
	for _, tc := range allCalls {
		callsByMessage[tc.MessageID] = append(callsByMessage[tc.MessageID], tc)
	}

	// Tool-result rows for error flags fall between the oldest visible message
	// and the before-cursor (or unbounded for the recent page).
	var toolResults []db.Message
	if before == "" {
		toolResults, err = s.q.ListToolResultMessagesAfter(ctx, db.ListToolResultMessagesAfterParams{
			SessionID:    sessionID,
			MinCreatedAt: rows[0].CreatedAt,
		})
	} else {
		toolResults, err = s.q.ListToolResultMessagesBetween(ctx, db.ListToolResultMessagesBetweenParams{
			SessionID:    sessionID,
			MinCreatedAt: rows[0].CreatedAt,
			MaxCreatedAt: beforeCreatedAt,
			MaxID:        beforeID,
		})
	}
	if err != nil {
		return TranscriptPage{}, err
	}

	// Merge tool_results into the row set so buildTranscriptEntries can map errors.
	// They are skipped as visible rows but populate toolErr.
	merged := mergeMessagesChronological(rows, toolResults)

	items, err := buildTranscriptEntries(merged, callsByMessage)
	if err != nil {
		return TranscriptPage{}, err
	}

	page := TranscriptPage{Items: items, HasMore: hasMore}
	if hasMore {
		// Oldest visible message in this page — client passes it as before for older history.
		oldest := rows[0]
		page.NextBefore = encodeTranscriptCursor(oldest.CreatedAt, oldest.ID)
	}
	return page, nil
}

// ErrInvalidTranscriptCursor is returned when before cannot be parsed.
var ErrInvalidTranscriptCursor = errors.New("invalid transcript cursor")

func encodeTranscriptCursor(createdAt int64, id string) string {
	return fmt.Sprintf("%d:%s", createdAt, id)
}

func decodeTranscriptCursor(raw string) (createdAt int64, id string, err error) {
	raw = strings.TrimSpace(raw)
	idx := strings.IndexByte(raw, ':')
	if idx <= 0 || idx == len(raw)-1 {
		return 0, "", fmt.Errorf("want created_at:id")
	}
	createdAt, err = strconv.ParseInt(raw[:idx], 10, 64)
	if err != nil {
		return 0, "", err
	}
	id = raw[idx+1:]
	if id == "" {
		return 0, "", fmt.Errorf("empty id")
	}
	return createdAt, id, nil
}




func reverseMessages(in []db.Message) []db.Message {
	out := make([]db.Message, len(in))
	for i := range in {
		out[len(in)-1-i] = in[i]
	}
	return out
}

func mergeMessagesChronological(visible, toolResults []db.Message) []db.Message {
	if len(toolResults) == 0 {
		return visible
	}
	out := make([]db.Message, 0, len(visible)+len(toolResults))
	i, j := 0, 0
	for i < len(visible) && j < len(toolResults) {
		a, b := visible[i], toolResults[j]
		if a.CreatedAt < b.CreatedAt || (a.CreatedAt == b.CreatedAt && a.ID <= b.ID) {
			out = append(out, a)
			i++
		} else {
			out = append(out, b)
			j++
		}
	}
	out = append(out, visible[i:]...)
	out = append(out, toolResults[j:]...)
	return out
}

func buildTranscriptEntries(rows []db.Message, callsByMessage map[string][]db.ToolCall) ([]TranscriptEntry, error) {
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
				if block.Type == "image" || block.Type == "video" {
					images = append(images, block)
				}
			}
			out = append(out, TranscriptEntry{
				Kind:     TranscriptKindUser,
				Text:     DisplayTextFromStoredContent(m.Content),
				Images:   images,
				Contexts: ContextsFromStoredContent(m.Content),
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
				entry := TranscriptEntry{Kind: TranscriptKindAssistant}
				if blocks, err := DecodeMessageContent(m.Content); err == nil && strings.HasPrefix(m.Content, contentEnvelopePrefix) {
					var images []ContentBlock
					for _, block := range blocks {
						if block.Type == "image" || block.Type == "video" {
							images = append(images, block)
						}
					}
					entry.Text = PlainTextFromContent(blocks)
					entry.Images = images
				} else {
					entry.Text = txt
				}
				if entry.Text != "" || len(entry.Images) > 0 {
					out = append(out, entry)
				}
			}
		case "system":
			if text, ok := DecodeErrorMessageContent(m.Content); ok {
				out = append(out, TranscriptEntry{
					Kind: TranscriptKindError,
					Text: text,
				})
				continue
			}
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
