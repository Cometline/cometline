package runtime

import (
	"context"
	"sync"
)

// supervisor tracks the runtime's background goroutines so Close can cancel
// them with one call and wait for every one to return before releasing the
// database and other shared resources.
type supervisor struct {
	mu      sync.Mutex
	stopped bool
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

func newSupervisor() *supervisor {
	ctx, cancel := context.WithCancel(context.Background())
	return &supervisor{ctx: ctx, cancel: cancel}
}

// Go runs fn in a new goroutine. fn's context is canceled when parent is done
// or the supervisor stops, whichever comes first. Go reports false and does
// not run fn once the supervisor has stopped. A nil supervisor runs fn
// unsupervised with parent as its context.
func (s *supervisor) Go(parent context.Context, fn func(ctx context.Context)) bool {
	if s == nil {
		go fn(parent)
		return true
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stopped {
		return false
	}
	ctx, cancel := context.WithCancel(parent)
	stop := context.AfterFunc(s.ctx, cancel)
	s.wg.Go(func() {
		defer stop()
		defer cancel()
		fn(ctx)
	})
	return true
}

// Stop cancels every goroutine started by Go and waits for them to return.
// It is safe to call more than once.
func (s *supervisor) Stop() {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.stopped = true
	s.mu.Unlock()
	s.cancel()
	s.wg.Wait()
}
