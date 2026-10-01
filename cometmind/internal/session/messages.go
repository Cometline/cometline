package session

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	cometsdk "github.com/Cometline/cometline/comet-sdk"
	"github.com/Cometline/cometline/cometmind/internal/db"
	"github.com/Cometline/cometline/cometmind/internal/id"
)

// AppendSystemMessage persists a system notice in the transcript.
func (s *Service) AppendSystemMessage(ctx context.Context, sessionID, text string) (Message, error) {
	msg, err := s.createMessage(ctx, db.CreateMessageParams{
		ID:         id.New(),
		SessionID:  sessionID,
		Role:       "system",
		Content:    text,
		TokenCount: 0,
	})
	if err != nil {
		return Message{}, err
	}
	if err := s.q.TouchSession(ctx, sessionID); err != nil {
		return Message{}, err
	}
	return messageFromDB(msg), nil
}

// AppendErrorMessage persists a turn-level error notice for transcript replay.
func (s *Service) AppendErrorMessage(ctx context.Context, sessionID, text string) (Message, error) {
	raw, err := marshalErrorMessageContent(text)
	if err != nil {
		return Message{}, err
	}
	return s.AppendSystemMessage(ctx, sessionID, raw)
}

// AppendUserMessage persists a user turn.
func (s *Service) AppendUserMessage(ctx context.Context, sessionID, text string) (Message, error) {
	return s.AppendUserMessageContent(ctx, sessionID, []ContentBlock{{Type: "text", Text: text}}, "", nil)
}

// AppendUserMessageContent persists a user turn with text and optional image blocks.
// When displayText is set, transcript UIs show it instead of the agent-facing text.
// Contexts are slim UI refs (no bodies) that survive transcript reload as chips.
func (s *Service) AppendUserMessageContent(ctx context.Context, sessionID string, blocks []ContentBlock, displayText string, contexts []MessageContextRef) (Message, error) {
	content, err := marshalMessageContent(blocks, displayText, contexts)
	if err != nil {
		return Message{}, err
	}
	msg, err := s.createMessage(ctx, db.CreateMessageParams{
		ID:         id.New(),
		SessionID:  sessionID,
		Role:       "user",
		Content:    content,
		TokenCount: 0,
	})
	if err != nil {
		return Message{}, err
	}
	return messageFromDB(msg), nil
}

// AppendUserMessageAndMaybeTitle persists a user turn and, if the session
// title is still empty, sets it to the first 80 characters of the message.
// This is the single place the first-turn title rule lives.
func (s *Service) AppendUserMessageAndMaybeTitle(ctx context.Context, sessionID, text string) (Message, error) {
	msg, err := s.AppendUserMessage(ctx, sessionID, text)
	if err != nil {
		return Message{}, err
	}
	title := text
	if len(title) > 80 {
		title = title[:80] + "…"
	}
	if err := s.SetTitleIfEmpty(ctx, sessionID, title); err != nil {
		return Message{}, err
	}
	return msg, nil
}

// AppendAssistantStep persists assistant text and tool call shells (before execution).
// It returns a mapping from provider-emitted tool call ids to persisted CometMind ids.
// injectedMemories, when non-empty, are persisted alongside the assistant message
// so the memory card can be rebuilt when the session is reloaded.
func (s *Service) AppendAssistantStep(ctx context.Context, sessionID string, text string, reasoningBlocks []cometsdk.Block, toolCalls []cometsdk.ToolCallBlock, injectedMemories []InjectedMemory) (Message, map[string]string, error) {
	reasoningJSON, err := marshalReasoningContent(reasoningBlocks)
	if err != nil {
		return Message{}, nil, fmt.Errorf("marshal reasoning: %w", err)
	}
	memoriesJSON, err := marshalInjectedMemories(injectedMemories)
	if err != nil {
		return Message{}, nil, fmt.Errorf("marshal injected memories: %w", err)
	}
	assistant, err := s.createMessage(ctx, db.CreateMessageParams{
		ID:               id.New(),
		SessionID:        sessionID,
		Role:             "assistant",
		Content:          text,
		ReasoningContent: reasoningJSON,
		InjectedMemories: memoriesJSON,
		TokenCount:       0,
	})
	if err != nil {
		return Message{}, nil, err
	}
	toolIDs := make(map[string]string, len(toolCalls))
	for _, tc := range toolCalls {
		args := string(tc.Input)
		if args == "" {
			args = "{}"
		}
		persistedID := id.New()
		if _, err := s.q.CreateToolCall(ctx, db.CreateToolCallParams{
			ID:         persistedID,
			MessageID:  assistant.ID,
			ToolName:   tc.Name,
			Arguments:  args,
			Result:     "",
			DurationMs: 0,
			ExitCode:   sqlNullInt(nil),
		}); err != nil {
			return Message{}, nil, err
		}
		toolIDs[tc.ID] = persistedID
	}
	if err := s.q.TouchSession(ctx, sessionID); err != nil {
		return Message{}, nil, err
	}
	return messageFromDB(assistant), toolIDs, nil
}

// SaveAssistantProviderState stores opaque provider continuation state outside
// transcript rows so it can never be returned by transcript APIs.
func (s *Service) SaveAssistantProviderState(ctx context.Context, messageID string, states []cometsdk.ProviderState) error {
	for _, state := range states {
		if state.ProviderID == "" || state.ModelID == "" || state.Data == "" {
			continue
		}
		if err := s.q.CreateAssistantProviderState(ctx, db.CreateAssistantProviderStateParams{
			MessageID:  messageID,
			ProviderID: state.ProviderID,
			ModelID:    state.ModelID,
			State:      state.Data,
		}); err != nil {
			return err
		}
	}
	return nil
}

// ClearAssistantProviderState drops opaque continuation data after compaction.
func (s *Service) ClearAssistantProviderState(ctx context.Context, sessionID string) error {
	return s.q.DeleteAssistantProviderStatesBySession(ctx, sessionID)
}

// AppendToolResultMessage persists a tool result turn referenced by tool call id.
func (s *Service) AppendToolResultMessage(ctx context.Context, sessionID, toolCallID, output string, isErr bool) (Message, error) {
	payload := toolResultPayload{
		ToolCallID: toolCallID,
		Content:    output,
		IsError:    isErr,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return Message{}, err
	}
	msg, err := s.createMessage(ctx, db.CreateMessageParams{
		ID:         id.New(),
		SessionID:  sessionID,
		Role:       "tool_result",
		Content:    string(raw),
		TokenCount: 0,
	})
	if err != nil {
		return Message{}, err
	}
	if err := s.q.TouchSession(ctx, sessionID); err != nil {
		return Message{}, err
	}
	return messageFromDB(msg), nil
}

// UpdateToolCallResult updates execution metadata on a persisted tool call row.
func (s *Service) UpdateToolCallResult(ctx context.Context, toolCallID, result string, durMs int64, exit *int64) error {
	return s.q.UpdateToolCallResult(ctx, db.UpdateToolCallResultParams{
		ID:         toolCallID,
		Result:     result,
		DurationMs: durMs,
		ExitCode:   sqlNullInt(exit),
	})
}

// MarkToolCallsCompacted records that tool outputs should be stubbed in prompts.
func (s *Service) MarkToolCallsCompacted(ctx context.Context, ids []string, compactedAt int64) error {
	ts := sql.NullInt64{Int64: compactedAt, Valid: true}
	for _, id := range ids {
		if err := s.q.MarkToolCallCompacted(ctx, db.MarkToolCallCompactedParams{
			CompactedAt: ts,
			ID:          id,
		}); err != nil {
			return err
		}
	}
	return nil
}
