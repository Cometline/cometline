package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/cometline/cometmind/internal/event"
)

const DefaultServeURL = "http://127.0.0.1:7700"

// EventBridge forwards gateway-owned run events to the serve process in order.
type EventBridge struct {
	BaseURL string
	Client  *http.Client
}

type bridgePacket struct {
	RunID    string       `json:"run_id"`
	Sequence uint64       `json:"sequence,omitempty"`
	Start    bool         `json:"start,omitempty"`
	Finish   bool         `json:"finish,omitempty"`
	Event    *event.Event `json:"event,omitempty"`
}

type EventForwarder struct {
	bridge    *EventBridge
	ctx       context.Context
	sessionID string
	runID     string
	mu        sync.Mutex
	queue     []bridgePacket
	nextSeq   uint64
	wake      chan struct{}
	closed    bool
	onFlushed func()
}

func (b *EventBridge) Start(ctx context.Context, sessionID, runID string, onFlushed func()) *EventForwarder {
	f := &EventForwarder{
		bridge:    b,
		ctx:       context.WithoutCancel(ctx),
		sessionID: sessionID,
		runID:     runID,
		wake:      make(chan struct{}, 1),
		queue:     []bridgePacket{{RunID: runID, Start: true}},
		onFlushed: onFlushed,
	}
	go f.run()
	f.signal()
	return f
}

func (f *EventForwarder) Forward(ev event.Event) {
	if f == nil {
		return
	}
	f.mu.Lock()
	if !f.closed {
		f.nextSeq++
		eventCopy := ev
		f.queue = append(f.queue, bridgePacket{RunID: f.runID, Sequence: f.nextSeq, Event: &eventCopy})
	}
	f.mu.Unlock()
	f.signal()
}

func (f *EventForwarder) Close() {
	if f == nil {
		return
	}
	f.mu.Lock()
	if !f.closed {
		f.closed = true
		f.queue = append(f.queue, bridgePacket{RunID: f.runID, Finish: true})
	}
	f.mu.Unlock()
	f.signal()
}

func (f *EventForwarder) signal() {
	select {
	case f.wake <- struct{}{}:
	default:
	}
}

func (f *EventForwarder) run() {
	defer func() {
		if f.onFlushed != nil {
			f.onFlushed()
		}
	}()
	for {
		f.mu.Lock()
		if len(f.queue) == 0 {
			closed := f.closed
			f.mu.Unlock()
			if closed {
				return
			}
			<-f.wake
			continue
		}
		packet := f.queue[0]
		f.mu.Unlock()

		status, err := f.send(packet)
		if err == nil && status >= 200 && status < 300 {
			f.mu.Lock()
			f.queue = f.queue[1:]
			f.mu.Unlock()
			continue
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func (f *EventForwarder) send(packet bridgePacket) (int, error) {
	raw, err := json.Marshal(packet)
	if err != nil {
		return 0, err
	}
	baseURL := f.bridge.BaseURL
	if baseURL == "" {
		baseURL = DefaultServeURL
	}
	ctx, cancel := context.WithTimeout(f.ctx, 2*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		fmt.Sprintf("%s/api/v1/sessions/%s/events", baseURL, f.sessionID),
		bytes.NewReader(raw),
	)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	client := f.bridge.Client
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	return resp.StatusCode, nil
}
