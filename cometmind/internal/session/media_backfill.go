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
	known, err := s.catalogedMediaIDs(ctx, sessionID)
	if err != nil {
		return err
	}
	if !hasUncatalogedFile(files, known) {
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
	if err := s.backfillTranscriptMedia(ctx, sess, sessionID, known, hints); err != nil {
		return err
	}
	return s.backfillStoredMediaFiles(ctx, sess, files, known, hints)
}

func (s *Service) catalogedMediaIDs(ctx context.Context, sessionID string) (map[string]struct{}, error) {
	known := map[string]struct{}{}
	rows, err := s.q.ListSessionMediaBySession(ctx, nullSessionID(sessionID))
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		known[row.ID] = struct{}{}
	}
	return known, nil
}

func hasUncatalogedFile(files []media.FileInfo, known map[string]struct{}) bool {
	for _, file := range files {
		if _, ok := known[file.ID]; !ok {
			return true
		}
	}
	return false
}

// backfillTranscriptMedia catalogs media referenced by transcript blocks whose
// file still exists, adding each new ID to known.
func (s *Service) backfillTranscriptMedia(ctx context.Context, sess Session, sessionID string, known map[string]struct{}, hints map[string]MediaMeta) error {
	messages, err := s.q.ListMessagesBySession(ctx, sessionID)
	if err != nil {
		return err
	}
	for _, msg := range messages {
		for _, block := range envelopeMediaBlocks(msg.Content) {
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
	return nil
}

// backfillStoredMediaFiles catalogs on-disk files that neither the catalog
// nor the transcript referenced.
func (s *Service) backfillStoredMediaFiles(ctx context.Context, sess Session, files []media.FileInfo, known map[string]struct{}, hints map[string]MediaMeta) error {
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
