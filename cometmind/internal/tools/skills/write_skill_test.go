package skills

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/Cometline/cometline/cometmind/internal/event"
)

func TestWriteSkillPublishesUpdate(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	hub := event.NewHub()
	sub := hub.Subscribe()
	defer sub.Close()

	content := "---\nname: ship-checklist\ndescription: Ship a reviewed change\n---\n\n# Steps\n"
	payload, err := json.Marshal(map[string]any{"name": "ship-checklist", "content": content})
	if err != nil {
		t.Fatal(err)
	}
	ctx := WithToolSession(t.Context(), "sess-1")
	res, err := (WriteSkill{Events: hub}).Execute(ctx, payload)
	if err != nil {
		t.Fatal(err)
	}
	if !res.OK {
		t.Fatalf("write failed: %s", res.Output)
	}

	select {
	case ev := <-sub.Events:
		if ev.Kind != event.KindSkillReviewUpdated || ev.SessionID != "sess-1" || len(ev.SkillReviews) != 1 {
			t.Fatalf("event = %+v", ev)
		}
		got := ev.SkillReviews[0]
		if got.Name != "ship-checklist" || got.Action != "created" || got.Description != "Ship a reviewed change" {
			t.Fatalf("change = %+v", got)
		}
	case <-time.After(time.Second):
		t.Fatal("missing skill update event")
	}
}
