package event

import "sync"

// Hub broadcasts runtime events that are produced outside a request-scoped
// stream, such as agent-initiated background memory writes.
type Hub struct {
	mu          sync.Mutex
	subscribers map[chan Event]struct{}
}

// NewHub creates an event hub with no subscribers.
func NewHub() *Hub {
	return &Hub{subscribers: make(map[chan Event]struct{})}
}

// Subscription is a cancellable event-hub subscription.
type Subscription struct {
	Events <-chan Event
	close  func()
}

// Close removes the subscription and closes its event channel.
func (s *Subscription) Close() {
	if s == nil || s.close == nil {
		return
	}
	s.close()
	s.close = nil
}

// Subscribe registers a buffered subscriber. The caller must close the
// subscription when it no longer needs events.
func (h *Hub) Subscribe() *Subscription {
	if h == nil {
		return &Subscription{Events: make(chan Event)}
	}
	ch := make(chan Event, 32)
	h.mu.Lock()
	if h.subscribers == nil {
		h.subscribers = make(map[chan Event]struct{})
	}
	h.subscribers[ch] = struct{}{}
	h.mu.Unlock()

	return &Subscription{
		Events: ch,
		close: func() {
			h.mu.Lock()
			if _, ok := h.subscribers[ch]; ok {
				delete(h.subscribers, ch)
				close(ch)
			}
			h.mu.Unlock()
		},
	}
}

// Publish sends an event to every current subscriber. Slow subscribers do not
// block the agent or other subscribers; a later event supersedes a dropped
// notification for this UI-only channel.
func (h *Hub) Publish(ev Event) {
	if h == nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.subscribers {
		select {
		case ch <- ev:
		default:
		}
	}
}
