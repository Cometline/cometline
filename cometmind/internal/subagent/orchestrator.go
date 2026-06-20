package subagent

import (
	"context"
	"fmt"
	"reflect"
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
}

// Orchestrator tracks in-flight subagents and coordinates wait/join.
type Orchestrator struct {
	mu       sync.Mutex
	maxPer   int
	children map[string]*handle
	byParent map[string]map[string]struct{}
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
func (o *Orchestrator) Complete(childID string, res Result) {
	o.mu.Lock()
	h, ok := o.children[childID]
	if ok {
		delete(o.children, childID)
		if active := o.byParent[h.parentID]; active != nil {
			delete(active, childID)
			if len(active) == 0 {
				delete(o.byParent, h.parentID)
			}
		}
	}
	o.mu.Unlock()

	if !ok {
		return
	}
	res.ChildSessionID = childID
	res.Kind = h.kind
	select {
	case h.done <- res:
	default:
	}
}

// Wait blocks until all requested children of parentID complete.
// If childIDs is empty, waits for all currently registered children of the parent.
func (o *Orchestrator) Wait(ctx context.Context, parentID string, childIDs []string) ([]Result, error) {
	handles := o.snapshotHandles(parentID, childIDs)
	if len(handles) == 0 {
		return nil, nil
	}

	results := make([]Result, 0, len(handles))
	remaining := make(map[string]chan Result, len(handles))
	for id, h := range handles {
		remaining[id] = h.done
	}

	for len(remaining) > 0 {
		ids := make([]string, 0, len(remaining))
		cases := make([]reflect.SelectCase, 0, len(remaining)+1)
		cases = append(cases, reflect.SelectCase{
			Dir:  reflect.SelectRecv,
			Chan: reflect.ValueOf(ctx.Done()),
		})
		for id, ch := range remaining {
			ids = append(ids, id)
			cases = append(cases, reflect.SelectCase{
				Dir:  reflect.SelectRecv,
				Chan: reflect.ValueOf(ch),
			})
		}

		chosen, recv, _ := reflect.Select(cases)
		if chosen == 0 {
			return results, ctx.Err()
		}
		id := ids[chosen-1]
		res := recv.Interface().(Result)
		if res.ChildSessionID == "" {
			res.ChildSessionID = id
		}
		results = append(results, res)
		delete(remaining, id)
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
				out[id] = h
			}
		}
		return out
	}
	for _, id := range childIDs {
		if h, ok := o.children[id]; ok && h.parentID == parentID {
			out[id] = h
		}
	}
	return out
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

// ActiveCount returns how many subagents are in flight for a parent.
func (o *Orchestrator) ActiveCount(parentID string) int {
	o.mu.Lock()
	defer o.mu.Unlock()
	return len(o.byParent[parentID])
}
