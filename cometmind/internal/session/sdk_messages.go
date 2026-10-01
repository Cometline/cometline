package session

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	cometsdk "github.com/Cometline/cometline/comet-sdk"
	"github.com/Cometline/cometline/cometmind/internal/db"
)

func sdkBlocksFromContent(blocks []ContentBlock) []cometsdk.Block {
	out := make([]cometsdk.Block, 0, len(blocks))
	for _, b := range blocks {
		switch b.Type {
		case "text":
			if b.Text != "" {
				out = append(out, cometsdk.TextBlock{Text: b.Text})
			}
		case "image":
			out = append(out, cometsdk.ImageBlock{MediaType: b.MediaType, Data: b.Data})
		}
	}
	return out
}

// BuildSDKMessages reconstructs provider-neutral messages from SQLite for the next LLM request.
func (s *Service) BuildSDKMessages(ctx context.Context, sessionID string) ([]cometsdk.Message, error) {
	sess, err := s.GetSession(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	rows, err := s.q.ListMessagesBySession(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	rows = FilterMessagesAfterCompacted(rows, sess.CompactedUntilMessageID)
	return s.buildSDKMessagesFromRows(ctx, sessionID, sess.ProviderID, rows)
}

// ListMessageRows returns raw persisted transcript rows in chronological order.
func (s *Service) ListMessageRows(ctx context.Context, sessionID string) ([]db.Message, error) {
	return s.q.ListMessagesBySession(ctx, sessionID)
}

// BuildSDKMessagesAll rebuilds the full transcript without compaction filtering.
func (s *Service) BuildSDKMessagesAll(ctx context.Context, sessionID string) ([]cometsdk.Message, error) {
	sess, err := s.GetSession(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	rows, err := s.q.ListMessagesBySession(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	return s.buildSDKMessagesFromRows(ctx, sessionID, sess.ProviderID, rows)
}

// ListToolCallsForSession returns all tool calls for a session in chronological order.
func (s *Service) ListToolCallsForSession(ctx context.Context, sessionID string) ([]db.ToolCall, error) {
	return s.q.ListToolCallsBySession(ctx, sessionID)
}

// GroupToolCallsByMessage indexes tool calls by assistant message id.
func GroupToolCallsByMessage(calls []db.ToolCall) map[string][]db.ToolCall {
	out := make(map[string][]db.ToolCall, len(calls))
	for _, tc := range calls {
		out[tc.MessageID] = append(out[tc.MessageID], tc)
	}
	return out
}

func (s *Service) buildSDKMessagesFromRows(ctx context.Context, sessionID, providerID string, rows []db.Message) ([]cometsdk.Message, error) {
	// One query for all tool calls instead of one per assistant message.
	allCalls, err := s.q.ListToolCallsBySession(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	completedToolCalls, err := completedToolCallIDs(rows)
	if err != nil {
		return nil, err
	}
	callsByMessage := make(map[string][]db.ToolCall, len(allCalls))
	compactedIDs := make(map[string]struct{})
	for _, tc := range allCalls {
		callsByMessage[tc.MessageID] = append(callsByMessage[tc.MessageID], tc)
		if tc.CompactedAt.Valid {
			compactedIDs[tc.ID] = struct{}{}
		}
	}
	states, err := s.q.ListAssistantProviderStatesBySession(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	statesByMessage := providerStatesByMessage(states, providerID)
	out := make([]cometsdk.Message, 0, len(rows))
	for _, m := range rows {
		switch m.Role {
		case "user":
			blocks, err := DecodeMessageContent(m.Content)
			if err != nil {
				return nil, fmt.Errorf("decode user content %s: %w", m.ID, err)
			}
			out = append(out, cometsdk.Message{
				Role:    cometsdk.RoleUser,
				Content: sdkBlocksFromContent(blocks),
			})
		case "assistant":
			blocks := assistantBlocks(m, callsByMessage[m.ID], completedToolCalls)
			reasoningBlocks, err := unmarshalReasoningContent(m.ReasoningContent)
			if err != nil {
				return nil, fmt.Errorf("decode reasoning_content %s: %w", m.ID, err)
			}
			out = append(out, cometsdk.Message{
				Role:             cometsdk.RoleAssistant,
				Content:          blocks,
				ReasoningContent: reasoningBlocks,
				ProviderState:    statesByMessage[m.ID],
			})
		case "tool_result":
			msg, err := toolResultSDKMessage(m, compactedIDs)
			if err != nil {
				return nil, err
			}
			out = append(out, msg)
		case "system":
			// Stored system rows are optional; the live system prompt comes from the agent.
			continue
		default:
			return nil, fmt.Errorf("unknown message role %q", m.Role)
		}
	}
	return out, nil
}

// providerStatesByMessage indexes continuation state by assistant message id,
// keeping only rows written by providerID.
func providerStatesByMessage(states []db.ListAssistantProviderStatesBySessionRow, providerID string) map[string][]cometsdk.ProviderState {
	out := make(map[string][]cometsdk.ProviderState, len(states))
	for _, state := range states {
		if state.ProviderID != providerID {
			continue
		}
		out[state.MessageID] = append(out[state.MessageID], cometsdk.ProviderState{
			ProviderID: state.ProviderID,
			ModelID:    state.ModelID,
			Data:       state.State,
		})
	}
	return out
}

func toolResultSDKMessage(m db.Message, compactedIDs map[string]struct{}) (cometsdk.Message, error) {
	var p toolResultPayload
	if err := json.Unmarshal([]byte(m.Content), &p); err != nil {
		return cometsdk.Message{}, fmt.Errorf("decode tool_result %s: %w", m.ID, err)
	}
	_, compacted := compactedIDs[p.ToolCallID]
	return cometsdk.Message{
		Role: cometsdk.RoleToolResult,
		Content: []cometsdk.Block{
			cometsdk.ToolResultBlock{
				ToolCallID: p.ToolCallID,
				Content:    ToolResultPromptContent(p.Content, compacted),
				IsError:    p.IsError,
			},
		},
	}, nil
}

func completedToolCallIDs(rows []db.Message) (map[string]struct{}, error) {
	completed := make(map[string]struct{})
	for _, m := range rows {
		if m.Role != "tool_result" {
			continue
		}
		var p toolResultPayload
		if err := json.Unmarshal([]byte(m.Content), &p); err != nil {
			return nil, fmt.Errorf("decode tool_result %s: %w", m.ID, err)
		}
		completed[p.ToolCallID] = struct{}{}
	}
	return completed, nil
}

func assistantBlocks(m db.Message, tcs []db.ToolCall, completedToolCalls map[string]struct{}) []cometsdk.Block {
	var blocks []cometsdk.Block
	if strings.TrimSpace(m.Content) != "" {
		text := m.Content
		if decoded, err := DecodeMessageContent(m.Content); err == nil {
			text = PlainTextFromContent(decoded)
		}
		if strings.TrimSpace(text) != "" {
			blocks = append(blocks, cometsdk.TextBlock{Text: text})
		}
	}
	for _, tc := range tcs {
		if _, ok := completedToolCalls[tc.ID]; !ok {
			// A cancelled turn may have persisted the call before its result. Do not
			// replay that incomplete protocol pair to any provider.
			continue
		}
		raw := json.RawMessage(tc.Arguments)
		if len(raw) == 0 {
			raw = json.RawMessage("{}")
		}
		blocks = append(blocks, cometsdk.ToolCallBlock{
			ID:    tc.ID,
			Name:  tc.ToolName,
			Input: raw,
		})
	}
	return blocks
}
