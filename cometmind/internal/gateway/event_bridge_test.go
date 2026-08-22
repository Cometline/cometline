package gateway

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cometline/cometmind/internal/event"
)

func TestEventBridgeRetriesAndPreservesReplayOrder(t *testing.T) {
	var ready atomic.Bool
	firstAttempt := make(chan struct{})
	var firstAttemptOnce sync.Once
	var mu sync.Mutex
	var packets []bridgePacket
	flushed := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		firstAttemptOnce.Do(func() { close(firstAttempt) })
		if !ready.Load() {
			http.Error(w, "not ready", http.StatusServiceUnavailable)
			return
		}
		var packet bridgePacket
		if err := json.NewDecoder(r.Body).Decode(&packet); err != nil {
			t.Errorf("Decode() error = %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		mu.Lock()
		packets = append(packets, packet)
		mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	forwarder := (&EventBridge{BaseURL: server.URL, Client: server.Client()}).Start(
		context.Background(),
		"session-1",
		"run-1",
		func() { close(flushed) },
	)
	forwarder.Forward(event.TextDelta("replayed"))
	forwarder.Forward(event.Done())
	forwarder.Close()

	select {
	case <-firstAttempt:
	case <-time.After(time.Second):
		t.Fatal("bridge did not attempt initial delivery")
	}
	ready.Store(true)

	deadline := time.Now().Add(3 * time.Second)
	for {
		mu.Lock()
		count := len(packets)
		mu.Unlock()
		if count == 4 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("received %d packets, want 4", count)
		}
		time.Sleep(20 * time.Millisecond)
	}
	select {
	case <-flushed:
	case <-time.After(time.Second):
		t.Fatal("bridge did not release the run after flushing")
	}

	mu.Lock()
	defer mu.Unlock()
	if !packets[0].Start || packets[0].RunID != "run-1" {
		t.Fatalf("first packet = %#v", packets[0])
	}
	if packets[1].Event == nil || packets[1].Event.Kind != event.KindTextDelta || packets[1].Event.Delta != "replayed" {
		t.Fatalf("second packet = %#v", packets[1])
	}
	if packets[1].Sequence != 1 || packets[2].Sequence != 2 {
		t.Fatalf("event sequences = %d, %d, want 1, 2", packets[1].Sequence, packets[2].Sequence)
	}
	if packets[2].Event == nil || packets[2].Event.Kind != event.KindDone {
		t.Fatalf("third packet = %#v", packets[2])
	}
	if !packets[3].Finish || packets[3].RunID != "run-1" {
		t.Fatalf("fourth packet = %#v", packets[3])
	}
}

func TestEventBridgeStopsRetryingPermanentRunMismatch(t *testing.T) {
	var attempts atomic.Int64
	flushed := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"error":{"code":"session_run_mismatch","message":"stale run"}}`))
	}))
	defer server.Close()

	forwarder := (&EventBridge{BaseURL: server.URL, Client: server.Client()}).Start(
		context.Background(),
		"session-1",
		"stale-run",
		func() { close(flushed) },
	)
	forwarder.Close()

	select {
	case <-flushed:
	case <-time.After(time.Second):
		t.Fatal("bridge did not release the stale run")
	}
	time.Sleep(600 * time.Millisecond)
	if got := attempts.Load(); got != 1 {
		t.Fatalf("attempts = %d, want 1", got)
	}
}
