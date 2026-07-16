package tools

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/cometline/cometmind/internal/event"
	"github.com/cometline/cometmind/internal/inbox"
	"github.com/cometline/cometmind/internal/store"
)

func TestLeaveInboxMessageTool(t *testing.T) {
	ctx := context.Background()
	sqlDB, err := store.OpenSQLite(ctx, ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()

	svc := inbox.NewService(sqlDB)
	hub := event.NewHub()
	tool := leaveInboxMessageTool{deps: InboxDeps{Inbox: svc, Events: hub, SessionID: "sess-1"}}
	raw, _ := json.Marshal(map[string]string{
		"title":  "Note",
		"body":   "Details",
		"job_id": "job-9",
	})
	res, err := tool.Execute(ctx, raw)
	if err != nil {
		t.Fatal(err)
	}
	if !res.OK {
		t.Fatalf("result: %+v", res)
	}
	items, err := svc.List(ctx, inbox.ListFilter{Status: inbox.StatusOpen})
	if err != nil || len(items) != 1 {
		t.Fatalf("list: %v len=%d", err, len(items))
	}
	if items[0].JobID != "job-9" || items[0].SessionID != "sess-1" {
		t.Fatalf("provenance: %+v", items[0])
	}
}

func TestNewInboxProcessRegistryLimitedTools(t *testing.T) {
	r := NewInboxProcessRegistry(RegistryOptions{})
	if r.Has("run_command") || r.Has("write_file") || r.Has("leave_inbox_message") {
		t.Fatalf("inbox process registry must be limited, got unexpected tools")
	}
}
