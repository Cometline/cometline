package session

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/Cometline/cometline/cometmind/internal/media"
)

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
	if ephemeralSessionOrigin(sess.Origin) {
		return nil
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
