package memory

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/cometline/cometmind/internal/db"
	"github.com/oklog/ulid/v2"
)

type store struct {
	conn *sql.DB
	q    *db.Queries
}

func newStore(dbConn *sql.DB) *store {
	return &store{conn: dbConn, q: db.New(dbConn)}
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

func (s *store) listCompactionCandidates(ctx context.Context, limit int) ([]Record, error) {
	if limit <= 0 || limit > 200 {
		limit = 200
	}
	rows, err := s.q.ListCompactionCandidates(ctx, int64(limit))
	if err != nil {
		return nil, err
	}
	out := make([]Record, len(rows))
	for i, row := range rows {
		out[i] = recordFromDB(row)
	}
	return out, nil
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
	rec = normalizeStoredRecord(rec)
	if err := s.q.InsertMemory(ctx, insertMemoryParams(rec, now)); err != nil {
		return err
	}
	return s.upsertFTS(ctx, rec.ID, rec.Content)
}

func normalizeStoredRecord(rec Record) Record {
	applyPolicyInvariants(&rec)
	if rec.SummaryJSON == "" {
		rec.SummaryJSON = "{}"
	}
	return rec
}

func insertMemoryParams(rec Record, now int64) db.InsertMemoryParams {
	return db.InsertMemoryParams{
		ID:                 rec.ID,
		Scope:              rec.Scope,
		Kind:               rec.Kind,
		PreferenceCategory: normalizePreferenceCategory(rec.Kind, rec.Content, rec.PreferenceCategory),
		Content:            rec.Content,
		Embedding:          encodeEmbedding(rec.Embedding),
		EmbeddingModel:     nullString(rec.EmbeddingModel),
		Source:             rec.Source,
		BaseWeight:         rec.BaseWeight,
		AccessCount:        rec.AccessCount,
		ApplicationPolicy:  rec.ApplicationPolicy,
		RetentionPolicy:    rec.RetentionPolicy,
		OriginType:         rec.OriginType,
		OriginID:           rec.OriginID,
		SummaryJson:        rec.SummaryJSON,
		SourceSessionID:    nullString(rec.SourceSessionID),
		SupersededBy:       sql.NullString{},
		Archived:           0,
		ArchivedReason:     sql.NullString{},
		LastAccessedAt:     nullInt64MS(rec.LastAccessedAt),
		CreatedAt:          now,
		UpdatedAt:          now,
	}
}

func (s *store) withTx(ctx context.Context, fn func(*sql.Tx, *db.Queries) error) error {
	tx, err := s.conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := fn(tx, db.New(tx)); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *store) replaceWithMerged(ctx context.Context, merged Record, sources []Record, detail string) error {
	merged = normalizeStoredRecord(merged)
	now := time.Now().UnixMilli()
	return s.withTx(ctx, func(tx *sql.Tx, q *db.Queries) error {
		if err := q.InsertMemory(ctx, insertMemoryParams(merged, now)); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO memories_fts (memory_id, content) VALUES (?, ?)`, merged.ID, merged.Content); err != nil {
			return err
		}
		for _, source := range sources {
			if err := q.ArchiveMemory(ctx, db.ArchiveMemoryParams{
				ArchivedReason: nullString("compaction"), SupersededBy: nullString(merged.ID), UpdatedAt: now, ID: source.ID,
			}); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `DELETE FROM memories_fts WHERE memory_id = ?`, source.ID); err != nil {
				return err
			}
			if err := q.InsertMemoryEvent(ctx, db.InsertMemoryEventParams{
				ID: ulid.Make().String(), MemoryID: nullString(source.ID), Action: "compact_merge", Detail: detail, CreatedAt: now,
			}); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *store) update(ctx context.Context, rec Record) error {
	applyPolicyInvariants(&rec)
	if rec.SummaryJSON == "" {
		rec.SummaryJSON = "{}"
	}
	if err := s.q.UpdateMemory(ctx, db.UpdateMemoryParams{
		Kind:               rec.Kind,
		PreferenceCategory: normalizePreferenceCategory(rec.Kind, rec.Content, rec.PreferenceCategory),
		Content:            rec.Content,
		Embedding:          encodeEmbedding(rec.Embedding),
		EmbeddingModel:     nullString(rec.EmbeddingModel),
		BaseWeight:         rec.BaseWeight,
		ApplicationPolicy:  rec.ApplicationPolicy,
		RetentionPolicy:    rec.RetentionPolicy,
		OriginType:         rec.OriginType,
		OriginID:           rec.OriginID,
		SummaryJson:        rec.SummaryJSON,
		LastAccessedAt:     nullInt64MS(rec.LastAccessedAt),
		UpdatedAt:          time.Now().UnixMilli(),
		ID:                 rec.ID,
	}); err != nil {
		return err
	}
	return s.upsertFTS(ctx, rec.ID, rec.Content)
}

func (s *store) archive(ctx context.Context, id, reason, supersededBy string) error {
	if err := s.q.ArchiveMemory(ctx, db.ArchiveMemoryParams{
		ArchivedReason: nullString(reason),
		SupersededBy:   nullString(supersededBy),
		UpdatedAt:      time.Now().UnixMilli(),
		ID:             id,
	}); err != nil {
		return err
	}
	return s.deleteFTS(ctx, id)
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
	if err := s.deleteFTS(ctx, id); err != nil {
		return err
	}
	return s.q.DeleteMemory(ctx, id)
}

func (s *store) listArchivedOlderThan(ctx context.Context, beforeMS int64) ([]string, error) {
	return s.q.ListArchivedMemoryIDsOlderThan(ctx, beforeMS)
}

func (s *store) listBaselinePreferences(ctx context.Context, limit int) ([]Record, error) {
	if limit <= 0 {
		limit = 3
	}
	rows, err := s.q.ListBaselinePreferences(ctx, int64(limit))
	if err != nil {
		return nil, err
	}
	out := make([]Record, len(rows))
	for i, row := range rows {
		out[i] = recordFromDB(row)
	}
	return out, nil
}

func (s *store) listRecentByKind(ctx context.Context, kind string, limit int) ([]Record, error) {
	if limit <= 0 {
		limit = 3
	}
	rows, err := s.q.ListRecentMemoriesByKind(ctx, db.ListRecentMemoriesByKindParams{
		Kind:  normalizeKind(kind),
		Limit: int64(limit),
	})
	if err != nil {
		return nil, err
	}
	out := make([]Record, len(rows))
	for i, row := range rows {
		out[i] = recordFromDB(row)
	}
	return out, nil
}

func (s *store) listActivePreferencesByCategory(ctx context.Context, category string) ([]Record, error) {
	rows, err := s.q.ListActivePreferencesByCategory(ctx, category)
	if err != nil {
		return nil, err
	}
	out := make([]Record, len(rows))
	for i, row := range rows {
		out[i] = recordFromDB(row)
	}
	return out, nil
}

func (s *store) deleteMemoryEventsForMemory(ctx context.Context, memoryID string) (int64, error) {
	res, err := s.conn.ExecContext(ctx, `DELETE FROM memory_events WHERE memory_id = ?`, memoryID)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return n, nil
}

func (s *store) deleteMemoryEventsOlderThan(ctx context.Context, beforeMS int64) (int64, error) {
	res, err := s.conn.ExecContext(ctx, `DELETE FROM memory_events WHERE created_at < ?`, beforeMS)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return n, nil
}

func (s *store) purgeArchived(ctx context.Context, beforeMS int64) (memories int, events int, err error) {
	ids, err := s.listArchivedOlderThan(ctx, beforeMS)
	if err != nil {
		return 0, 0, err
	}
	for _, id := range ids {
		n, err := s.deleteMemoryEventsForMemory(ctx, id)
		if err != nil {
			return memories, events, err
		}
		events += int(n)
		if err := s.delete(ctx, id); err != nil {
			return memories, events, err
		}
		memories++
	}
	extra, err := s.deleteMemoryEventsOlderThan(ctx, beforeMS)
	if err != nil {
		return memories, events, err
	}
	events += int(extra)
	return memories, events, nil
}

func (s *store) logEvent(ctx context.Context, memoryID, action, detail string) error {
	return s.q.InsertMemoryEvent(ctx, db.InsertMemoryEventParams{
		ID:        ulid.Make().String(),
		MemoryID:  nullString(memoryID),
		Action:    action,
		Detail:    detail,
		CreatedAt: time.Now().UnixMilli(),
	})
}

func (s *store) upsertFTS(ctx context.Context, id, content string) error {
	if err := s.deleteFTS(ctx, id); err != nil {
		return err
	}
	_, err := s.conn.ExecContext(ctx,
		`INSERT INTO memories_fts (memory_id, content) VALUES (?, ?)`,
		id, content,
	)
	return err
}

func (s *store) deleteFTS(ctx context.Context, id string) error {
	_, err := s.conn.ExecContext(ctx,
		`DELETE FROM memories_fts WHERE memory_id = ?`,
		id,
	)
	return err
}

func (s *store) searchFTS(ctx context.Context, query string, limit int) ([]string, error) {
	match := buildFTSMatchQuery(query)
	if match == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = retrievalPoolSize
	}
	rows, err := s.conn.QueryContext(ctx, `
		SELECT memory_id
		FROM memories_fts
		WHERE memories_fts MATCH ?
		ORDER BY rank
		LIMIT ?
	`, match, limit)
	if err != nil {
		return nil, fmt.Errorf("fts search: %w", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
