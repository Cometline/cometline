package inbox

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/cometline/cometmind/internal/db"
	"github.com/cometline/cometmind/internal/id"
)

// Service manages the global agent inbox.
type Service struct {
	q *db.Queries
}

// NewService creates an inbox service.
func NewService(conn *sql.DB) *Service {
	return &Service{q: db.New(conn)}
}

func nowMillis() int64 {
	return time.Now().UnixMilli()
}

func nullString(v string) sql.NullString {
	v = strings.TrimSpace(v)
	if v == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: v, Valid: true}
}

func nullStringVal(v sql.NullString) string {
	if !v.Valid {
		return ""
	}
	return v.String
}

func nullInt64Ptr(v sql.NullInt64) *int64 {
	if !v.Valid {
		return nil
	}
	x := v.Int64
	return &x
}

func messageFromRow(row db.InboxMessage) Message {
	return Message{
		ID:              row.ID,
		Title:           row.Title,
		Body:            row.Body,
		WorkspaceID:     nullStringVal(row.WorkspaceID),
		JobID:           nullStringVal(row.JobID),
		SessionID:       nullStringVal(row.SessionID),
		Status:          row.Status,
		ArchiveReason:   nullStringVal(row.ArchiveReason),
		UserReply:       nullStringVal(row.UserReply),
		ProcessedAt:     nullInt64Ptr(row.ProcessedAt),
		ProcessError:    nullStringVal(row.ProcessError),
		ProcessAttempts: row.ProcessAttempts,
		ArchivedAt:      nullInt64Ptr(row.ArchivedAt),
		DeletedAt:       nullInt64Ptr(row.DeletedAt),
		CreatedAt:       row.CreatedAt,
		UpdatedAt:       row.UpdatedAt,
	}
}

// Create leaves a new open inbox message.
func (s *Service) Create(ctx context.Context, in CreateInput) (Message, error) {
	title := strings.TrimSpace(in.Title)
	body := strings.TrimSpace(in.Body)
	if title == "" || body == "" {
		return Message{}, fmt.Errorf("%w: title and body are required", ErrInvalidInput)
	}
	now := nowMillis()
	row, err := s.q.CreateInboxMessage(ctx, db.CreateInboxMessageParams{
		ID:          id.New(),
		Title:       title,
		Body:        body,
		WorkspaceID: nullString(in.WorkspaceID),
		JobID:       nullString(in.JobID),
		SessionID:   nullString(in.SessionID),
		CreatedAt:   now,
		UpdatedAt:   now,
	})
	if err != nil {
		return Message{}, err
	}
	return messageFromRow(row), nil
}

// Get loads one non-deleted inbox message.
func (s *Service) Get(ctx context.Context, messageID string) (Message, error) {
	row, err := s.q.GetInboxMessage(ctx, messageID)
	if err != nil {
		if err == sql.ErrNoRows {
			return Message{}, ErrNotFound
		}
		return Message{}, err
	}
	return messageFromRow(row), nil
}

// List returns inbox messages matching filter.
func (s *Service) List(ctx context.Context, filter ListFilter) ([]Message, error) {
	var status sql.NullString
	if strings.TrimSpace(filter.Status) != "" {
		status = sql.NullString{String: filter.Status, Valid: true}
	}
	rows, err := s.q.ListInboxMessages(ctx, status)
	if err != nil {
		return nil, err
	}
	out := make([]Message, 0, len(rows))
	for _, row := range rows {
		out = append(out, messageFromRow(row))
	}
	return out, nil
}

// CountOpen returns how many open messages exist (for UI badge).
func (s *Service) CountOpen(ctx context.Context) (int64, error) {
	return s.q.CountOpenInboxMessages(ctx)
}

// Reply archives an open message with the user's reply.
func (s *Service) Reply(ctx context.Context, messageID, content string) (Message, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return Message{}, fmt.Errorf("%w: reply content is required", ErrInvalidInput)
	}
	now := nowMillis()
	row, err := s.q.ReplyInboxMessage(ctx, db.ReplyInboxMessageParams{
		UserReply:  sql.NullString{String: content, Valid: true},
		ArchivedAt: sql.NullInt64{Int64: now, Valid: true},
		UpdatedAt:  now,
		ID:         messageID,
	})
	if err != nil {
		if err == sql.ErrNoRows {
			if _, getErr := s.Get(ctx, messageID); getErr == ErrNotFound {
				return Message{}, ErrNotFound
			}
			return Message{}, ErrNotOpen
		}
		return Message{}, err
	}
	return messageFromRow(row), nil
}

// Dismiss archives an open message without a reply (no background processing).
func (s *Service) Dismiss(ctx context.Context, messageID string) (Message, error) {
	now := nowMillis()
	row, err := s.q.DismissInboxMessage(ctx, db.DismissInboxMessageParams{
		ArchivedAt: sql.NullInt64{Int64: now, Valid: true},
		UpdatedAt:  now,
		ID:         messageID,
	})
	if err != nil {
		if err == sql.ErrNoRows {
			if _, getErr := s.Get(ctx, messageID); getErr == ErrNotFound {
				return Message{}, ErrNotFound
			}
			return Message{}, ErrNotOpen
		}
		return Message{}, err
	}
	return messageFromRow(row), nil
}

// ListPendingProcess returns replied messages awaiting internalization.
func (s *Service) ListPendingProcess(ctx context.Context, limit int) ([]Message, error) {
	if limit <= 0 {
		limit = 10
	}
	rows, err := s.q.ListInboxMessagesPendingProcess(ctx, db.ListInboxMessagesPendingProcessParams{
		ProcessAttempts: MaxProcessAttempts,
		Limit:           int64(limit),
	})
	if err != nil {
		return nil, err
	}
	out := make([]Message, 0, len(rows))
	for _, row := range rows {
		out = append(out, messageFromRow(row))
	}
	return out, nil
}

// ClaimForProcess increments attempts for one pending message.
func (s *Service) ClaimForProcess(ctx context.Context, messageID string) (Message, error) {
	row, err := s.q.ClaimInboxMessageForProcess(ctx, db.ClaimInboxMessageForProcessParams{
		UpdatedAt:       nowMillis(),
		ID:              messageID,
		ProcessAttempts: MaxProcessAttempts,
	})
	if err != nil {
		if err == sql.ErrNoRows {
			return Message{}, ErrAlreadyClaimed
		}
		return Message{}, err
	}
	return messageFromRow(row), nil
}

// MarkProcessed records that background internalization finished.
func (s *Service) MarkProcessed(ctx context.Context, messageID, processError string) (Message, error) {
	now := nowMillis()
	row, err := s.q.MarkInboxMessageProcessed(ctx, db.MarkInboxMessageProcessedParams{
		ProcessedAt:  sql.NullInt64{Int64: now, Valid: true},
		ProcessError: nullString(processError),
		UpdatedAt:    now,
		ID:           messageID,
	})
	if err != nil {
		if err == sql.ErrNoRows {
			return Message{}, ErrNotFound
		}
		return Message{}, err
	}
	return messageFromRow(row), nil
}

// PurgeExpired hard-deletes archived messages older than retentionHours.
func (s *Service) PurgeExpired(ctx context.Context, retentionHours int) (int64, error) {
	if retentionHours <= 0 {
		retentionHours = 24
	}
	cutoff := time.Now().Add(-time.Duration(retentionHours) * time.Hour).UnixMilli()
	return s.q.DeleteExpiredInboxMessages(ctx, sql.NullInt64{Int64: cutoff, Valid: true})
}
