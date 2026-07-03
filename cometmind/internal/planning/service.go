package planning

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/cometline/cometmind/internal/db"
	"github.com/cometline/cometmind/internal/id"
)

var ErrInvalidInput = errors.New("invalid plan input")

type Service struct {
	db *sql.DB
	q  *db.Queries
}

func NewService(conn *sql.DB) *Service {
	return &Service{db: conn, q: db.New(conn)}
}

func nowMillis() int64 { return time.Now().UnixMilli() }

func normalizeStatus(status string) (string, error) {
	status = strings.TrimSpace(status)
	if status == "" {
		return StatusPending, nil
	}
	switch status {
	case StatusPending, StatusInProgress, StatusCompleted, StatusBlocked:
		return status, nil
	default:
		return "", fmt.Errorf("%w: unknown status %q", ErrInvalidInput, status)
	}
}

func stepFromDB(row db.SessionPlan) Step {
	return Step{
		ID:            row.ID,
		SessionID:     row.SessionID,
		StepIndex:     row.StepIndex,
		Description:   row.Description,
		Status:        row.Status,
		BlockerReason: row.BlockerReason,
		CreatedAt:     row.CreatedAt,
		UpdatedAt:     row.UpdatedAt,
	}
}

// SetPlan replaces the session's current plan with the supplied ordered steps.
func (s *Service) SetPlan(ctx context.Context, sessionID string, steps []StepInput) ([]Step, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("planning service unavailable")
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return nil, fmt.Errorf("%w: session_id is required", ErrInvalidInput)
	}
	if len(steps) == 0 {
		return nil, fmt.Errorf("%w: at least one step is required", ErrInvalidInput)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	q := s.q.WithTx(tx)
	if err := q.DeletePlanForSession(ctx, sessionID); err != nil {
		return nil, err
	}
	ts := nowMillis()
	for i, step := range steps {
		desc := strings.TrimSpace(step.Description)
		if desc == "" {
			return nil, fmt.Errorf("%w: step %d description is required", ErrInvalidInput, i)
		}
		status, err := normalizeStatus(step.Status)
		if err != nil {
			return nil, err
		}
		if err := q.InsertPlanStep(ctx, db.InsertPlanStepParams{
			ID:            id.New(),
			SessionID:     sessionID,
			StepIndex:     int64(i),
			Description:   desc,
			Status:        status,
			BlockerReason: strings.TrimSpace(step.BlockerReason),
			CreatedAt:     ts,
			UpdatedAt:     ts,
		}); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return s.GetPlan(ctx, sessionID)
}

func (s *Service) GetPlan(ctx context.Context, sessionID string) ([]Step, error) {
	if s == nil || s.q == nil {
		return nil, fmt.Errorf("planning service unavailable")
	}
	rows, err := s.q.ListPlanSteps(ctx, strings.TrimSpace(sessionID))
	if err != nil {
		return nil, err
	}
	steps := make([]Step, 0, len(rows))
	for _, row := range rows {
		steps = append(steps, stepFromDB(row))
	}
	return steps, nil
}

func (s *Service) UpdateStep(ctx context.Context, sessionID string, stepIndex int64, status, blockerReason string) ([]Step, error) {
	if s == nil || s.q == nil {
		return nil, fmt.Errorf("planning service unavailable")
	}
	if stepIndex < 0 {
		return nil, fmt.Errorf("%w: step_index must be non-negative", ErrInvalidInput)
	}
	status, err := normalizeStatus(status)
	if err != nil {
		return nil, err
	}
	changed, err := s.q.UpdatePlanStep(ctx, db.UpdatePlanStepParams{
		Status:        status,
		BlockerReason: strings.TrimSpace(blockerReason),
		UpdatedAt:     nowMillis(),
		SessionID:     strings.TrimSpace(sessionID),
		StepIndex:     stepIndex,
	})
	if err != nil {
		return nil, err
	}
	if changed == 0 {
		return nil, sql.ErrNoRows
	}
	return s.GetPlan(ctx, sessionID)
}

func FormatPromptBlock(steps []Step) string {
	if len(steps) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("## Current Plan\n")
	b.WriteString("Use `plan_write` to replace this plan and `plan_update` after meaningful progress.\n")
	for _, step := range steps {
		fmt.Fprintf(&b, "%d. [%s] %s", step.StepIndex, step.Status, step.Description)
		if step.BlockerReason != "" {
			fmt.Fprintf(&b, " (blocked: %s)", step.BlockerReason)
		}
		b.WriteByte('\n')
	}
	return strings.TrimSpace(b.String())
}
