package memory

import (
	"context"
	"database/sql"
	"time"

	"github.com/cometline/cometmind/internal/db"
	"github.com/oklog/ulid/v2"
)

type store struct {
	q *db.Queries
}

func newStore(dbConn *sql.DB) *store {
	return &store{q: db.New(dbConn)}
}

func (s *store) listActive(ctx context.Context) ([]Record, error) {
	rows, err := s.q.ListActiveMemories(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Record, len(rows))
	for i, row := range rows {
		out[i] = recordFromDB(row)
	}
	return out, nil
}

func (s *store) countActive(ctx context.Context) (int64, error) {
	return s.q.CountActiveMemories(ctx)
}

func (s *store) get(ctx context.Context, id string) (Record, error) {
	row, err := s.q.GetMemory(ctx, id)
	if err != nil {
		return Record{}, err
	}
	return recordFromDB(row), nil
}

func (s *store) insert(ctx context.Context, rec Record) error {
	now := time.Now().UnixMilli()
	return s.q.InsertMemory(ctx, db.InsertMemoryParams{
		ID:              rec.ID,
		Scope:           rec.Scope,
		Kind:            rec.Kind,
		Content:         rec.Content,
		Embedding:       encodeEmbedding(rec.Embedding),
		EmbeddingModel:  nullString(rec.EmbeddingModel),
		Source:          rec.Source,
		BaseWeight:      rec.BaseWeight,
		AccessCount:     rec.AccessCount,
		Pinned:          boolToInt64(rec.Pinned),
		SourceSessionID: nullString(rec.SourceSessionID),
		SupersededBy:    sql.NullString{},
		Archived:        0,
		ArchivedReason:  sql.NullString{},
		LastAccessedAt:  nullInt64MS(rec.LastAccessedAt),
		CreatedAt:       now,
		UpdatedAt:       now,
	})
}

func (s *store) update(ctx context.Context, rec Record) error {
	return s.q.UpdateMemory(ctx, db.UpdateMemoryParams{
		Kind:           rec.Kind,
		Content:        rec.Content,
		Embedding:      encodeEmbedding(rec.Embedding),
		EmbeddingModel: nullString(rec.EmbeddingModel),
		BaseWeight:     rec.BaseWeight,
		Pinned:         boolToInt64(rec.Pinned),
		LastAccessedAt: nullInt64MS(rec.LastAccessedAt),
		UpdatedAt:      time.Now().UnixMilli(),
		ID:             rec.ID,
	})
}

func (s *store) archive(ctx context.Context, id, reason, supersededBy string) error {
	return s.q.ArchiveMemory(ctx, db.ArchiveMemoryParams{
		ArchivedReason: nullString(reason),
		SupersededBy:   nullString(supersededBy),
		UpdatedAt:      time.Now().UnixMilli(),
		ID:             id,
	})
}

func (s *store) touchAccess(ctx context.Context, id string) error {
	now := time.Now().UnixMilli()
	return s.q.TouchMemoryAccess(ctx, db.TouchMemoryAccessParams{
		LastAccessedAt: sql.NullInt64{Int64: now, Valid: true},
		UpdatedAt:      now,
		ID:             id,
	})
}

func (s *store) delete(ctx context.Context, id string) error {
	return s.q.DeleteMemory(ctx, id)
}

func (s *store) logEvent(ctx context.Context, memoryID, action, detail string) error {
	return s.q.InsertMemoryEvent(ctx, db.InsertMemoryEventParams{
		ID:       ulid.Make().String(),
		MemoryID: nullString(memoryID),
		Action:   action,
		Detail:   detail,
		CreatedAt: time.Now().UnixMilli(),
	})
}

func boolToInt64(v bool) int64 {
	if v {
		return 1
	}
	return 0
}
