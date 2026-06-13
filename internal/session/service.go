package session

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	cometsdk "github.com/cometline/comet-sdk"
	"github.com/cometline/cometmind/internal/db"
	"github.com/cometline/cometmind/internal/id"
)

// toolResultPayload is stored in messages.content for role=tool_result.
type toolResultPayload struct {
	ToolCallID string `json:"tool_call_id"`
	Content    string `json:"content"`
	IsError    bool   `json:"is_error"`
}

// Service coordinates persistence for workspaces, sessions, messages, and tool calls.
type Service struct {
	q *db.Queries
}

// New creates a session service bound to the shared sqlc querier.
func New(sqlDB *sql.DB) *Service {
	return &Service{q: db.New(sqlDB)}
}

// EnsureWorkspace registers the absolute workspace root in the global store when missing.
func (s *Service) EnsureWorkspace(ctx context.Context, absRoot string) (db.Workspace, error) {
	clean := filepath.Clean(absRoot)
	w, err := s.q.GetWorkspaceByPath(ctx, clean)
	if err == nil {
		return w, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return db.Workspace{}, err
	}
	return s.q.CreateWorkspace(ctx, db.CreateWorkspaceParams{
		ID:   id.New(),
		Name: filepath.Base(clean),
		Path: clean,
	})
}

// GetWorkspace loads a workspace by id.
func (s *Service) GetWorkspace(ctx context.Context, workspaceID string) (db.Workspace, error) {
	return s.q.GetWorkspace(ctx, workspaceID)
}

// LookupWorkspaceByPath loads a workspace by path without creating it.
func (s *Service) LookupWorkspaceByPath(ctx context.Context, absRoot string) (db.Workspace, error) {
	return s.q.GetWorkspaceByPath(ctx, filepath.Clean(absRoot))
}

// NewSession creates a persisted session row scoped to a workspace.
func (s *Service) NewSession(ctx context.Context, workspaceID string, modelID, providerID string) (db.Session, error) {
	return s.q.CreateSession(ctx, db.CreateSessionParams{
		ID:          id.New(),
		WorkspaceID: workspaceID,
		Title:       "",
		ModelID:     modelID,
		ProviderID:  providerID,
		Status:      "active",
	})
}

// GetSession loads a session by id.
func (s *Service) GetSession(ctx context.Context, sessionID string) (db.Session, error) {
	return s.q.GetSession(ctx, sessionID)
}

// ListSessions lists sessions for a workspace ordered by recent activity.
func (s *Service) ListSessions(ctx context.Context, workspaceID string) ([]db.Session, error) {
	return s.q.ListSessionsByWorkspace(ctx, workspaceID)
}

// DeleteSession removes a session and cascades its messages and tool calls.
func (s *Service) DeleteSession(ctx context.Context, sessionID string) error {
	return s.q.DeleteSession(ctx, sessionID)
}

// WorkspacePath resolves the filesystem root for a workspace id.
func (s *Service) WorkspacePath(ctx context.Context, workspaceID string) (string, error) {
	w, err := s.q.GetWorkspace(ctx, workspaceID)
	if err != nil {
		return "", err
	}
	return w.Path, nil
}

// SetTitleIfEmpty updates session title once (used after first user turn).
func (s *Service) SetTitleIfEmpty(ctx context.Context, sessionID, title string) error {
	sess, err := s.q.GetSession(ctx, sessionID)
	if err != nil {
		return err
	}
	if strings.TrimSpace(sess.Title) != "" {
		return nil
	}
	return s.q.UpdateSessionTitle(ctx, db.UpdateSessionTitleParams{
		ID:    sessionID,
		Title: title,
	})
}

// AppendUserMessage persists a user turn.
func (s *Service) AppendUserMessage(ctx context.Context, sessionID, text string) (db.Message, error) {
	return s.q.CreateMessage(ctx, db.CreateMessageParams{
		ID:         id.New(),
		SessionID:  sessionID,
		Role:       "user",
		Content:    text,
		TokenCount: 0,
	})
}

type reasoningBlockPayload struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

func marshalReasoningContent(blocks []cometsdk.Block) (string, error) {
	var payloads []reasoningBlockPayload
	for _, b := range blocks {
		switch v := b.(type) {
		case cometsdk.TextBlock:
			payloads = append(payloads, reasoningBlockPayload{Type: "text", Text: v.Text})
		case cometsdk.ReasoningBlock:
			payloads = append(payloads, reasoningBlockPayload{Type: "reasoning", Text: v.Text})
		}
	}
	raw, err := json.Marshal(payloads)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func unmarshalReasoningContent(raw string) ([]cometsdk.Block, error) {
	var payloads []reasoningBlockPayload
	if err := json.Unmarshal([]byte(raw), &payloads); err != nil {
		return nil, err
	}
	var blocks []cometsdk.Block
	for _, p := range payloads {
		switch p.Type {
		case "text":
			blocks = append(blocks, cometsdk.TextBlock{Text: p.Text})
		case "reasoning":
			blocks = append(blocks, cometsdk.ReasoningBlock{Text: p.Text})
		}
	}
	return blocks, nil
}

// AppendAssistantStep persists assistant text and tool call shells (before execution).
// It returns a mapping from provider-emitted tool call ids to persisted CometMind ids.
func (s *Service) AppendAssistantStep(ctx context.Context, sessionID string, text string, reasoningBlocks []cometsdk.Block, toolCalls []cometsdk.ToolCallBlock) (db.Message, map[string]string, error) {
	reasoningJSON, err := marshalReasoningContent(reasoningBlocks)
	if err != nil {
		return db.Message{}, nil, fmt.Errorf("marshal reasoning: %w", err)
	}
	assistant, err := s.q.CreateMessage(ctx, db.CreateMessageParams{
		ID:               id.New(),
		SessionID:        sessionID,
		Role:             "assistant",
		Content:          text,
		ReasoningContent: reasoningJSON,
		TokenCount:       0,
	})
	if err != nil {
		return db.Message{}, nil, err
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
			return db.Message{}, nil, err
		}
		toolIDs[tc.ID] = persistedID
	}
	if err := s.q.TouchSession(ctx, sessionID); err != nil {
		return db.Message{}, nil, err
	}
	return assistant, toolIDs, nil
}

func sqlNullInt(v *int64) sql.NullInt64 {
	if v == nil {
		return sql.NullInt64{Valid: false}
	}
	return sql.NullInt64{Int64: *v, Valid: true}
}

// AppendToolResultMessage persists a tool result turn referenced by tool call id.
func (s *Service) AppendToolResultMessage(ctx context.Context, sessionID, toolCallID, output string, isErr bool) (db.Message, error) {
	payload := toolResultPayload{
		ToolCallID: toolCallID,
		Content:    output,
		IsError:    isErr,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return db.Message{}, err
	}
	msg, err := s.q.CreateMessage(ctx, db.CreateMessageParams{
		ID:         id.New(),
		SessionID:  sessionID,
		Role:       "tool_result",
		Content:    string(raw),
		TokenCount: 0,
	})
	if err != nil {
		return db.Message{}, err
	}
	if err := s.q.TouchSession(ctx, sessionID); err != nil {
		return db.Message{}, err
	}
	return msg, nil
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

// SaveTokenUsage writes the latest cumulative-ish usage snapshot on the session row as JSON.
func (s *Service) SaveTokenUsage(ctx context.Context, sessionID string, u cometsdk.TokenUsage) error {
	b, err := json.Marshal(u)
	if err != nil {
		return err
	}
	return s.q.UpdateSessionTokenUsage(ctx, db.UpdateSessionTokenUsageParams{
		TokenUsage: string(b),
		ID:         sessionID,
	})
}

// BuildSDKMessages reconstructs provider-neutral messages from SQLite for the next LLM request.
func (s *Service) BuildSDKMessages(ctx context.Context, sessionID string) ([]cometsdk.Message, error) {
	rows, err := s.q.ListMessagesBySession(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	out := make([]cometsdk.Message, 0, len(rows))
	for _, m := range rows {
		switch m.Role {
		case "user":
			out = append(out, cometsdk.Message{
				Role:    cometsdk.RoleUser,
				Content: []cometsdk.Block{cometsdk.TextBlock{Text: m.Content}},
			})
		case "assistant":
			blocks, err := s.assistantBlocks(ctx, m)
			if err != nil {
				return nil, err
			}
			reasoningBlocks, err := unmarshalReasoningContent(m.ReasoningContent)
			if err != nil {
				return nil, fmt.Errorf("decode reasoning_content %s: %w", m.ID, err)
			}
			out = append(out, cometsdk.Message{
				Role:             cometsdk.RoleAssistant,
				Content:          blocks,
				ReasoningContent: reasoningBlocks,
			})
		case "tool_result":
			var p toolResultPayload
			if err := json.Unmarshal([]byte(m.Content), &p); err != nil {
				return nil, fmt.Errorf("decode tool_result %s: %w", m.ID, err)
			}
			out = append(out, cometsdk.Message{
				Role: cometsdk.RoleToolResult,
				Content: []cometsdk.Block{
					cometsdk.ToolResultBlock{
						ToolCallID: p.ToolCallID,
						Content:    p.Content,
						IsError:    p.IsError,
					},
				},
			})
		case "system":
			// Stored system rows are optional; the live system prompt comes from the agent.
			continue
		default:
			return nil, fmt.Errorf("unknown message role %q", m.Role)
		}
	}
	return out, nil
}

func (s *Service) assistantBlocks(ctx context.Context, m db.Message) ([]cometsdk.Block, error) {
	var blocks []cometsdk.Block
	if strings.TrimSpace(m.Content) != "" {
		blocks = append(blocks, cometsdk.TextBlock{Text: m.Content})
	}
	tcs, err := s.q.ListToolCallsByMessage(ctx, m.ID)
	if err != nil {
		return nil, err
	}
	for _, tc := range tcs {
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
	return blocks, nil
}
