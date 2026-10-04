package subagent

import (
	"context"
	"fmt"
	"sync"
)

// Kind identifies a subagent execution path.
type Kind string

const (
	KindGeneral Kind = "general"
	KindACP     Kind = "acp"
)

// Result is the terminal outcome of one subagent run.
type Result struct {
	ChildSessionID string
	Kind           Kind
	Status         string // completed | failed | cancelled
	Summary        string
}

type handle struct {
	parentID string
	kind     Kind
	done     chan Result
	cancel   context.CancelFunc
	// claimed is set when a Wait snapshot holds this handle. Complete then
	// delivers on done instead of the pending queue so the result is not
	// returned twice.
	claimed bool
}

// Orchestrator tracks in-flight subagents and coordinates wait/join.
type Orchestrator struct {
	mu       sync.Mutex
	maxPer   int
	children map[string]*handle
	byParent map[string]map[string]struct{}
	// pending holds results that finished before any Wait claimed the child.
	// ActiveCount ignores them; finish and wait_subagents still observe them.
	pending map[string][]Result
}

// NewOrchestrator returns an orchestrator with the given per-parent concurrency cap.
// maxPerParent <= 0 defaults to 5.
func NewOrchestrator(maxPerParent int) *Orchestrator {
	if maxPerParent <= 0 {
		maxPerParent = 5
	}
	return &Orchestrator{
		maxPer:   maxPerParent,
		children: make(map[string]*handle),
		byParent: make(map[string]map[string]struct{}),
		pending:  make(map[string][]Result),
	}
}

// Register records a new in-flight subagent. cancel is invoked on parent abort.
func (o *Orchestrator) Register(parentID, childID string, kind Kind, cancel context.CancelFunc) error {
	if parentID == "" || childID == "" {
		return fmt.Errorf("parent and child session ids are required")
	}
	o.mu.Lock()
	defer o.mu.Unlock()

	if _, exists := o.children[childID]; exists {
		return fmt.Errorf("child session %s already registered", childID)
	}
	active := o.byParent[parentID]
	if len(active) >= o.maxPer {
		return fmt.Errorf("max concurrent subagents (%d) reached for parent %s", o.maxPer, parentID)
	}

	h := &handle{
		parentID: parentID,
		kind:     kind,
		done:     make(chan Result, 1),
		cancel:   cancel,
	}
	o.children[childID] = h
	if active == nil {
		active = make(map[string]struct{})
		o.byParent[parentID] = active
	}
	active[childID] = struct{}{}
	return nil
}

// Complete signals completion and removes the child from the active set.
// A result that finishes before Wait claims the child is kept until a later
// Wait, so a parent that stops after the child has already exited still sees it.
func (o *Orchestrator) Complete(childID string, res Result) {
	o.mu.Lock()
	h, ok := o.children[childID]
	if !ok {
		o.mu.Unlock()
		return
	}
	delete(o.children, childID)
	if active := o.byParent[h.parentID]; active != nil {
		delete(active, childID)
		if len(active) == 0 {
			delete(o.byParent, h.parentID)
		}
	}
	res.ChildSessionID = childID
	res.Kind = h.kind
	claimed := h.claimed
	if !claimed {
		o.pending[h.parentID] = append(o.pending[h.parentID], res)
	}
	o.mu.Unlock()

	if !claimed {
		return
	}
	select {
	case h.done <- res:
	default:
	}
}

// Wait blocks until all requested children of parentID complete.
// If childIDs is empty, waits for all currently registered children of the parent.
// Children that already finished are returned immediately from the pending queue.
func (o *Orchestrator) Wait(ctx context.Context, parentID string, childIDs []string) ([]Result, error) {
	handles := o.snapshotHandles(parentID, childIDs)
	inflight, err := waitHandles(ctx, handles)
	if err != nil {
		return inflight, err
	}
	// A cancelled turn with nothing in flight must not consume results that a
	// later wait can still deliver, and must not look like a subagent failure.
	if ctx.Err() != nil {
		return inflight, nil
	}
	return append(o.takePending(parentID, childIDs), inflight...), nil
}

func waitHandles(ctx context.Context, handles map[string]*handle) ([]Result, error) {
	if len(handles) == 0 {
		return nil, nil
	}

	out := make(chan Result, len(handles))
	var wg sync.WaitGroup
	wg.Add(len(handles))
	for id, h := range handles {
		go func(id string, h *handle) {
			defer wg.Done()
			select {
			case res := <-h.done:
				if res.ChildSessionID == "" {
					res.ChildSessionID = id
				}
				out <- res
			case <-ctx.Done():
			}
		}(id, h)
	}

	go func() {
		wg.Wait()
		close(out)
	}()

	results := make([]Result, 0, len(handles))
	for res := range out {
		results = append(results, res)
		if len(results) == len(handles) {
			break
		}
	}

	if ctx.Err() != nil {
		return results, ctx.Err()
	}
	return results, nil
}

func (o *Orchestrator) snapshotHandles(parentID string, childIDs []string) map[string]*handle {
	o.mu.Lock()
	defer o.mu.Unlock()

	out := make(map[string]*handle)
	if len(childIDs) == 0 {
		for id := range o.byParent[parentID] {
			if h, ok := o.children[id]; ok {
				h.claimed = true
				out[id] = h
			}
		}
		return out
	}
	for _, id := range childIDs {
		if h, ok := o.children[id]; ok && h.parentID == parentID {
			h.claimed = true
			out[id] = h
		}
	}
	return out
}

// takePending removes completed-but-unclaimed results for parentID.
// An empty childIDs list takes every pending result for that parent.
func (o *Orchestrator) takePending(parentID string, childIDs []string) []Result {
	o.mu.Lock()
	defer o.mu.Unlock()

	queued := o.pending[parentID]
	if len(queued) == 0 {
		return nil
	}
	if len(childIDs) == 0 {
		delete(o.pending, parentID)
		return queued
	}
	want := make(map[string]struct{}, len(childIDs))
	for _, id := range childIDs {
		want[id] = struct{}{}
	}
	taken := make([]Result, 0, len(queued))
	kept := make([]Result, 0, len(queued))
	for _, res := range queued {
		if _, ok := want[res.ChildSessionID]; ok {
			taken = append(taken, res)
			continue
		}
		kept = append(kept, res)
	}
	if len(kept) == 0 {
		delete(o.pending, parentID)
	} else {
		o.pending[parentID] = kept
	}
	return taken
}

// CancelForParent cancels all in-flight subagents for a parent session.
func (o *Orchestrator) CancelForParent(parentID string) {
	o.mu.Lock()
	var cancels []context.CancelFunc
	for id := range o.byParent[parentID] {
		if h, ok := o.children[id]; ok && h.cancel != nil {
			cancels = append(cancels, h.cancel)
		}
	}
	o.mu.Unlock()

	for _, cancel := range cancels {
		cancel()
	}
}

// CancelChild cancels one in-flight subagent by child session ID.
func (o *Orchestrator) CancelChild(childID string) bool {
	o.mu.Lock()
	h, ok := o.children[childID]
	var cancel context.CancelFunc
	if ok {
		cancel = h.cancel
	}
	o.mu.Unlock()
	if !ok || cancel == nil {
		return false
	}
	cancel()
	return true
}

// ActiveCount returns how many subagents are in flight for a parent.
func (o *Orchestrator) ActiveCount(parentID string) int {
	o.mu.Lock()
	defer o.mu.Unlock()
	return len(o.byParent[parentID])
}
