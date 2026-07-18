package memory

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/cometline/cometmind/internal/logging"
	"github.com/oklog/ulid/v2"
)

// ReembedJobStatus is the durable state of a background re-embedding run.
type ReembedJobStatus string

const (
	ReembedPending   ReembedJobStatus = "pending"
	ReembedRunning   ReembedJobStatus = "running"
	ReembedCompleted ReembedJobStatus = "completed"
	ReembedFailed    ReembedJobStatus = "failed"
	ReembedCancelled ReembedJobStatus = "cancelled"
)

// ReembedJob describes progress for switching embedding models without mixing vectors.
type ReembedJob struct {
	ID           string           `json:"id"`
	Status       ReembedJobStatus `json:"status"`
	FromModel    string           `json:"from_model"`
	ToProvider   string           `json:"to_provider"`
	ToModel      string           `json:"to_model"`
	ToBaseURL    string           `json:"to_base_url"`
	Total        int64            `json:"total"`
	Completed    int64            `json:"completed"`
	CursorID     string           `json:"cursor_id,omitempty"`
	Error        string           `json:"error,omitempty"`
	CreatedAt    int64            `json:"created_at"`
	UpdatedAt    int64            `json:"updated_at"`
	TargetAPIKey string           `json:"-"`
}

// ReembedPreview estimates how many active memories need re-embedding.
type ReembedPreview struct {
	ActiveCount     int64  `json:"active_count"`
	NeedsMigration  int64  `json:"needs_migration"`
	CurrentModel    string `json:"current_model"`
	RequestedModel  string `json:"requested_model"`
	MigrationNeeded bool   `json:"migration_needed"`
}

type reembedState struct {
	mu     sync.Mutex
	cancel context.CancelFunc
	job    *ReembedJob
}

func (s *Service) ensureReembedTable(ctx context.Context) error {
	_, err := s.store.conn.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS memory_reembed_jobs (
	id TEXT PRIMARY KEY,
	status TEXT NOT NULL,
	from_model TEXT NOT NULL DEFAULT '',
	to_provider TEXT NOT NULL DEFAULT '',
	to_model TEXT NOT NULL DEFAULT '',
	to_base_url TEXT NOT NULL DEFAULT '',
	to_api_key TEXT NOT NULL DEFAULT '',
	total INTEGER NOT NULL DEFAULT 0,
	completed INTEGER NOT NULL DEFAULT 0,
	cursor_id TEXT NOT NULL DEFAULT '',
	error TEXT NOT NULL DEFAULT '',
	created_at INTEGER NOT NULL,
	updated_at INTEGER NOT NULL
)`)
	return err
}

// PreviewReembed reports whether switching embeddings requires migration.
func (s *Service) PreviewReembed(ctx context.Context, target EmbeddingSettings) (ReembedPreview, error) {
	if s == nil {
		return ReembedPreview{}, fmt.Errorf("memory service is nil")
	}
	total, err := s.store.countActive(ctx)
	if err != nil {
		return ReembedPreview{}, err
	}
	current := strings.TrimSpace(s.settings.Embedding.Model)
	requested := strings.TrimSpace(target.Model)
	needs := int64(0)
	if requested != "" && total > 0 {
		rows, err := s.store.listActive(ctx)
		if err != nil {
			return ReembedPreview{}, err
		}
		for _, row := range rows {
			if strings.TrimSpace(row.EmbeddingModel) != requested {
				needs++
			}
		}
	}
	return ReembedPreview{
		ActiveCount:     total,
		NeedsMigration:  needs,
		CurrentModel:    current,
		RequestedModel:  requested,
		// Only require an explicit migration when switching away from the
		// currently active retrieval model while old vectors still exist.
		MigrationNeeded: needs > 0 && requested != "" && requested != current,
	}, nil
}

// CurrentReembedJob returns the latest job, if any.
func (s *Service) CurrentReembedJob(ctx context.Context) (*ReembedJob, error) {
	if s == nil {
		return nil, fmt.Errorf("memory service is nil")
	}
	if err := s.ensureReembedTable(ctx); err != nil {
		return nil, err
	}
	s.reembed.mu.Lock()
	if s.reembed.job != nil {
		job := *s.reembed.job
		s.reembed.mu.Unlock()
		return &job, nil
	}
	s.reembed.mu.Unlock()

	row := s.store.conn.QueryRowContext(ctx, `
SELECT id, status, from_model, to_provider, to_model, to_base_url, to_api_key, total, completed, cursor_id, error, created_at, updated_at
FROM memory_reembed_jobs
ORDER BY created_at DESC
LIMIT 1`)
	job, err := scanReembedJob(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &job, nil
}

// StartReembed queues a background re-embed. Retrieval keeps using the current
// embedder until the job completes successfully.
func (s *Service) StartReembed(ctx context.Context, target EmbeddingSettings) (*ReembedJob, error) {
	if s == nil {
		return nil, fmt.Errorf("memory service is nil")
	}
	if strings.TrimSpace(target.Model) == "" {
		return nil, fmt.Errorf("target embedding model is required")
	}
	preview, err := s.PreviewReembed(ctx, target)
	if err != nil {
		return nil, err
	}
	if !preview.MigrationNeeded {
		// Nothing to migrate — apply immediately.
		if err := s.UpdateSettings(s.settingsWithEmbedding(target)); err != nil {
			return nil, err
		}
		now := time.Now().UnixMilli()
		job := &ReembedJob{
			ID:         ulid.Make().String(),
			Status:     ReembedCompleted,
			FromModel:  preview.CurrentModel,
			ToProvider: target.Provider,
			ToModel:    target.Model,
			ToBaseURL:  target.BaseURL,
			Total:      0,
			Completed:  0,
			CreatedAt:  now,
			UpdatedAt:  now,
		}
		return job, nil
	}

	s.reembed.mu.Lock()
	if s.reembed.cancel != nil {
		s.reembed.mu.Unlock()
		return nil, fmt.Errorf("a re-embed job is already running")
	}
	s.reembed.mu.Unlock()

	if err := s.ensureReembedTable(ctx); err != nil {
		return nil, err
	}
	now := time.Now().UnixMilli()
	job := &ReembedJob{
		ID:           ulid.Make().String(),
		Status:       ReembedRunning,
		FromModel:    preview.CurrentModel,
		ToProvider:   strings.TrimSpace(target.Provider),
		ToModel:      strings.TrimSpace(target.Model),
		ToBaseURL:    strings.TrimSpace(target.BaseURL),
		TargetAPIKey: target.APIKey,
		Total:        preview.NeedsMigration,
		Completed:    0,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := s.persistReembedJob(ctx, job); err != nil {
		return nil, err
	}

	runCtx, cancel := context.WithCancel(context.Background())
	s.reembed.mu.Lock()
	s.reembed.cancel = cancel
	s.reembed.job = job
	s.reembed.mu.Unlock()

	go s.runReembed(runCtx, *job, target)
	out := *job
	return &out, nil
}

// CancelReembed aborts the active job if any.
func (s *Service) CancelReembed(ctx context.Context) (*ReembedJob, error) {
	if s == nil {
		return nil, fmt.Errorf("memory service is nil")
	}
	s.reembed.mu.Lock()
	cancel := s.reembed.cancel
	job := s.reembed.job
	s.reembed.mu.Unlock()
	if cancel == nil || job == nil {
		current, err := s.CurrentReembedJob(ctx)
		if err != nil {
			return nil, err
		}
		return current, nil
	}
	cancel()
	job.Status = ReembedCancelled
	job.UpdatedAt = time.Now().UnixMilli()
	job.Error = "cancelled by user"
	_ = s.persistReembedJob(ctx, job)
	out := *job
	return &out, nil
}

func (s *Service) settingsWithEmbedding(embedding EmbeddingSettings) Settings {
	next := s.settings
	next.Embedding = embedding
	return next
}

func (s *Service) runReembed(ctx context.Context, job ReembedJob, target EmbeddingSettings) {
	defer func() {
		s.reembed.mu.Lock()
		s.reembed.cancel = nil
		s.reembed.mu.Unlock()
	}()

	embedder, err := NewEmbedder(target)
	if err != nil {
		s.failReembed(ctx, &job, err)
		return
	}

	rows, err := s.store.listActive(ctx)
	if err != nil {
		s.failReembed(ctx, &job, err)
		return
	}

	for _, rec := range rows {
		if ctx.Err() != nil {
			job.Status = ReembedCancelled
			job.Error = "cancelled by user"
			job.UpdatedAt = time.Now().UnixMilli()
			_ = s.persistReembedJob(context.Background(), &job)
			s.reembed.mu.Lock()
			s.reembed.job = &job
			s.reembed.mu.Unlock()
			return
		}
		if strings.TrimSpace(rec.EmbeddingModel) == target.Model && len(rec.Embedding) > 0 {
			continue
		}
		vectors, err := embedder.Embed(ctx, rec.Content)
		if err != nil {
			s.failReembed(ctx, &job, err)
			return
		}
		if len(vectors) == 0 {
			s.failReembed(ctx, &job, fmt.Errorf("empty embedding for memory %s", rec.ID))
			return
		}
		rec.Embedding = vectors[0]
		rec.EmbeddingModel = target.Model
		if err := s.store.update(ctx, rec); err != nil {
			s.failReembed(ctx, &job, err)
			return
		}
		job.Completed++
		job.CursorID = rec.ID
		job.UpdatedAt = time.Now().UnixMilli()
		if job.Completed%5 == 0 || job.Completed == job.Total {
			_ = s.persistReembedJob(ctx, &job)
			s.reembed.mu.Lock()
			copied := job
			s.reembed.job = &copied
			s.reembed.mu.Unlock()
		}
	}

	// Switch retrieval/write embedder only after the index is coherent.
	if err := s.UpdateSettings(s.settingsWithEmbedding(target)); err != nil {
		s.failReembed(ctx, &job, err)
		return
	}
	job.Status = ReembedCompleted
	job.Error = ""
	job.UpdatedAt = time.Now().UnixMilli()
	_ = s.persistReembedJob(ctx, &job)
	s.reembed.mu.Lock()
	s.reembed.job = &job
	s.reembed.mu.Unlock()
	logging.L().Info("memory.reembed.completed", "job_id", job.ID, "to_model", job.ToModel, "completed", job.Completed)
}

func (s *Service) failReembed(ctx context.Context, job *ReembedJob, err error) {
	job.Status = ReembedFailed
	job.Error = err.Error()
	job.UpdatedAt = time.Now().UnixMilli()
	_ = s.persistReembedJob(ctx, job)
	s.reembed.mu.Lock()
	copied := *job
	s.reembed.job = &copied
	s.reembed.mu.Unlock()
	logging.L().Warn("memory.reembed.failed", "job_id", job.ID, "error", err.Error())
}

func (s *Service) persistReembedJob(ctx context.Context, job *ReembedJob) error {
	if err := s.ensureReembedTable(ctx); err != nil {
		return err
	}
	_, err := s.store.conn.ExecContext(ctx, `
INSERT INTO memory_reembed_jobs (
	id, status, from_model, to_provider, to_model, to_base_url, to_api_key,
	total, completed, cursor_id, error, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
	status=excluded.status,
	total=excluded.total,
	completed=excluded.completed,
	cursor_id=excluded.cursor_id,
	error=excluded.error,
	updated_at=excluded.updated_at
`, job.ID, string(job.Status), job.FromModel, job.ToProvider, job.ToModel, job.ToBaseURL, job.TargetAPIKey,
		job.Total, job.Completed, job.CursorID, job.Error, job.CreatedAt, job.UpdatedAt)
	return err
}

func scanReembedJob(row *sql.Row) (ReembedJob, error) {
	var job ReembedJob
	var status string
	err := row.Scan(
		&job.ID, &status, &job.FromModel, &job.ToProvider, &job.ToModel, &job.ToBaseURL, &job.TargetAPIKey,
		&job.Total, &job.Completed, &job.CursorID, &job.Error, &job.CreatedAt, &job.UpdatedAt,
	)
	if err != nil {
		return ReembedJob{}, err
	}
	job.Status = ReembedJobStatus(status)
	return job, nil
}
