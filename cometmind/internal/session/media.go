package session

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Cometline/cometline/cometmind/internal/db"
	"github.com/Cometline/cometline/cometmind/internal/id"
	"github.com/Cometline/cometline/cometmind/internal/media"
)

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
		// Ephemeral runs keep the blob in the transcript so the model can
		// still see it, but they are not a user collection. Publishing them
		// into session_media would put private run artifacts on Gallery.
		if !ephemeralSessionOrigin(sess.Origin) {
			created, err := s.ensureSessionMedia(ctx, sess, block, meta)
			if err != nil {
				return Message{}, err
			}
			if created {
				createdIDs = append(createdIDs, block.ID)
			}
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

// PurgeDetachedMedia deletes files whose owning session has been gone past the retention window.
func (s *Service) PurgeDetachedMedia(ctx context.Context, retentionDays int) (int, error) {
	if retentionDays <= 0 {
		return 0, nil
	}
	now := time.Now()
	if err := s.q.InitializeDetachedSessionMedia(ctx, now.UnixMilli()); err != nil {
		return 0, err
	}
	cutoff := now.Add(-time.Duration(retentionDays) * 24 * time.Hour).UnixMilli()
	ids, err := s.q.ListExpiredDetachedSessionMediaIDs(ctx, cutoff)
	if err != nil {
		return 0, err
	}
	deleted := 0
	for _, mediaID := range ids {
		if _, err := s.DeleteMedia(ctx, mediaID); err != nil {
			return deleted, err
		}
		deleted++
	}
	return deleted, nil
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
