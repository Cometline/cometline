package tools

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/cometline/cometmind/internal/event"
	"github.com/cometline/cometmind/internal/inbox"
	"github.com/cometline/cometmind/internal/session"
	"github.com/cometline/cometmind/internal/skills"
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

func TestNewInboxProcessRegistryUsesParentSurface(t *testing.T) {
	root := t.TempDir()
	skillReg := skills.Discover(root, skills.Config{Enabled: true})
	r := NewInboxProcessRegistry(root, RegistryOptions{Skills: &skillReg})
	for _, name := range []string{
		"read_file", "edit_file", "write_file", "run_command",
		"write_skill_draft", "list_skill_drafts", "read_skill_draft",
		"list_settings",
	} {
		if !r.Has(name) {
			t.Errorf("inbox process registry missing %q", name)
		}
	}
	for _, name := range []string{
		"write_skill", "promote_skill_draft",
		"spawn_general_agent", "wait_subagents", "delegate_coding_task",
	} {
		if r.Has(name) {
			t.Errorf("inbox process registry must not include %q", name)
		}
	}
}

func TestNewInboxProcessRegistryHonorsPlanMode(t *testing.T) {
	root := t.TempDir()
	skillReg := skills.Discover(root, skills.Config{Enabled: true})
	r := NewInboxProcessRegistry(root, RegistryOptions{
		Skills:    &skillReg,
		AgentMode: session.AgentModePlan,
	})
	if !r.Has("read_file") || !r.Has("load_skill") {
		t.Fatal("plan inbox registry missing read tools")
	}
	for _, name := range []string{"edit_file", "write_file", "run_command", "write_skill", "write_skill_draft"} {
		if r.Has(name) {
			t.Errorf("plan inbox registry must not include %q", name)
		}
	}
}
