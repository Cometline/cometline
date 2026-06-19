package memory

import (
	"database/sql"
	"time"

	"github.com/cometline/cometmind/internal/db"
)

// Record is the domain view of a memory row.
type Record struct {
	ID                 string
	Scope              string
	Kind               string
	PreferenceCategory string
	Content            string
	Embedding          []float32
	EmbeddingModel     string
	Source             string
	BaseWeight         float64
	AccessCount        int64
	Pinned             bool
	SourceSessionID    string
	Archived           bool
	ArchivedReason     string
	LastAccessedAt     *time.Time
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

func recordFromDB(row db.Memory) Record {
	r := Record{
		ID:                 row.ID,
		Scope:              row.Scope,
		Kind:               row.Kind,
		PreferenceCategory: row.PreferenceCategory,
		Content:            row.Content,
		Embedding:          decodeEmbedding(row.Embedding),
		Source:             row.Source,
		BaseWeight:         row.BaseWeight,
		AccessCount:        row.AccessCount,
		Pinned:             row.Pinned == 1,
		Archived:           row.Archived == 1,
		CreatedAt:          time.UnixMilli(row.CreatedAt),
		UpdatedAt:          time.UnixMilli(row.UpdatedAt),
	}
	if row.EmbeddingModel.Valid {
		r.EmbeddingModel = row.EmbeddingModel.String
	}
	if row.SourceSessionID.Valid {
		r.SourceSessionID = row.SourceSessionID.String
	}
	if row.ArchivedReason.Valid {
		r.ArchivedReason = row.ArchivedReason.String
	}
	if row.LastAccessedAt.Valid {
		t := time.UnixMilli(row.LastAccessedAt.Int64)
		r.LastAccessedAt = &t
	}
	return r
}

func nullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}

func nullInt64MS(t *time.Time) sql.NullInt64 {
	if t == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: t.UnixMilli(), Valid: true}
}
