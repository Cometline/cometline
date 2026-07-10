package event

import (
	"testing"
	"time"
)

func TestHubPublishesAndClosesSubscriptions(t *testing.T) {
	hub := NewHub()
	sub := hub.Subscribe()
	hub.Publish(MemoryUpdated([]MemoryChangeWire{{Action: "create", Kind: "fact", Content: "likes tea"}}))

	select {
	case got := <-sub.Events:
		if got.Kind != KindMemoryUpdated || len(got.MemoryChanges) != 1 {
			t.Fatalf("unexpected event: %+v", got)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for published event")
	}

	sub.Close()
	select {
	case _, ok := <-sub.Events:
		if ok {
			t.Fatal("subscription channel should be closed")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for subscription close")
	}
}
