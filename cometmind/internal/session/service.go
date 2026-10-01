package session

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/Cometline/cometline/cometmind/internal/db"
	"github.com/Cometline/cometline/cometmind/internal/usage"
)

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

func (s *Service) createMessage(ctx context.Context, params db.CreateMessageParams) (db.Message, error) {
	if err := s.q.MarkSessionNonDisposable(ctx, params.SessionID); err != nil {
		return db.Message{}, err
	}
	return s.q.CreateMessage(ctx, params)
}

func nullSessionID(id string) sql.NullString {
	id = strings.TrimSpace(id)
	if id == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: id, Valid: true}
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
