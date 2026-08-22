package event

import "sync"

const maxFinishedRuns = 1024

type sessionStream struct {
	runID        string
	replay       []Event
	subscribers  map[*sessionSubscriber]struct{}
	lastSeq      uint64
	stepFinished bool
	started      bool
}

type sessionSubscriber struct {
	mu     sync.Mutex
	queue  []Event
	events chan Event
	wake   chan struct{}
	stop   chan struct{}
	done   chan struct{}
	closed bool
	once   sync.Once
}

func newSessionSubscriber() *sessionSubscriber {
	s := &sessionSubscriber{
		events: make(chan Event, 256),
		wake:   make(chan struct{}, 1),
		stop:   make(chan struct{}),
		done:   make(chan struct{}),
	}
	go s.run()
	return s
}

func (s *sessionSubscriber) enqueue(ev Event) {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.queue = append(s.queue, ev)
	s.mu.Unlock()
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

func (s *sessionSubscriber) run() {
	defer close(s.done)
	defer close(s.events)
	for {
		s.mu.Lock()
		if len(s.queue) > 0 {
			ev := s.queue[0]
			s.queue = s.queue[1:]
			s.mu.Unlock()
			select {
			case s.events <- ev:
			case <-s.stop:
				return
			}
			continue
		}
		closed := s.closed
		s.mu.Unlock()
		if closed {
			return
		}
		select {
		case <-s.wake:
		case <-s.stop:
			return
		}
	}
}

func (s *sessionSubscriber) close() {
	s.once.Do(func() {
		s.mu.Lock()
		s.closed = true
		s.mu.Unlock()
		close(s.stop)
		<-s.done
	})
}

func (s *sessionSubscriber) finish(ev *Event) {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	if ev != nil {
		s.queue = append(s.queue, *ev)
	}
	s.closed = true
	s.mu.Unlock()
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

// SessionHub keeps a replayable, lossless event stream for each active session run.
type SessionHub struct {
	mu       sync.Mutex
	streams  map[string]*sessionStream
	finished map[string]struct{}
}

func NewSessionHub() *SessionHub {
	return &SessionHub{
		streams:  make(map[string]*sessionStream),
		finished: make(map[string]struct{}),
	}
}

func finishedRunKey(sessionID, runID string) string {
	return sessionID + "\x00" + runID
}

// Start creates the stream for runID. Starting the same run is idempotent so a
// gateway subscriber may arrive before the gateway's first ingest request.
func (h *SessionHub) Start(sessionID, runID string) bool {
	if h == nil {
		return false
	}
	h.mu.Lock()
	if current := h.streams[sessionID]; current != nil && current.runID == runID {
		if !current.started {
			current.started = true
			h.mu.Unlock()
			return true
		}
		h.mu.Unlock()
		return false
	}
	var stale []*sessionSubscriber
	if current := h.streams[sessionID]; current != nil {
		for subscriber := range current.subscribers {
			stale = append(stale, subscriber)
		}
	}
	h.streams[sessionID] = &sessionStream{
		runID:       runID,
		subscribers: make(map[*sessionSubscriber]struct{}),
		started:     true,
	}
	h.mu.Unlock()
	for _, subscriber := range stale {
		subscriber.close()
	}
	return true
}

// Finish removes replay state for a released run while retaining a bounded
// idempotency marker for a retried terminal bridge packet.
func (h *SessionHub) Finish(sessionID, runID string) bool {
	if h == nil {
		return false
	}
	h.mu.Lock()
	stream := h.streams[sessionID]
	if stream == nil || stream.runID != runID {
		h.mu.Unlock()
		return false
	}
	var terminal *Event
	if len(stream.replay) == 0 || stream.replay[len(stream.replay)-1].Kind != KindDone {
		done := Done()
		appendReplay(stream, done)
		terminal = &done
	}
	subscribers := make([]*sessionSubscriber, 0, len(stream.subscribers))
	for subscriber := range stream.subscribers {
		subscribers = append(subscribers, subscriber)
	}
	delete(h.streams, sessionID)
	if len(h.finished) >= maxFinishedRuns {
		for staleSessionID := range h.finished {
			delete(h.finished, staleSessionID)
			break
		}
	}
	h.finished[finishedRunKey(sessionID, runID)] = struct{}{}
	h.mu.Unlock()
	for _, subscriber := range subscribers {
		subscriber.finish(terminal)
	}
	return true
}

func (h *SessionHub) Finished(sessionID, runID string) bool {
	if h == nil {
		return false
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	_, ok := h.finished[finishedRunKey(sessionID, runID)]
	return ok
}

// Publish appends to replay before fan-out. Consecutive token deltas are merged
// to bound replay overhead without changing reducer-visible ordering.
func (h *SessionHub) Publish(sessionID, runID string, ev Event) bool {
	if h == nil {
		return false
	}
	h.mu.Lock()
	stream := h.streams[sessionID]
	if stream == nil || stream.runID != runID {
		h.mu.Unlock()
		return false
	}
	appendReplay(stream, ev)
	subscribers := make([]*sessionSubscriber, 0, len(stream.subscribers))
	for subscriber := range stream.subscribers {
		subscribers = append(subscribers, subscriber)
	}
	h.mu.Unlock()
	for _, subscriber := range subscribers {
		subscriber.enqueue(ev)
	}
	return true
}

// PublishSequenced makes retrying gateway delivery idempotent within one run.
func (h *SessionHub) PublishSequenced(sessionID, runID string, sequence uint64, ev Event) (accepted, published bool) {
	if h == nil || sequence == 0 {
		return false, false
	}
	h.mu.Lock()
	stream := h.streams[sessionID]
	if stream == nil || stream.runID != runID {
		h.mu.Unlock()
		return false, false
	}
	if sequence <= stream.lastSeq {
		h.mu.Unlock()
		return true, false
	}
	stream.lastSeq = sequence
	appendReplay(stream, ev)
	subscribers := make([]*sessionSubscriber, 0, len(stream.subscribers))
	for subscriber := range stream.subscribers {
		subscribers = append(subscribers, subscriber)
	}
	h.mu.Unlock()
	for _, subscriber := range subscribers {
		subscriber.enqueue(ev)
	}
	return true, true
}

func (h *SessionHub) Current(sessionID, runID string) bool {
	if h == nil {
		return false
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	stream := h.streams[sessionID]
	return stream != nil && stream.runID == runID
}

func appendMerged(events []Event, ev Event) []Event {
	if len(events) == 0 {
		return append(events, ev)
	}
	last := &events[len(events)-1]
	switch {
	case ev.Kind == KindTextDelta && last.Kind == KindTextDelta:
		last.Delta += ev.Delta
		return events
	case ev.Kind == KindReasoningDelta && last.Kind == KindReasoningDelta:
		last.Text += ev.Text
		return events
	case ev.Kind == KindTurnStatus && last.Kind == KindTurnStatus:
		*last = ev
		return events
	default:
		return append(events, ev)
	}
}

func appendReplay(stream *sessionStream, ev Event) {
	if ev.Kind == KindDone {
		stream.replay = []Event{ev}
		stream.stepFinished = false
		return
	}
	if ev.Kind == KindTurnStatus && stream.stepFinished {
		// A status after step_finish is emitted only after that step has been
		// persisted. Reloaded clients get it from the transcript, not replay.
		stream.replay = nil
		stream.stepFinished = false
	}
	stream.replay = appendMerged(stream.replay, ev)
	if ev.Kind == KindStepFinish {
		stream.stepFinished = true
	}
}

type SessionSubscription struct {
	Replay []Event
	Events <-chan Event
	close  func()
}

func (s *SessionSubscription) Close() {
	if s == nil || s.close == nil {
		return
	}
	s.close()
	s.close = nil
}

func (h *SessionHub) Subscribe(sessionID, runID string) (*SessionSubscription, bool) {
	subscriber := newSessionSubscriber()
	h.mu.Lock()
	stream := h.streams[sessionID]
	var stale []*sessionSubscriber
	if stream == nil {
		stream = &sessionStream{
			runID:       runID,
			subscribers: make(map[*sessionSubscriber]struct{}),
		}
		h.streams[sessionID] = stream
	} else if stream.runID != runID {
		for oldSubscriber := range stream.subscribers {
			stale = append(stale, oldSubscriber)
		}
		stream = &sessionStream{
			runID:       runID,
			subscribers: make(map[*sessionSubscriber]struct{}),
		}
		h.streams[sessionID] = stream
	}
	replay := append([]Event(nil), stream.replay...)
	stream.subscribers[subscriber] = struct{}{}
	h.mu.Unlock()
	for _, oldSubscriber := range stale {
		oldSubscriber.close()
	}
	return &SessionSubscription{
		Replay: replay,
		Events: subscriber.events,
		close: func() {
			h.mu.Lock()
			if current := h.streams[sessionID]; current != nil && current.runID == runID {
				delete(current.subscribers, subscriber)
			}
			h.mu.Unlock()
			subscriber.close()
		},
	}, true
}
