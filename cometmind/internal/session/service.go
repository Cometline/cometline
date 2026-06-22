package session

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	cometsdk "github.com/cometline/comet-sdk"
	"github.com/cometline/cometmind/internal/db"
	"github.com/cometline/cometmind/internal/id"
)

const contentEnvelopePrefix = "cometmind:content:v1\n"

// ContentBlock is the persisted/API representation of user multimodal content.
type ContentBlock struct {
	Type      string `json:"type"`
	Text      string `json:"text,omitempty"`
	MediaType string `json:"media_type,omitempty"`
	Data      string `json:"data,omitempty"`
}

type contentEnvelope struct {
	Blocks      []ContentBlock `json:"blocks"`
	DisplayText string         `json:"display_text,omitempty"`
}

// toolResultPayload is stored in messages.content for role=tool_result.
type toolResultPayload struct {
	ToolCallID string `json:"tool_call_id"`
	Content    string `json:"content"`
	IsError    bool   `json:"is_error"`
}

// InjectedMemory is a memory surfaced to the UI for a turn. It is persisted as
// a JSON array in messages.injected_memories so the memory card survives a
// session reload (previously these were only emitted live over SSE).
type InjectedMemory struct {
	ID              string  `json:"id"`
	Content         string  `json:"content"`
	Kind            string  `json:"kind"`
	Similarity      float64 `json:"similarity"`
	EffectiveWeight float64 `json:"effective_weight"`
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
func (s *Service) EnsureWorkspace(ctx context.Context, absRoot string) (Workspace, error) {
	clean := filepath.Clean(absRoot)
	w, err := s.q.GetWorkspaceByPath(ctx, clean)
	if err == nil {
		return workspaceFromDB(w), nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return Workspace{}, err
	}
	created, err := s.q.CreateWorkspace(ctx, db.CreateWorkspaceParams{
		ID:   id.New(),
		Name: filepath.Base(clean),
		Path: clean,
	})
	if err != nil {
		return Workspace{}, err
	}
	return workspaceFromDB(created), nil
}

// GetWorkspace loads a workspace by id.
func (s *Service) GetWorkspace(ctx context.Context, workspaceID string) (Workspace, error) {
	w, err := s.q.GetWorkspace(ctx, workspaceID)
	if err != nil {
		return Workspace{}, mapNotFound(err, ErrWorkspaceNotFound)
	}
	return workspaceFromDB(w), nil
}

// LookupWorkspaceByPath loads a workspace by path without creating it.
func (s *Service) LookupWorkspaceByPath(ctx context.Context, absRoot string) (Workspace, error) {
	w, err := s.q.GetWorkspaceByPath(ctx, filepath.Clean(absRoot))
	if err != nil {
		return Workspace{}, mapNotFound(err, ErrWorkspaceNotFound)
	}
	return workspaceFromDB(w), nil
}

// ListWorkspaces returns registered workspace roots that still exist on disk.
func (s *Service) ListWorkspaces(ctx context.Context) ([]Workspace, error) {
	rows, err := s.q.ListWorkspaces(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Workspace, 0, len(rows))
	for _, row := range rows {
		ws := workspaceFromDB(row)
		if !workspaceRootExists(ws.Path) {
			continue
		}
		out = append(out, ws)
	}
	return out, nil
}

// CountSessionsForWorkspace returns how many sessions reference a workspace.
func (s *Service) CountSessionsForWorkspace(ctx context.Context, workspaceID string) (int64, error) {
	return s.q.CountSessionsForWorkspace(ctx, workspaceID)
}

// PruneMissingWorkspaces removes registered workspaces whose directories are
// gone and that have no sessions. Workspaces with sessions are kept for history.
func (s *Service) PruneMissingWorkspaces(ctx context.Context) (int, error) {
	rows, err := s.q.ListWorkspaces(ctx)
	if err != nil {
		return 0, err
	}
	pruned := 0
	for _, row := range rows {
		if workspaceRootExists(row.Path) {
			continue
		}
		count, err := s.q.CountSessionsForWorkspace(ctx, row.ID)
		if err != nil {
			return pruned, err
		}
		if count > 0 {
			continue
		}
		if err := s.q.DeleteWorkspace(ctx, row.ID); err != nil {
			return pruned, err
		}
		pruned++
	}
	return pruned, nil
}

// DeleteWorkspaceByPath removes a workspace registration when it has no sessions.
func (s *Service) DeleteWorkspaceByPath(ctx context.Context, absRoot string) error {
	clean := filepath.Clean(strings.TrimSpace(absRoot))
	if clean == "" {
		return fmt.Errorf("workspace path is required")
	}
	w, err := s.q.GetWorkspaceByPath(ctx, clean)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	count, err := s.q.CountSessionsForWorkspace(ctx, w.ID)
	if err != nil {
		return err
	}
	if count > 0 {
		return ErrWorkspaceHasSessions
	}
	return s.q.DeleteWorkspace(ctx, w.ID)
}

func workspaceRootExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func activeDelegationStatuses() map[string]bool {
	return map[string]bool{
		"pending": true,
		"running": true,
	}
}

// ChangeSessionWorkspace reassigns a session to a different workspace root.
func (s *Service) ChangeSessionWorkspace(ctx context.Context, sessionID, absPath string) (Session, error) {
	sess, err := s.GetSession(ctx, sessionID)
	if err != nil {
		return Session{}, err
	}
	if activeDelegationStatuses()[sess.DelegationStatus] {
		return Session{}, ErrActiveDelegation
	}

	ws, err := s.EnsureWorkspace(ctx, absPath)
	if err != nil {
		return Session{}, err
	}
	if ws.ID == sess.WorkspaceID {
		return sess, nil
	}

	oldPath, err := s.WorkspacePath(ctx, sess.WorkspaceID)
	if err != nil {
		return Session{}, err
	}

	if err := s.q.UpdateSessionWorkspace(ctx, db.UpdateSessionWorkspaceParams{
		WorkspaceID: ws.ID,
		ID:          sessionID,
	}); err != nil {
		return Session{}, err
	}
	_ = s.q.UpdateGatewaySessionWorkspace(ctx, db.UpdateGatewaySessionWorkspaceParams{
		WorkspaceID:        ws.ID,
		CometmindSessionID: sessionID,
	})

	note := fmt.Sprintf(
		"Workspace changed from %s to %s. File tools now operate under this directory.",
		oldPath,
		ws.Path,
	)
	if _, err := s.AppendSystemMessage(ctx, sessionID, note); err != nil {
		return Session{}, err
	}

	return s.GetSession(ctx, sessionID)
}

// ForkSession creates a new session in the target workspace, copying the
// originating session's metadata and full message/tool-call transcript. The
// original session is left untouched.
func (s *Service) ForkSession(ctx context.Context, sessionID, absPath string) (Session, error) {
	src, err := s.GetSession(ctx, sessionID)
	if err != nil {
		return Session{}, err
	}

	ws, err := s.EnsureWorkspace(ctx, absPath)
	if err != nil {
		return Session{}, err
	}

	forked, err := s.q.CreateSession(ctx, db.CreateSessionParams{
		ID:          id.New(),
		WorkspaceID: ws.ID,
		Title:       src.Title,
		ModelID:     src.ModelID,
		ProviderID:  src.ProviderID,
		Status:      "active",
	})
	if err != nil {
		return Session{}, err
	}

	msgs, err := s.q.ListMessagesBySession(ctx, sessionID)
	if err != nil {
		return Session{}, err
	}
	// Tool-call IDs are referenced by both the assistant's tool_call blocks and
	// the matching tool_result payloads. Copying with fresh IDs requires
	// remapping the tool_result references so the provider sees consistent
	// tool_call_id pairs; otherwise it rejects the request (HTTP 400).
	toolCallIDMap := make(map[string]string)
	for _, msg := range msgs {
		content := msg.Content
		if msg.Role == "tool_result" {
			remapped, err := remapToolResultContent(content, toolCallIDMap)
			if err != nil {
				return Session{}, err
			}
			content = remapped
		}
		newMsg, err := s.q.CreateMessage(ctx, db.CreateMessageParams{
			ID:               id.New(),
			SessionID:        forked.ID,
			Role:             msg.Role,
			Content:          content,
			ReasoningContent: msg.ReasoningContent,
			TokenCount:       msg.TokenCount,
		})
		if err != nil {
			return Session{}, err
		}
		calls, err := s.q.ListToolCallsByMessage(ctx, msg.ID)
		if err != nil {
			return Session{}, err
		}
		for _, call := range calls {
			newCallID := id.New()
			toolCallIDMap[call.ID] = newCallID
			if _, err := s.q.CreateToolCall(ctx, db.CreateToolCallParams{
				ID:         newCallID,
				MessageID:  newMsg.ID,
				ToolName:   call.ToolName,
				Arguments:  call.Arguments,
				Result:     call.Result,
				DurationMs: call.DurationMs,
				ExitCode:   call.ExitCode,
			}); err != nil {
				return Session{}, err
			}
		}
	}

	oldPath, err := s.WorkspacePath(ctx, src.WorkspaceID)
	if err == nil && oldPath != ws.Path {
		note := fmt.Sprintf(
			"Forked from a session in %s. File tools now operate under %s.",
			oldPath,
			ws.Path,
		)
		if _, err := s.AppendSystemMessage(ctx, forked.ID, note); err != nil {
			return Session{}, err
		}
	}

	return s.GetSession(ctx, forked.ID)
}

// remapToolResultContent rewrites the tool_call_id inside a persisted
// tool_result payload using the old→new tool-call ID mapping built while
// copying a forked transcript. Unknown IDs are left untouched.
func remapToolResultContent(content string, idMap map[string]string) (string, error) {
	var p toolResultPayload
	if err := json.Unmarshal([]byte(content), &p); err != nil {
		return "", fmt.Errorf("decode tool_result for fork: %w", err)
	}
	if newID, ok := idMap[p.ToolCallID]; ok {
		p.ToolCallID = newID
	}
	raw, err := json.Marshal(p)
	if err != nil {
		return "", fmt.Errorf("encode tool_result for fork: %w", err)
	}
	return string(raw), nil
}

// AppendSystemMessage persists a system notice in the transcript.
func (s *Service) AppendSystemMessage(ctx context.Context, sessionID, text string) (Message, error) {
	msg, err := s.q.CreateMessage(ctx, db.CreateMessageParams{
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

// NewSession creates a persisted session row scoped to a workspace.
func (s *Service) NewSession(ctx context.Context, workspaceID string, modelID, providerID string) (Session, error) {
	sess, err := s.q.CreateSession(ctx, db.CreateSessionParams{
		ID:          id.New(),
		WorkspaceID: workspaceID,
		Title:       "",
		ModelID:     modelID,
		ProviderID:  providerID,
		Status:      "active",
	})
	if err != nil {
		return Session{}, err
	}
	return sessionFromDB(sess), nil
}

// GetSession loads a session by id.
func (s *Service) GetSession(ctx context.Context, sessionID string) (Session, error) {
	row, err := s.q.GetSession(ctx, sessionID)
	if err != nil {
		return Session{}, mapNotFound(err, ErrSessionNotFound)
	}
	return attachGatewayMetadata(
		sessionFromDB(row.Session),
		row.GatewayPlatform,
		row.GatewayChannelID,
		row.GatewayThreadID,
	), nil
}

// ListSessions lists sessions for a workspace ordered by recent activity.
func (s *Service) ListSessions(ctx context.Context, workspaceID string) ([]Session, error) {
	rows, err := s.q.ListSessionsByWorkspace(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	return sessionsFromDB(rows), nil
}

// ListAllSessions lists top-level sessions across every workspace, ordered by
// recent activity. Delegated child sessions are excluded.
func (s *Service) ListAllSessions(ctx context.Context) ([]Session, error) {
	rows, err := s.q.ListAllSessions(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Session, len(rows))
	for i, row := range rows {
		out[i] = attachGatewayMetadata(
			sessionFromDB(row.Session),
			row.GatewayPlatform,
			row.GatewayChannelID,
			row.GatewayThreadID,
		)
	}
	return out, nil
}

// DeleteSession removes a session and cascades its messages and tool calls.
// Child sessions are deleted first so delegated rows cannot orphan into the sidebar.
func (s *Service) DeleteSession(ctx context.Context, sessionID string) error {
	children, err := s.ListChildSessions(ctx, sessionID)
	if err != nil {
		return err
	}
	for _, child := range children {
		if err := s.DeleteSession(ctx, child.ID); err != nil {
			return err
		}
	}
	return s.q.DeleteSession(ctx, sessionID)
}

// ClearSessionTranscript deletes all transcript rows for a session and resets
// compaction, token usage, and title while preserving the session identity.
// Delegated child sessions are removed as well so subagent UI does not reappear
// on transcript reload.
func (s *Service) ClearSessionTranscript(ctx context.Context, sessionID string) error {
	if _, err := s.GetSession(ctx, sessionID); err != nil {
		return err
	}
	children, err := s.ListChildSessions(ctx, sessionID)
	if err != nil {
		return err
	}
	for _, child := range children {
		if err := s.DeleteSession(ctx, child.ID); err != nil {
			return err
		}
	}
	if err := s.q.DeleteMessagesBySession(ctx, sessionID); err != nil {
		return err
	}
	return s.q.ResetSessionTranscriptState(ctx, db.ResetSessionTranscriptStateParams{
		Title:      "",
		TokenUsage: "{}",
		ID:         sessionID,
	})
}

// UpdateContextSummary persists rolling compaction state for a session.
func (s *Service) UpdateContextSummary(ctx context.Context, sessionID, summary, untilMessageID string) error {
	var until sql.NullString
	if strings.TrimSpace(untilMessageID) != "" {
		until = sql.NullString{String: untilMessageID, Valid: true}
	}
	updatedAt := time.Now().UTC().Format(time.RFC3339)
	return s.q.UpdateSessionContextSummary(ctx, db.UpdateSessionContextSummaryParams{
		ContextSummary:          summary,
		CompactedUntilMessageID: until,
		ContextSummaryUpdatedAt: sql.NullString{String: updatedAt, Valid: true},
		ID:                      sessionID,
	})
}

// WorkspacePath resolves the filesystem root for a workspace id.
func (s *Service) WorkspacePath(ctx context.Context, workspaceID string) (string, error) {
	w, err := s.q.GetWorkspace(ctx, workspaceID)
	if err != nil {
		return "", mapNotFound(err, ErrWorkspaceNotFound)
	}
	return w.Path, nil
}

// SetTitleIfEmpty updates session title once (used after first user turn).
// The update is expressed as a single atomic SQL statement whose WHERE clause
// checks for a blank title, eliminating the read-check-write TOCTOU race that
// would occur if two concurrent callers both observed an empty title.
func (s *Service) SetTitleIfEmpty(ctx context.Context, sessionID, title string) error {
	return s.q.SetTitleIfEmpty(ctx, db.SetTitleIfEmptyParams{
		ID:    sessionID,
		Title: title,
	})
}

// UpdateTitle unconditionally overwrites a session's title. Used when an
// LLM-generated title replaces the provisional first-turn placeholder.
func (s *Service) UpdateTitle(ctx context.Context, sessionID, title string) error {
	return s.q.UpdateSessionTitle(ctx, db.UpdateSessionTitleParams{
		ID:    sessionID,
		Title: title,
	})
}

// UpdateSessionModel persists a new model/provider pair for an existing session.
func (s *Service) UpdateSessionModel(ctx context.Context, sessionID, modelID, providerID string) (Session, error) {
	modelID = strings.TrimSpace(modelID)
	providerID = strings.TrimSpace(providerID)
	if modelID == "" || providerID == "" {
		return Session{}, fmt.Errorf("model_id and provider_id are required")
	}
	if _, err := s.GetSession(ctx, sessionID); err != nil {
		return Session{}, err
	}
	if err := s.q.UpdateSessionModel(ctx, db.UpdateSessionModelParams{
		ModelID:    modelID,
		ProviderID: providerID,
		ID:         sessionID,
	}); err != nil {
		return Session{}, err
	}
	return s.GetSession(ctx, sessionID)
}

// UpdateSessionPinned persists whether a session is pinned in the sidebar.
func (s *Service) UpdateSessionPinned(ctx context.Context, sessionID string, pinned bool) (Session, error) {
	if _, err := s.GetSession(ctx, sessionID); err != nil {
		return Session{}, err
	}
	var pinnedInt int64
	if pinned {
		pinnedInt = 1
	}
	if err := s.q.UpdateSessionPinned(ctx, db.UpdateSessionPinnedParams{
		Pinned: pinnedInt,
		ID:     sessionID,
	}); err != nil {
		return Session{}, err
	}
	return s.GetSession(ctx, sessionID)
}

// UpdateSessionTitle persists a new display title for an existing session.
func (s *Service) UpdateSessionTitle(ctx context.Context, sessionID, title string) (Session, error) {
	if _, err := s.GetSession(ctx, sessionID); err != nil {
		return Session{}, err
	}
	if err := s.q.UpdateSessionTitle(ctx, db.UpdateSessionTitleParams{
		ID:    sessionID,
		Title: strings.TrimSpace(title),
	}); err != nil {
		return Session{}, err
	}
	return s.GetSession(ctx, sessionID)
}

// AppendUserMessage persists a user turn.
func (s *Service) AppendUserMessage(ctx context.Context, sessionID, text string) (Message, error) {
	return s.AppendUserMessageContent(ctx, sessionID, []ContentBlock{{Type: "text", Text: text}}, "")
}

// AppendUserMessageContent persists a user turn with text and optional image blocks.
// When displayText is set, transcript UIs show it instead of the agent-facing text.
func (s *Service) AppendUserMessageContent(ctx context.Context, sessionID string, blocks []ContentBlock, displayText string) (Message, error) {
	content, err := marshalMessageContent(blocks, displayText)
	if err != nil {
		return Message{}, err
	}
	msg, err := s.q.CreateMessage(ctx, db.CreateMessageParams{
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

func marshalMessageContent(blocks []ContentBlock, displayText string) (string, error) {
	displayText = strings.TrimSpace(displayText)
	if displayText == "" && len(blocks) == 1 && blocks[0].Type == "text" {
		return blocks[0].Text, nil
	}
	env := contentEnvelope{Blocks: blocks}
	if displayText != "" {
		env.DisplayText = displayText
	}
	raw, err := json.Marshal(env)
	if err != nil {
		return "", err
	}
	return contentEnvelopePrefix + string(raw), nil
}

// DecodeMessageContent returns content blocks from a persisted message. Plain
// legacy content is treated as a single text block.
func DecodeMessageContent(raw string) ([]ContentBlock, error) {
	if !strings.HasPrefix(raw, contentEnvelopePrefix) {
		return []ContentBlock{{Type: "text", Text: raw}}, nil
	}
	var env contentEnvelope
	if err := json.Unmarshal([]byte(strings.TrimPrefix(raw, contentEnvelopePrefix)), &env); err != nil {
		return nil, err
	}
	return env.Blocks, nil
}

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

// PlainTextFromContent extracts agent-facing text from decoded content blocks.
func PlainTextFromContent(blocks []ContentBlock) string {
	var b strings.Builder
	for _, block := range blocks {
		if block.Type == "text" {
			b.WriteString(block.Text)
		}
	}
	return b.String()
}

// DisplayTextFromStoredContent returns the UI label for a persisted user message.
func DisplayTextFromStoredContent(raw string) string {
	if !strings.HasPrefix(raw, contentEnvelopePrefix) {
		return raw
	}
	var env contentEnvelope
	if err := json.Unmarshal([]byte(strings.TrimPrefix(raw, contentEnvelopePrefix)), &env); err != nil {
		return raw
	}
	if strings.TrimSpace(env.DisplayText) != "" {
		return env.DisplayText
	}
	return PlainTextFromContent(env.Blocks)
}

// TitleTextFromContent picks a short session title from user content.
func TitleTextFromContent(blocks []ContentBlock, displayText string) string {
	if strings.TrimSpace(displayText) != "" {
		return displayText
	}
	return PlainTextFromContent(blocks)
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

// marshalInjectedMemories serializes injected memories to a JSON array string,
// always returning a valid array (never "null") for the NOT NULL column.
func marshalInjectedMemories(memories []InjectedMemory) (string, error) {
	if len(memories) == 0 {
		return "[]", nil
	}
	raw, err := json.Marshal(memories)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

// unmarshalInjectedMemories parses the persisted JSON array, tolerating empty
// or malformed values by returning an empty slice.
func unmarshalInjectedMemories(raw string) []InjectedMemory {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "[]" || raw == "null" {
		return nil
	}
	var memories []InjectedMemory
	if err := json.Unmarshal([]byte(raw), &memories); err != nil {
		return nil
	}
	return memories
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
	assistant, err := s.q.CreateMessage(ctx, db.CreateMessageParams{
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

func sqlNullInt(v *int64) sql.NullInt64 {
	if v == nil {
		return sql.NullInt64{Valid: false}
	}
	return sql.NullInt64{Int64: *v, Valid: true}
}

func mapNotFound(err error, notFound error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return notFound
	}
	return err
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
	msg, err := s.q.CreateMessage(ctx, db.CreateMessageParams{
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
	sess, err := s.GetSession(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	rows, err := s.q.ListMessagesBySession(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	rows = FilterMessagesAfterCompacted(rows, sess.CompactedUntilMessageID)
	return s.buildSDKMessagesFromRows(ctx, sessionID, rows)
}

// ListMessageRows returns raw persisted transcript rows in chronological order.
func (s *Service) ListMessageRows(ctx context.Context, sessionID string) ([]db.Message, error) {
	return s.q.ListMessagesBySession(ctx, sessionID)
}

// BuildSDKMessagesAll rebuilds the full transcript without compaction filtering.
func (s *Service) BuildSDKMessagesAll(ctx context.Context, sessionID string) ([]cometsdk.Message, error) {
	rows, err := s.q.ListMessagesBySession(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	return s.buildSDKMessagesFromRows(ctx, sessionID, rows)
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

func (s *Service) buildSDKMessagesFromRows(ctx context.Context, sessionID string, rows []db.Message) ([]cometsdk.Message, error) {
	// One query for all tool calls instead of one per assistant message.
	allCalls, err := s.q.ListToolCallsBySession(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	callsByMessage := make(map[string][]db.ToolCall, len(allCalls))
	for _, tc := range allCalls {
		callsByMessage[tc.MessageID] = append(callsByMessage[tc.MessageID], tc)
	}
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
			blocks := assistantBlocks(m, callsByMessage[m.ID])
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
						Content:    TruncateToolResultForPrompt(p.Content, MaxToolResultPromptRunes),
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

func assistantBlocks(m db.Message, tcs []db.ToolCall) []cometsdk.Block {
	var blocks []cometsdk.Block
	if strings.TrimSpace(m.Content) != "" {
		blocks = append(blocks, cometsdk.TextBlock{Text: m.Content})
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
	return blocks
}

// NewChildSession creates a delegated child session linked to a parent.
func (s *Service) NewChildSession(ctx context.Context, parent Session, purpose, subagentKind string) (Session, error) {
	title := purpose
	if len(title) > 80 {
		title = title[:80]
	}
	sess, err := s.q.CreateChildSession(ctx, db.CreateChildSessionParams{
		ID:               id.New(),
		WorkspaceID:      parent.WorkspaceID,
		Title:            title,
		ModelID:          parent.ModelID,
		ProviderID:       parent.ProviderID,
		Status:           "active",
		ParentSessionID:  sql.NullString{String: parent.ID, Valid: true},
		Purpose:          purpose,
		DelegationStatus: "pending",
		OutputSummary:    "",
		SubagentKind:     subagentKind,
	})
	if err != nil {
		return Session{}, err
	}
	return sessionFromDB(sess), nil
}

// CompactChildSession wipes a child transcript while preserving delegation metadata.
func (s *Service) CompactChildSession(ctx context.Context, childID string) error {
	if err := s.q.DeleteMessagesBySession(ctx, childID); err != nil {
		return err
	}
	return s.q.CompactChildSession(ctx, childID)
}

// LastAssistantText returns the most recent assistant message text for a session.
func (s *Service) LastAssistantText(ctx context.Context, sessionID string) (string, error) {
	msgs, err := s.q.ListMessagesBySession(ctx, sessionID)
	if err != nil {
		return "", err
	}
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role == "assistant" {
			return msgs[i].Content, nil
		}
	}
	return "", nil
}

// ListChildSessions returns delegated sessions for a parent session.
func (s *Service) ListChildSessions(ctx context.Context, parentSessionID string) ([]Session, error) {
	rows, err := s.q.ListChildSessions(ctx, sql.NullString{String: parentSessionID, Valid: true})
	if err != nil {
		return nil, err
	}
	return sessionsFromDB(rows), nil
}

// UpdateDelegation persists delegation status and summary for a child session.
func (s *Service) UpdateDelegation(ctx context.Context, sessionID, status, summary string) error {
	return s.q.UpdateSessionDelegation(ctx, db.UpdateSessionDelegationParams{
		DelegationStatus: status,
		OutputSummary:    summary,
		ID:               sessionID,
	})
}

// UpdateDelegationState persists delegation status, summary, and pending question.
func (s *Service) UpdateDelegationState(ctx context.Context, sessionID, status, summary, pendingQuestion string) error {
	return s.q.UpdateSessionDelegationState(ctx, db.UpdateSessionDelegationStateParams{
		DelegationStatus: status,
		OutputSummary:    summary,
		PendingQuestion:  pendingQuestion,
		ID:               sessionID,
	})
}

// UpdateACPSessionID stores the external ACP session identifier for a child session.
func (s *Service) UpdateACPSessionID(ctx context.Context, sessionID, acpSessionID string) error {
	return s.q.UpdateSessionACP(ctx, db.UpdateSessionACPParams{
		AcpSessionID: acpSessionID,
		ID:           sessionID,
	})
}

// GetActiveChildForParent returns the most recently updated active delegated child.
func (s *Service) GetActiveChildForParent(ctx context.Context, parentSessionID string) (Session, error) {
	row, err := s.q.GetActiveChildForParent(ctx, sql.NullString{String: parentSessionID, Valid: true})
	if err != nil {
		return Session{}, err
	}
	return sessionFromDB(row), nil
}

// UpsertGatewaySession maps an external chat surface to a CometMind session.
func (s *Service) UpsertGatewaySession(ctx context.Context, platform, userID, channelID, threadID, sessionID, workspaceID string) (db.GatewaySession, error) {
	return s.q.UpsertGatewaySession(ctx, db.UpsertGatewaySessionParams{
		ID:                 id.New(),
		Platform:           platform,
		PlatformUserID:     userID,
		PlatformChannelID:  channelID,
		ThreadID:           threadID,
		CometmindSessionID: sessionID,
		WorkspaceID:        workspaceID,
	})
}

// LookupGatewaySession finds a mapped CometMind session for a platform identity.
func (s *Service) LookupGatewaySession(ctx context.Context, platform, userID, channelID, threadID string) (db.GatewaySession, error) {
	return s.q.GetGatewaySession(ctx, db.GetGatewaySessionParams{
		Platform:          platform,
		PlatformUserID:    userID,
		PlatformChannelID: channelID,
		ThreadID:          threadID,
	})
}
