package session

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/Cometline/cometline/cometmind/internal/db"
	"github.com/Cometline/cometline/cometmind/internal/id"
	"github.com/Cometline/cometline/cometmind/internal/media"
)

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

	if err := s.copyTranscript(ctx, sessionID, forked.ID); err != nil {
		return Session{}, err
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

// copyTranscript copies every message and tool call of srcSessionID into
// destSessionID under fresh IDs.
func (s *Service) copyTranscript(ctx context.Context, srcSessionID, destSessionID string) error {
	msgs, err := s.q.ListMessagesBySession(ctx, srcSessionID)
	if err != nil {
		return err
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
				return err
			}
			content = remapped
		}
		newMsg, err := s.createMessage(ctx, db.CreateMessageParams{
			ID:               id.New(),
			SessionID:        destSessionID,
			Role:             msg.Role,
			Content:          content,
			ReasoningContent: msg.ReasoningContent,
			TokenCount:       msg.TokenCount,
		})
		if err != nil {
			return err
		}
		if err := s.copyToolCalls(ctx, msg.ID, newMsg.ID, toolCallIDMap); err != nil {
			return err
		}
	}
	return nil
}

// copyToolCalls copies the tool calls of srcMessageID onto destMessageID and
// records each old→new tool-call ID in idMap.
func (s *Service) copyToolCalls(ctx context.Context, srcMessageID, destMessageID string, idMap map[string]string) error {
	calls, err := s.q.ListToolCallsByMessage(ctx, srcMessageID)
	if err != nil {
		return err
	}
	for _, call := range calls {
		newCallID := id.New()
		idMap[call.ID] = newCallID
		if _, err := s.q.CreateToolCall(ctx, db.CreateToolCallParams{
			ID:         newCallID,
			MessageID:  destMessageID,
			ToolName:   call.ToolName,
			Arguments:  call.Arguments,
			Result:     call.Result,
			DurationMs: call.DurationMs,
			ExitCode:   call.ExitCode,
		}); err != nil {
			return err
		}
	}
	return nil
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
