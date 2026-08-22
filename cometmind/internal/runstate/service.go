package runstate

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/cometline/cometmind/internal/db"
	"github.com/cometline/cometmind/internal/id"
)

const (
	OwnerHTTP    = "http"
	OwnerGateway = "gateway"

	heartbeatInterval = time.Second
	staleAfter        = 15 * time.Second
)

var ErrAlreadyRunning = errors.New("session already running")

// Service coordinates run ownership through the shared SQLite database.
type Service struct {
	q *db.Queries
}

func New(database *sql.DB) *Service {
	return &Service{q: db.New(database)}
}

// Lease owns one session run until Finish is called or its database row is lost.
type Lease struct {
	service   *Service
	sessionID string
	runID     string
	ctx       context.Context
	cancel    context.CancelFunc
	stop      chan struct{}
	done      chan struct{}
	once      sync.Once
	released  bool
}

func (l *Lease) Context() context.Context { return l.ctx }
func (l *Lease) RunID() string            { return l.runID }
func (l *Lease) Cancel()                  { l.cancel() }

func (l *Lease) Finish() bool {
	if l == nil {
		return false
	}
	l.once.Do(func() {
		close(l.stop)
		l.cancel()
		<-l.done
		rows, err := l.service.q.ReleaseSessionRun(context.Background(), db.ReleaseSessionRunParams{
			SessionID: l.sessionID,
			RunID:     l.runID,
		})
		l.released = err == nil && rows > 0
	})
	return l.released
}

// Acquire atomically claims the session across serve, gateway, and background workers.
func (s *Service) Acquire(parent context.Context, sessionID, owner string) (*Lease, error) {
	if s == nil || s.q == nil {
		return nil, fmt.Errorf("run state service is required")
	}
	if owner != OwnerHTTP && owner != OwnerGateway {
		return nil, fmt.Errorf("invalid run owner %q", owner)
	}
	runID := id.New()
	tryAcquire := func() (bool, error) {
		rows, err := s.q.AcquireSessionRun(parent, db.AcquireSessionRunParams{
			SessionID: sessionID,
			RunID:     runID,
			Owner:     owner,
		})
		return rows > 0, err
	}
	acquired, err := tryAcquire()
	if err != nil {
		return nil, fmt.Errorf("acquire session run: %w", err)
	}
	if !acquired {
		if err := s.pruneStale(parent, sessionID); err != nil {
			return nil, fmt.Errorf("prune stale session run: %w", err)
		}
		acquired, err = tryAcquire()
		if err != nil {
			return nil, fmt.Errorf("acquire session run: %w", err)
		}
	}
	if !acquired {
		return nil, fmt.Errorf("%w: %s", ErrAlreadyRunning, sessionID)
	}

	ctx, cancel := context.WithCancel(parent)
	lease := &Lease{
		service:   s,
		sessionID: sessionID,
		runID:     runID,
		ctx:       ctx,
		cancel:    cancel,
		stop:      make(chan struct{}),
		done:      make(chan struct{}),
	}
	go lease.watch()
	return lease, nil
}

func (l *Lease) watch() {
	defer close(l.done)
	ticker := time.NewTicker(heartbeatInterval)
	defer ticker.Stop()
	for {
		select {
		case <-l.stop:
			return
		case <-ticker.C:
			row, err := l.service.q.GetSessionRun(context.Background(), l.sessionID)
			if errors.Is(err, sql.ErrNoRows) || (err == nil && row.RunID != l.runID) {
				l.cancel()
				return
			}
			if err != nil {
				continue
			}
			if row.AbortRequested != 0 {
				l.cancel()
			}
			rows, err := l.service.q.HeartbeatSessionRun(context.Background(), db.HeartbeatSessionRunParams{
				SessionID: l.sessionID,
				RunID:     l.runID,
			})
			if err == nil && rows == 0 {
				l.cancel()
				return
			}
		}
	}
}

func (s *Service) pruneStale(ctx context.Context, sessionID string) error {
	_, err := s.q.DeleteStaleSessionRun(ctx, db.DeleteStaleSessionRunParams{
		SessionID: sessionID,
		UpdatedAt: time.Now().Add(-staleAfter).UnixMilli(),
	})
	return err
}

func (s *Service) RequestAbort(ctx context.Context, sessionID string) (bool, error) {
	if err := s.pruneStale(ctx, sessionID); err != nil {
		return false, err
	}
	rows, err := s.q.RequestSessionRunAbort(ctx, sessionID)
	return rows > 0, err
}

func (s *Service) Running(ctx context.Context, sessionID string) (bool, error) {
	if err := s.pruneStale(ctx, sessionID); err != nil {
		return false, err
	}
	return s.q.SessionRunExists(ctx, sessionID)
}

func (s *Service) Current(ctx context.Context, sessionID string) (db.SessionRun, error) {
	if err := s.pruneStale(ctx, sessionID); err != nil {
		return db.SessionRun{}, err
	}
	return s.q.GetSessionRun(ctx, sessionID)
}

// Release removes the matching lease without requiring access to its in-process handle.
func (s *Service) Release(ctx context.Context, sessionID, runID string) (bool, error) {
	rows, err := s.q.ReleaseSessionRun(ctx, db.ReleaseSessionRunParams{
		SessionID: sessionID,
		RunID:     runID,
	})
	return rows > 0, err
}
