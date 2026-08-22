package event

import (
	"testing"
	"time"

	cometsdk "github.com/cometline/comet-sdk"
)

func TestSessionHubReplaysMergedSnapshotThenTails(t *testing.T) {
	hub := NewSessionHub()
	hub.Start("session-1", "run-1")
	if !hub.Publish("session-1", "run-1", TextDelta("hello ")) {
		t.Fatal("Publish() = false")
	}
	hub.Publish("session-1", "run-1", TextDelta("world"))
	hub.Publish("session-1", "run-1", ReasoningStart())
	hub.Publish("session-1", "run-1", ReasoningDelta("thinking"))

	sub, ok := hub.Subscribe("session-1", "run-1")
	if !ok {
		t.Fatal("Subscribe() = false")
	}
	defer sub.Close()
	if len(sub.Replay) != 3 {
		t.Fatalf("Replay length = %d, want 3", len(sub.Replay))
	}
	if got := sub.Replay[0]; got.Kind != KindTextDelta || got.Delta != "hello world" {
		t.Fatalf("first replay event = %#v", got)
	}

	done := Done()
	go hub.Publish("session-1", "run-1", done)
	select {
	case got := <-sub.Events:
		if got.Kind != KindDone {
			t.Fatalf("tail event = %#v, want done", got)
		}
	case <-time.After(time.Second):
		t.Fatal("subscriber did not receive live event")
	}

	if hub.Publish("session-1", "another-run", TextDelta("stale")) {
		t.Fatal("stale run Publish() = true")
	}
}

func TestSessionHubSlowSubscriberDoesNotBlockPublisher(t *testing.T) {
	hub := NewSessionHub()
	hub.Start("session-1", "run-1")
	sub, ok := hub.Subscribe("session-1", "run-1")
	if !ok {
		t.Fatal("Subscribe() = false")
	}
	defer sub.Close()

	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 1000; i++ {
			hub.Publish("session-1", "run-1", ToolResult("tool", "test", "output", ""))
		}
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("publisher blocked behind slow subscriber")
	}
}

func TestSessionHubReplacingRunClosesOldSubscription(t *testing.T) {
	hub := NewSessionHub()
	hub.Start("session-1", "run-1")
	sub, ok := hub.Subscribe("session-1", "run-1")
	if !ok {
		t.Fatal("Subscribe() = false")
	}
	hub.Start("session-1", "run-2")

	select {
	case _, ok := <-sub.Events:
		if ok {
			t.Fatal("old subscription remained open")
		}
	case <-time.After(time.Second):
		t.Fatal("old subscription was not closed")
	}
}

func TestSessionHubAuthoritativeSubscriptionReplacesStaleRun(t *testing.T) {
	hub := NewSessionHub()
	hub.Start("session-1", "stale-run")
	stale, ok := hub.Subscribe("session-1", "stale-run")
	if !ok {
		t.Fatal("stale Subscribe() = false")
	}

	current, ok := hub.Subscribe("session-1", "current-run")
	if !ok {
		t.Fatal("current Subscribe() = false")
	}
	defer current.Close()
	if !hub.Current("session-1", "current-run") {
		t.Fatal("authoritative subscription did not replace stale run")
	}
	select {
	case _, open := <-stale.Events:
		if open {
			t.Fatal("stale subscription remained open")
		}
	case <-time.After(time.Second):
		t.Fatal("stale subscription was not closed")
	}
	if !hub.Start("session-1", "current-run") {
		t.Fatal("producer Start() after authoritative subscription = false")
	}
}

func TestSessionHubFinishTerminatesSubscriberWithoutPublishedDone(t *testing.T) {
	hub := NewSessionHub()
	hub.Start("session-1", "run-1")
	sub, ok := hub.Subscribe("session-1", "run-1")
	if !ok {
		t.Fatal("Subscribe() = false")
	}
	defer sub.Close()

	if !hub.Finish("session-1", "run-1") {
		t.Fatal("Finish() = false")
	}
	select {
	case got, open := <-sub.Events:
		if !open || got.Kind != KindDone {
			t.Fatalf("terminal event = %#v, open=%v; want done", got, open)
		}
	case <-time.After(time.Second):
		t.Fatal("subscriber did not receive synthesized done")
	}
	select {
	case _, open := <-sub.Events:
		if open {
			t.Fatal("subscription remained open after done")
		}
	case <-time.After(time.Second):
		t.Fatal("subscription was not closed after done")
	}
}

func TestSessionHubProducerStartAfterEarlySubscriberIsNew(t *testing.T) {
	hub := NewSessionHub()
	sub, ok := hub.Subscribe("session-1", "run-1")
	if !ok {
		t.Fatal("Subscribe() = false")
	}
	defer sub.Close()
	if !hub.Start("session-1", "run-1") {
		t.Fatal("first producer Start() = false")
	}
	if hub.Start("session-1", "run-1") {
		t.Fatal("duplicate producer Start() = true")
	}
}

func TestSessionHubDeduplicatesSequencedGatewayEvents(t *testing.T) {
	hub := NewSessionHub()
	hub.Start("session-1", "run-1")
	accepted, published := hub.PublishSequenced("session-1", "run-1", 1, TextDelta("once"))
	if !accepted || !published {
		t.Fatalf("first PublishSequenced() = %v, %v", accepted, published)
	}
	accepted, published = hub.PublishSequenced("session-1", "run-1", 1, TextDelta("once"))
	if !accepted || published {
		t.Fatalf("duplicate PublishSequenced() = %v, %v", accepted, published)
	}
	sub, ok := hub.Subscribe("session-1", "run-1")
	if !ok {
		t.Fatal("Subscribe() = false")
	}
	defer sub.Close()
	if len(sub.Replay) != 1 || sub.Replay[0].Delta != "once" {
		t.Fatalf("Replay = %#v", sub.Replay)
	}
}

func TestSessionHubDropsPersistedStepsFromReplay(t *testing.T) {
	hub := NewSessionHub()
	hub.Start("session-1", "run-1")
	hub.Publish("session-1", "run-1", TextDelta("persisted step"))
	hub.Publish("session-1", "run-1", StepFinish(cometsdk.TokenUsage{}))
	hub.Publish("session-1", "run-1", TurnStatus(PhaseRunningTools, ""))
	hub.Publish("session-1", "run-1", TextDelta("current step"))

	sub, ok := hub.Subscribe("session-1", "run-1")
	if !ok {
		t.Fatal("Subscribe() = false")
	}
	defer sub.Close()
	if len(sub.Replay) != 2 || sub.Replay[0].Kind != KindTurnStatus || sub.Replay[1].Delta != "current step" {
		t.Fatalf("Replay = %#v", sub.Replay)
	}

	hub.Publish("session-1", "run-1", Done())
	finished, ok := hub.Subscribe("session-1", "run-1")
	if !ok {
		t.Fatal("finished Subscribe() = false")
	}
	defer finished.Close()
	if len(finished.Replay) != 1 || finished.Replay[0].Kind != KindDone {
		t.Fatalf("finished replay = %#v", finished.Replay)
	}
}
