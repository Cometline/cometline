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
	"github.com/cometline/cometmind/internal/logging"
	"github.com/cometline/cometmind/internal/media"
	"github.com/cometline/cometmind/internal/usage"
)

const (
	contentEnvelopePrefix = "cometmind:content:v1\n"
	errorMessagePrefix    = "cometmind:error:v1\n"
)

// ContentBlock is the persisted/API representation of multimodal content.
// User images typically carry base64 Data. Assistant-presented images store
// a media-store ID (no inline bytes) so blobs live under ~/.cometmind/media.
type ContentBlock struct {
	Type      string `json:"type"`
	Text      string `json:"text,omitempty"`
	MediaType string `json:"media_type,omitempty"`
	Data      string `json:"data,omitempty"`
	ID        string `json:"id,omitempty"`
	Alt       string `json:"alt,omitempty"`
}

// MessageContextRef is a slim UI reference for a web/file/terminal/message context
// attached to a user turn. Content bodies are not stored here — they are
// already inlined into the agent-facing text blocks.
type MessageContextRef struct {
	Kind   string `json:"kind"`
	Title  string `json:"title,omitempty"`
	Source string `json:"source"`
	Role   string `json:"role,omitempty"` // "viewing" for path-only file refs
}

type contentEnvelope struct {
	Blocks      []ContentBlock      `json:"blocks"`
	DisplayText string              `json:"display_text,omitempty"`
	Contexts    []MessageContextRef `json:"contexts,omitempty"`
}

type errorMessageEnvelope struct {
	Message string `json:"message"`
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
type MemoryBucket string

const (
	MemoryBucketPreference  MemoryBucket = "preference"
	MemoryBucketTaskOutcome MemoryBucket = "task_outcome"
	MemoryBucketSemantic    MemoryBucket = "semantic"
)

type InjectedMemory struct {
	ID              string       `json:"id"`
	Content         string       `json:"content"`
	Kind            string       `json:"kind"`
	Bucket          MemoryBucket `json:"bucket"`
	Similarity      float64      `json:"similarity"`
	EffectiveWeight float64      `json:"effective_weight"`
}

// Service coordinates persistence for workspaces, sessions, messages, and tool calls.
type Service struct {
	q     *db.Queries
	usage usage.Recorder
}

// New creates a session service bound to the shared sqlc querier.
func New(sqlDB *sql.DB) *Service {
	return &Service{q: db.New(sqlDB)}
}

// SetUsageRecorder records agent-step token usage on the spend ledger.
func (s *Service) SetUsageRecorder(r usage.Recorder) {
	if s == nil {
		return
	}
	s.usage = r
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

// ChangeSessionWorkspace reassigns a session to a different workspace root.
func (s *Service) ChangeSessionWorkspace(ctx context.Context, sessionID, absPath string) (Session, error) {
	sess, err := s.GetSession(ctx, sessionID)
	if err != nil {
		return Session{}, err
	}
	if sess.DelegationStatus.IsActive() {
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
	_ = s.q.UpdateSessionMediaWorkspace(ctx, db.UpdateSessionMediaWorkspaceParams{
		WorkspaceID: nullSessionID(ws.ID),
		SessionID:   nullSessionID(sessionID),
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
		Origin:      "user",
		AgentMode:   string(AgentModeAuto),
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
		newMsg, err := s.createMessage(ctx, db.CreateMessageParams{
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
	if err := s.copySessionMedia(ctx, src, forked.ID, ws.ID); err != nil {
		return Session{}, err
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

func (s *Service) createMessage(ctx context.Context, params db.CreateMessageParams) (db.Message, error) {
	if err := s.q.MarkSessionNonDisposable(ctx, params.SessionID); err != nil {
		return db.Message{}, err
	}
	return s.q.CreateMessage(ctx, params)
}

// AppendErrorMessage persists a turn-level error notice for transcript replay.
func (s *Service) AppendErrorMessage(ctx context.Context, sessionID, text string) (Message, error) {
	raw, err := marshalErrorMessageContent(text)
	if err != nil {
		return Message{}, err
	}
	return s.AppendSystemMessage(ctx, sessionID, raw)
}

func marshalErrorMessageContent(message string) (string, error) {
	env := errorMessageEnvelope{Message: strings.TrimSpace(message)}
	raw, err := json.Marshal(env)
	if err != nil {
		return "", err
	}
	return errorMessagePrefix + string(raw), nil
}

// DecodeErrorMessageContent decodes system rows that represent transcript errors.
func DecodeErrorMessageContent(raw string) (string, bool) {
	if !strings.HasPrefix(raw, errorMessagePrefix) {
		return "", false
	}
	var env errorMessageEnvelope
	if err := json.Unmarshal([]byte(strings.TrimPrefix(raw, errorMessagePrefix)), &env); err != nil {
		return strings.TrimSpace(raw), true
	}
	msg := strings.TrimSpace(env.Message)
	if msg == "" {
		msg = "The request failed."
	}
	return msg, true
}

// NewSession creates a persisted session row scoped to a workspace.
func (s *Service) NewSession(ctx context.Context, workspaceID string, modelID, providerID string) (Session, error) {
	return s.newSessionWithOrigin(ctx, workspaceID, modelID, providerID, "user")
}

// NewAutonomySession creates a session dedicated to an autonomous job run.
func (s *Service) NewAutonomySession(ctx context.Context, workspaceID string, modelID, providerID string) (Session, error) {
	return s.newSessionWithOrigin(ctx, workspaceID, modelID, providerID, "autonomy")
}

// NewInboxSession creates a short-lived session for inbox reply internalization.
func (s *Service) NewInboxSession(ctx context.Context, workspaceID string, modelID, providerID string) (Session, error) {
	return s.newSessionWithOrigin(ctx, workspaceID, modelID, providerID, "inbox")
}

func (s *Service) newSessionWithOrigin(ctx context.Context, workspaceID string, modelID, providerID, origin string) (Session, error) {
	sess, err := s.q.CreateSession(ctx, db.CreateSessionParams{
		ID:          id.New(),
		WorkspaceID: workspaceID,
		Title:       "",
		ModelID:     modelID,
		ProviderID:  providerID,
		Status:      "active",
		Origin:      origin,
		AgentMode:   string(AgentModeAuto),
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

// ListAllSessions lists user-facing top-level sessions across every workspace,
// ordered by recent activity. Delegated child sessions and autonomous job-run
// sessions are excluded.
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
// Gallery media stays on disk and in the catalog with a null session_id.
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

// PruneUnusedUserSessions removes top-level user sessions that were created
// but never changed or given a transcript. Cleared conversations are retained.
func (s *Service) PruneUnusedUserSessions(ctx context.Context) (int, error) {
	ids, err := s.q.ListUnusedUserSessionIDs(ctx)
	if err != nil {
		return 0, err
	}
	for _, id := range ids {
		if err := s.DeleteSession(ctx, id); err != nil {
			return 0, err
		}
	}
	return len(ids), nil
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

// WorkspacePath resolves the filesystem root for a workspace id. This method
// is intentionally duplicated from the WorkspaceStore interface seam so the
// full *Service can satisfy it.
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

// UpdateSessionAgentMode persists the preferred agent mode for a session.
func (s *Service) UpdateSessionAgentMode(ctx context.Context, sessionID string, mode AgentMode) (Session, error) {
	if _, err := s.GetSession(ctx, sessionID); err != nil {
		return Session{}, err
	}
	row, err := s.q.UpdateSessionAgentMode(ctx, db.UpdateSessionAgentModeParams{
		AgentMode: string(mode),
		ID:        sessionID,
	})
	if err != nil {
		return Session{}, err
	}
	return sessionFromDB(row), nil
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

func marshalMessageContent(blocks []ContentBlock, displayText string, contexts []MessageContextRef) (string, error) {
	displayText = strings.TrimSpace(displayText)
	if displayText == "" && len(contexts) == 0 && len(blocks) == 1 && blocks[0].Type == "text" {
		return blocks[0].Text, nil
	}
	env := contentEnvelope{Blocks: blocks}
	if displayText != "" {
		env.DisplayText = displayText
	}
	if len(contexts) > 0 {
		env.Contexts = contexts
	}
	raw, err := json.Marshal(env)
	if err != nil {
		return "", err
	}
	return contentEnvelopePrefix + string(raw), nil
}

// ContextsFromStoredContent returns UI context refs persisted on a user message.
func ContextsFromStoredContent(raw string) []MessageContextRef {
	if !strings.HasPrefix(raw, contentEnvelopePrefix) {
		return nil
	}
	var env contentEnvelope
	if err := json.Unmarshal([]byte(strings.TrimPrefix(raw, contentEnvelopePrefix)), &env); err != nil {
		return nil
	}
	if len(env.Contexts) == 0 {
		return nil
	}
	return env.Contexts
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
	// Always emit a JSON array (never "null") so the NOT NULL column and
	// OpenAI-compatible reasoning_content replay stay well-formed.
	payloads := make([]reasoningBlockPayload, 0, len(blocks))
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
	for i := range memories {
		if memories[i].Bucket != "" {
			continue
		}
		switch memories[i].Kind {
		case "preference":
			memories[i].Bucket = MemoryBucketPreference
		case "task_outcome", "task_summary":
			memories[i].Bucket = MemoryBucketTaskOutcome
		default:
			memories[i].Bucket = MemoryBucketSemantic
		}
	}
	return memories
}

func unmarshalReasoningContent(raw string) ([]cometsdk.Block, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "null" || raw == "[]" {
		return nil, nil
	}
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

// MediaRecord is one cataloged session media item.
type MediaRecord struct {
	ID               string
	SessionID        string
	StorageSessionID string
	WorkspaceID      string
	Kind             string
	MediaType        string
	Alt              string
	Prompt           string
	Model            string
	ProviderID       string
	Source           string
	SourceMediaID    string
	Status           string
	ByteSize         int64
	DurationMs       *int64
	CreatedAt        int64
}

// MediaMeta is optional catalog metadata recorded with an assistant media block.
type MediaMeta struct {
	Source        string
	Prompt        string
	Model         string
	ProviderID    string
	SourceMediaID string
	ByteSize      int64
	DurationMs    *int64
}

// AppendAssistantMedia persists an assistant turn that presents media to the user.
// Image and video blocks must reference media-store IDs (Data should be empty).
func (s *Service) AppendAssistantMedia(ctx context.Context, sessionID string, images []ContentBlock) (Message, error) {
	return s.AppendAssistantMediaWithMeta(ctx, sessionID, images, MediaMeta{Source: "presented"})
}

// AppendAssistantMediaWithMeta persists assistant media and upserts catalog rows.
func (s *Service) AppendAssistantMediaWithMeta(ctx context.Context, sessionID string, items []ContentBlock, meta MediaMeta) (Message, error) {
	if len(items) == 0 {
		return Message{}, fmt.Errorf("at least one media item is required")
	}
	sess, err := s.GetSession(ctx, sessionID)
	if err != nil {
		return Message{}, err
	}
	blocks := make([]ContentBlock, 0, len(items))
	createdIDs := make([]string, 0, len(items))
	for _, item := range items {
		block, err := normalizeAssistantMediaBlock(item)
		if err != nil {
			return Message{}, err
		}
		created, err := s.ensureSessionMedia(ctx, sess, block, meta)
		if err != nil {
			return Message{}, err
		}
		if created {
			createdIDs = append(createdIDs, block.ID)
		}
		blocks = append(blocks, block)
	}
	content, err := marshalMessageContent(blocks, "", nil)
	if err != nil {
		s.rollbackCreatedSessionMedia(ctx, createdIDs)
		return Message{}, err
	}
	assistant, err := s.createMessage(ctx, db.CreateMessageParams{
		ID:         id.New(),
		SessionID:  sessionID,
		Role:       "assistant",
		Content:    content,
		TokenCount: 0,
	})
	if err != nil {
		s.rollbackCreatedSessionMedia(ctx, createdIDs)
		return Message{}, err
	}
	if err := s.q.TouchSession(ctx, sessionID); err != nil {
		return Message{}, err
	}
	return messageFromDB(assistant), nil
}

func normalizeAssistantMediaBlock(item ContentBlock) (ContentBlock, error) {
	kind := strings.TrimSpace(item.Type)
	if kind == "" {
		kind = media.KindImage
	}
	if kind != media.KindImage && kind != media.KindVideo {
		return ContentBlock{}, fmt.Errorf("unexpected content block type %q", kind)
	}
	if strings.TrimSpace(item.ID) == "" {
		return ContentBlock{}, fmt.Errorf("%s id is required", kind)
	}
	if strings.TrimSpace(item.MediaType) == "" {
		return ContentBlock{}, fmt.Errorf("%s media_type is required", kind)
	}
	return ContentBlock{
		Type:      kind,
		ID:        strings.TrimSpace(item.ID),
		MediaType: strings.TrimSpace(item.MediaType),
		Alt:       strings.TrimSpace(item.Alt),
	}, nil
}

// ReadySessionImage loads a ready still that belongs to sessionID.
func (s *Service) ReadySessionImage(ctx context.Context, sessionID, mediaID string) (ReadyImage, error) {
	row, err := s.q.GetReadySessionMedia(ctx, db.GetReadySessionMediaParams{
		ID:        strings.TrimSpace(mediaID),
		SessionID: nullSessionID(sessionID),
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ReadyImage{}, fmt.Errorf("image %s is not available in this session", mediaID)
		}
		return ReadyImage{}, err
	}
	if row.Kind != media.KindImage {
		return ReadyImage{}, fmt.Errorf("media %s is not an image", mediaID)
	}
	mediaType, data, err := media.Read(row.StorageSessionID, row.ID)
	if err != nil {
		return ReadyImage{}, err
	}
	if mediaType == "" {
		mediaType = row.MediaType
	}
	return ReadyImage{ID: row.ID, MediaType: mediaType, Data: data}, nil
}

// MediaListFilter selects ready gallery items.
type MediaListFilter struct {
	WorkspaceID string
	SessionID   string
	Kind        string
}

// ListMedia returns ready catalog items newest first.
func (s *Service) ListMedia(ctx context.Context, filter MediaListFilter) ([]MediaRecord, error) {
	kind := strings.TrimSpace(filter.Kind)
	if kind != "" && kind != media.KindImage && kind != media.KindVideo {
		return nil, fmt.Errorf("kind must be image or video")
	}
	if err := s.backfillLegacyMedia(ctx, filter); err != nil {
		return nil, err
	}
	rows, err := s.q.ListSessionMedia(ctx, db.ListSessionMediaParams{
		WorkspaceID: nullableText(filter.WorkspaceID),
		SessionID:   nullableText(filter.SessionID),
		Kind:        nullableText(kind),
	})
	if err != nil {
		return nil, err
	}
	out := make([]MediaRecord, 0, len(rows))
	for _, row := range rows {
		out = append(out, mediaRecordFromDB(row))
	}
	if strings.TrimSpace(filter.SessionID) == "" {
		out = dedupeGalleryMedia(out)
	}
	return out, nil
}

func (s *Service) backfillLegacyMedia(ctx context.Context, filter MediaListFilter) error {
	sessionIDs := make([]string, 0, 8)
	if id := strings.TrimSpace(filter.SessionID); id != "" {
		sessionIDs = append(sessionIDs, id)
	} else if workspaceID := strings.TrimSpace(filter.WorkspaceID); workspaceID != "" {
		rows, err := s.q.ListSessionsByWorkspace(ctx, workspaceID)
		if err != nil {
			return err
		}
		for _, row := range rows {
			sessionIDs = append(sessionIDs, row.ID)
		}
	} else {
		ids, err := s.q.ListAllSessionIDs(ctx)
		if err != nil {
			return err
		}
		sessionIDs = append(sessionIDs, ids...)
	}
	for _, sessionID := range sessionIDs {
		if err := s.backfillSessionMedia(ctx, sessionID); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) backfillSessionMedia(ctx context.Context, sessionID string) error {
	files, err := media.ListSessionFiles(sessionID)
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return nil
	}
	known := map[string]struct{}{}
	rows, err := s.q.ListSessionMediaBySession(ctx, nullSessionID(sessionID))
	if err != nil {
		return err
	}
	for _, row := range rows {
		known[row.ID] = struct{}{}
	}
	needsBackfill := false
	for _, file := range files {
		if _, ok := known[file.ID]; !ok {
			needsBackfill = true
			break
		}
	}
	if !needsBackfill {
		return nil
	}

	sess, err := s.GetSession(ctx, sessionID)
	if err != nil {
		return err
	}
	hints, err := s.generatedMediaHints(ctx, sessionID)
	if err != nil {
		return err
	}
	messages, err := s.q.ListMessagesBySession(ctx, sessionID)
	if err != nil {
		return err
	}
	for _, msg := range messages {
		if !strings.HasPrefix(msg.Content, contentEnvelopePrefix) {
			continue
		}
		blocks, decodeErr := DecodeMessageContent(msg.Content)
		if decodeErr != nil {
			continue
		}
		for _, block := range blocks {
			if block.Type != media.KindImage && block.Type != media.KindVideo {
				continue
			}
			id := strings.TrimSpace(block.ID)
			if id == "" {
				continue
			}
			if _, ok := known[id]; ok {
				continue
			}
			if _, pathErr := media.AbsolutePath(sessionID, id); pathErr != nil {
				continue
			}
			if _, err := s.ensureSessionMedia(ctx, sess, ContentBlock{
				Type:      block.Type,
				ID:        id,
				MediaType: block.MediaType,
				Alt:       block.Alt,
			}, mediaMetaForLegacy(id, hints, 0)); err != nil {
				return err
			}
			known[id] = struct{}{}
		}
	}

	for _, file := range files {
		if _, ok := known[file.ID]; ok {
			continue
		}
		kind, kindErr := media.KindForMediaType(file.MediaType)
		if kindErr != nil {
			continue
		}
		if _, err := s.ensureSessionMedia(ctx, sess, ContentBlock{
			Type:      kind,
			ID:        file.ID,
			MediaType: file.MediaType,
		}, mediaMetaForLegacy(file.ID, hints, file.ByteSize)); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) generatedMediaHints(ctx context.Context, sessionID string) (map[string]MediaMeta, error) {
	calls, err := s.q.ListToolCallsBySession(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	out := make(map[string]MediaMeta, len(calls))
	for _, call := range calls {
		if call.ToolName != "generate_image" && call.ToolName != "generate_video" {
			continue
		}
		id := parseGeneratedMediaID(call.Result)
		if id == "" {
			continue
		}
		out[id] = MediaMeta{
			Source: "generated",
			Prompt: parseToolPrompt(call.Arguments),
		}
	}
	return out, nil
}

func mediaMetaForLegacy(id string, hints map[string]MediaMeta, byteSize int64) MediaMeta {
	meta := MediaMeta{Source: "presented", ByteSize: byteSize}
	if hint, ok := hints[id]; ok {
		meta.Source = hint.Source
		meta.Prompt = hint.Prompt
	}
	return meta
}

func parseGeneratedMediaID(result string) string {
	const marker = " id="
	idx := strings.Index(result, marker)
	if idx < 0 {
		return ""
	}
	rest := strings.TrimSpace(result[idx+len(marker):])
	id, _, _ := strings.Cut(rest, " ")
	return strings.TrimSpace(id)
}

func parseToolPrompt(arguments string) string {
	var in struct {
		Prompt string `json:"prompt"`
	}
	if err := json.Unmarshal([]byte(arguments), &in); err != nil {
		return ""
	}
	return strings.TrimSpace(in.Prompt)
}

func dedupeGalleryMedia(items []MediaRecord) []MediaRecord {
	ids := make(map[string]struct{}, len(items))
	for _, item := range items {
		ids[item.ID] = struct{}{}
	}
	out := make([]MediaRecord, 0, len(items))
	for _, item := range items {
		if item.SourceMediaID != "" {
			if _, ok := ids[item.SourceMediaID]; ok {
				continue
			}
		}
		out = append(out, item)
	}
	return out
}

// GetMedia returns one catalog row, including tombstones.
func (s *Service) GetMedia(ctx context.Context, mediaID string) (MediaRecord, error) {
	row, err := s.q.GetSessionMedia(ctx, strings.TrimSpace(mediaID))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return MediaRecord{}, fmt.Errorf("media not found")
		}
		return MediaRecord{}, err
	}
	return mediaRecordFromDB(row), nil
}

// ImportMedia copies a ready item into destSessionID as a new file and catalog row.
func (s *Service) ImportMedia(ctx context.Context, destSessionID, mediaID string) (MediaRecord, error) {
	dest, err := s.GetSession(ctx, destSessionID)
	if err != nil {
		return MediaRecord{}, err
	}
	src, err := s.q.GetSessionMedia(ctx, strings.TrimSpace(mediaID))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return MediaRecord{}, fmt.Errorf("media not found")
		}
		return MediaRecord{}, err
	}
	if src.Status != "ready" {
		return MediaRecord{}, fmt.Errorf("media %s is not available", mediaID)
	}
	srcPath, err := media.AbsolutePath(src.StorageSessionID, src.ID)
	if err != nil {
		return MediaRecord{}, err
	}
	copied, err := media.CopyFile(dest.ID, srcPath, src.MediaType, src.Alt)
	if err != nil {
		return MediaRecord{}, err
	}
	row, err := s.q.CreateSessionMedia(ctx, db.CreateSessionMediaParams{
		ID:               copied.ID,
		SessionID:        nullSessionID(dest.ID),
		StorageSessionID: dest.ID,
		WorkspaceID:      nullSessionID(dest.WorkspaceID),
		Kind:             src.Kind,
		MediaType:        src.MediaType,
		Alt:              src.Alt,
		Prompt:           src.Prompt,
		Model:            src.Model,
		ProviderID:       src.ProviderID,
		Source:           "imported",
		SourceMediaID:    src.ID,
		Status:           "ready",
		ByteSize:         copied.ByteSize,
		DurationMs:       src.DurationMs,
	})
	if err != nil {
		_ = media.DeleteFile(dest.ID, copied.ID)
		return MediaRecord{}, err
	}
	record := mediaRecordFromDB(row)
	if _, err := s.AppendAssistantMediaWithMeta(ctx, dest.ID, []ContentBlock{{
		Type:      record.Kind,
		ID:        record.ID,
		MediaType: record.MediaType,
		Alt:       record.Alt,
	}}, MediaMeta{
		Source:        "imported",
		Prompt:        record.Prompt,
		Model:         record.Model,
		ProviderID:    record.ProviderID,
		SourceMediaID: record.SourceMediaID,
		ByteSize:      record.ByteSize,
		DurationMs:    record.DurationMs,
	}); err != nil {
		_ = media.DeleteFile(dest.ID, copied.ID)
		_ = s.deleteImportedCatalog(ctx, record.ID)
		return MediaRecord{}, err
	}
	return record, nil
}

func (s *Service) deleteImportedCatalog(ctx context.Context, mediaID string) error {
	_, err := s.q.MarkSessionMediaDeleted(ctx, mediaID)
	return err
}

// DeleteMedia hard-deletes the file and leaves a tombstone catalog row.
func (s *Service) DeleteMedia(ctx context.Context, mediaID string) (MediaRecord, error) {
	row, err := s.q.GetSessionMedia(ctx, strings.TrimSpace(mediaID))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return MediaRecord{}, fmt.Errorf("media not found")
		}
		return MediaRecord{}, err
	}
	if row.Status == "deleted" {
		return mediaRecordFromDB(row), nil
	}
	if err := media.DeleteFile(row.StorageSessionID, row.ID); err != nil {
		return MediaRecord{}, err
	}
	updated, err := s.q.MarkSessionMediaDeleted(ctx, row.ID)
	if err != nil {
		return MediaRecord{}, err
	}
	return mediaRecordFromDB(updated), nil
}

func mediaRecordFromDB(row db.SessionMedia) MediaRecord {
	var duration *int64
	if row.DurationMs.Valid {
		value := row.DurationMs.Int64
		duration = &value
	}
	return MediaRecord{
		ID:               row.ID,
		SessionID:        mediaSessionID(row.SessionID),
		StorageSessionID: row.StorageSessionID,
		WorkspaceID:      mediaSessionID(row.WorkspaceID),
		Kind:             row.Kind,
		MediaType:        row.MediaType,
		Alt:              row.Alt,
		Prompt:           row.Prompt,
		Model:            row.Model,
		ProviderID:       row.ProviderID,
		Source:           row.Source,
		SourceMediaID:    row.SourceMediaID,
		Status:           row.Status,
		ByteSize:         row.ByteSize,
		DurationMs:       duration,
		CreatedAt:        row.CreatedAt,
	}
}

func nullSessionID(id string) sql.NullString {
	id = strings.TrimSpace(id)
	if id == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: id, Valid: true}
}

func mediaSessionID(value sql.NullString) string {
	if value.Valid {
		return value.String
	}
	return ""
}

func nullableText(value string) any {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return value
}

func (s *Service) rollbackCreatedSessionMedia(ctx context.Context, ids []string) {
	for _, id := range ids {
		_, _ = s.q.MarkSessionMediaDeleted(ctx, id)
	}
}

func (s *Service) ensureSessionMedia(ctx context.Context, sess Session, block ContentBlock, meta MediaMeta) (bool, error) {
	existing, err := s.q.GetSessionMedia(ctx, block.ID)
	if err == nil {
		if mediaSessionID(existing.SessionID) != sess.ID {
			return false, fmt.Errorf("media %s belongs to another session", block.ID)
		}
		return false, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return false, err
	}
	source := strings.TrimSpace(meta.Source)
	if source == "" {
		source = "presented"
	}
	var duration sql.NullInt64
	if meta.DurationMs != nil {
		duration = sql.NullInt64{Int64: *meta.DurationMs, Valid: true}
	}
	_, err = s.q.CreateSessionMedia(ctx, db.CreateSessionMediaParams{
		ID:               block.ID,
		SessionID:        nullSessionID(sess.ID),
		StorageSessionID: sess.ID,
		WorkspaceID:      nullSessionID(sess.WorkspaceID),
		Kind:             block.Type,
		MediaType:        block.MediaType,
		Alt:              block.Alt,
		Prompt:           strings.TrimSpace(meta.Prompt),
		Model:            strings.TrimSpace(meta.Model),
		ProviderID:       strings.TrimSpace(meta.ProviderID),
		Source:           source,
		SourceMediaID:    strings.TrimSpace(meta.SourceMediaID),
		Status:           "ready",
		ByteSize:         meta.ByteSize,
		DurationMs:       duration,
	})
	if err != nil {
		return false, err
	}
	return true, nil
}

func (s *Service) copySessionMedia(ctx context.Context, src Session, destSessionID, destWorkspaceID string) error {
	rows, err := s.q.ListSessionMediaBySession(ctx, nullSessionID(src.ID))
	if err != nil {
		return err
	}
	idMap := make(map[string]string, len(rows))
	for _, row := range rows {
		srcPath, pathErr := media.AbsolutePath(row.StorageSessionID, row.ID)
		if pathErr != nil {
			if errors.Is(pathErr, media.ErrNotFound) {
				continue
			}
			return pathErr
		}
		copied, copyErr := media.CopyFile(destSessionID, srcPath, row.MediaType, row.Alt)
		if copyErr != nil {
			return copyErr
		}
		idMap[row.ID] = copied.ID
		if _, err := s.q.CreateSessionMedia(ctx, db.CreateSessionMediaParams{
			ID:               copied.ID,
			SessionID:        nullSessionID(destSessionID),
			StorageSessionID: destSessionID,
			WorkspaceID:      nullSessionID(destWorkspaceID),
			Kind:             row.Kind,
			MediaType:        row.MediaType,
			Alt:              row.Alt,
			Prompt:           row.Prompt,
			Model:            row.Model,
			ProviderID:       row.ProviderID,
			Source:           row.Source,
			SourceMediaID:    row.ID,
			Status:           "ready",
			ByteSize:         copied.ByteSize,
			DurationMs:       row.DurationMs,
		}); err != nil {
			return err
		}
	}
	if len(idMap) == 0 {
		return nil
	}
	msgs, err := s.q.ListMessagesBySession(ctx, destSessionID)
	if err != nil {
		return err
	}
	for _, msg := range msgs {
		if !strings.HasPrefix(msg.Content, contentEnvelopePrefix) {
			continue
		}
		rewritten, changed, rewriteErr := remapMessageMediaIDs(msg.Content, idMap)
		if rewriteErr != nil {
			return rewriteErr
		}
		if !changed {
			continue
		}
		if err := s.q.UpdateMessageContent(ctx, db.UpdateMessageContentParams{
			ID:      msg.ID,
			Content: rewritten,
		}); err != nil {
			return err
		}
	}
	return nil
}

func remapMessageMediaIDs(raw string, idMap map[string]string) (string, bool, error) {
	blocks, err := DecodeMessageContent(raw)
	if err != nil {
		return "", false, err
	}
	changed := false
	for i, block := range blocks {
		if block.Type != media.KindImage && block.Type != media.KindVideo {
			continue
		}
		if next, ok := idMap[block.ID]; ok {
			blocks[i].ID = next
			changed = true
		}
	}
	if !changed {
		return raw, false, nil
	}
	display := ""
	if strings.HasPrefix(raw, contentEnvelopePrefix) {
		var env contentEnvelope
		if err := json.Unmarshal([]byte(strings.TrimPrefix(raw, contentEnvelopePrefix)), &env); err == nil {
			display = env.DisplayText
		}
	}
	out, err := marshalMessageContent(blocks, display, ContextsFromStoredContent(raw))
	if err != nil {
		return "", false, err
	}
	return out, true, nil
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

// SaveTokenUsage accumulates session token totals and appends a ledger row.
// Ledger failures are logged and never fail the caller: the model has already
// been billed and the assistant step still needs to persist. providerID/modelID
// are the step-time ids; empty values fall back to the session's current model.
func (s *Service) SaveTokenUsage(ctx context.Context, sessionID string, u cometsdk.TokenUsage, providerID, modelID string) error {
	sess, err := s.GetSession(ctx, sessionID)
	if err != nil {
		logging.L().Warn("usage.session_lookup_failed", "session", sessionID, "error", err)
		s.recordAgentStep(ctx, sessionID, "", providerID, modelID, u)
		return nil
	}
	var current cometsdk.TokenUsage
	if strings.TrimSpace(sess.TokenUsage) != "" && sess.TokenUsage != "{}" {
		if unmarshalErr := json.Unmarshal([]byte(sess.TokenUsage), &current); unmarshalErr != nil {
			logging.L().Warn("usage.token_usage_json_invalid", "session", sessionID, "error", unmarshalErr)
			current = cometsdk.TokenUsage{}
		}
	}
	current.InputTokens += u.InputTokens
	current.OutputTokens += u.OutputTokens
	current.CacheRead += u.CacheRead
	current.CacheWrite += u.CacheWrite
	b, err := json.Marshal(current)
	if err != nil {
		return err
	}
	if err := s.q.UpdateSessionTokenUsage(ctx, db.UpdateSessionTokenUsageParams{
		TokenUsage: string(b),
		ID:         sessionID,
	}); err != nil {
		return err
	}
	if providerID == "" {
		providerID = sess.ProviderID
	}
	if modelID == "" {
		modelID = sess.ModelID
	}
	s.recordAgentStep(ctx, sessionID, sess.WorkspaceID, providerID, modelID, u)
	return nil
}

func (s *Service) recordAgentStep(ctx context.Context, sessionID, workspaceID, providerID, modelID string, u cometsdk.TokenUsage) {
	if s.usage == nil {
		return
	}
	if err := s.usage.Record(ctx, usage.Event{
		WorkspaceID: workspaceID,
		SessionID:   sessionID,
		ProviderID:  providerID,
		ModelID:     modelID,
		CallKind:    usage.KindAgentStep,
		Usage:       u,
	}); err != nil {
		logging.L().Warn("usage.record_failed", "kind", usage.KindAgentStep, "session", sessionID, "error", err)
	}
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
	for _, tc := range allCalls {
		callsByMessage[tc.MessageID] = append(callsByMessage[tc.MessageID], tc)
	}
	states, err := s.q.ListAssistantProviderStatesBySession(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	statesByMessage := make(map[string][]cometsdk.ProviderState, len(states))
	for _, state := range states {
		if state.ProviderID != providerID {
			continue
		}
		statesByMessage[state.MessageID] = append(statesByMessage[state.MessageID], cometsdk.ProviderState{
			ProviderID: state.ProviderID,
			ModelID:    state.ModelID,
			Data:       state.State,
		})
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
		DelegationStatus: DelegationPending.String(),
		OutputSummary:    "",
		SubagentKind:     subagentKind,
		AgentMode:        parent.AgentMode,
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
func (s *Service) UpdateDelegation(ctx context.Context, sessionID string, status DelegationStatus, summary string) error {
	return s.q.UpdateSessionDelegation(ctx, db.UpdateSessionDelegationParams{
		DelegationStatus: status.String(),
		OutputSummary:    summary,
		ID:               sessionID,
	})
}

// UpdateDelegationState persists delegation status, summary, and pending question.
func (s *Service) UpdateDelegationState(ctx context.Context, sessionID string, status DelegationStatus, summary, pendingQuestion string) error {
	return s.q.UpdateSessionDelegationState(ctx, db.UpdateSessionDelegationStateParams{
		DelegationStatus: status.String(),
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
