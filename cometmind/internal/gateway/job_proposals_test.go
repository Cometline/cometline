package gateway

import (
	"testing"
)

func TestParseJobProposalOutput(t *testing.T) {
	payload, ok := ParseJobProposalOutput(`{"status":"awaiting_workspace","description":"Fix auth","default_workspace":"/tmp/ws"}`)
	if !ok {
		t.Fatal("expected ok")
	}
	if payload.Description != "Fix auth" {
		t.Fatalf("description=%q", payload.Description)
	}
	if payload.DefaultWorkspace != "/tmp/ws" {
		t.Fatalf("default_workspace=%q", payload.DefaultWorkspace)
	}
}

func TestParseJobProposalOutputRejectsInvalid(t *testing.T) {
	if _, ok := ParseJobProposalOutput(`{"status":"done"}`); ok {
		t.Fatal("expected reject")
	}
	if _, ok := ParseJobProposalOutput(`not json`); ok {
		t.Fatal("expected reject")
	}
}

func TestJobProposalStorePutAndSelect(t *testing.T) {
	store := NewJobProposalStore()
	msg := InboundMessage{UserID: "u1", ChannelID: "c1"}
	pending := store.Put(msg, JobProposalPayload{
		Status:           "awaiting_workspace",
		Description:      "Task",
		DefaultWorkspace: "/default",
	}, "sess-1", "chan-1", "/default")
	if pending.SelectedWorkspace != "/default" {
		t.Fatalf("selected=%q", pending.SelectedWorkspace)
	}
	if !store.SetWorkspace(pending.ID, "/other") {
		t.Fatal("SetWorkspace failed")
	}
	got, ok := store.Get(pending.ID)
	if !ok || got.SelectedWorkspace != "/other" {
		t.Fatalf("got=%+v ok=%v", got, ok)
	}
}

func TestParseJobProposalCustomID(t *testing.T) {
	action, id, ok := ParseJobProposalCustomID("job_pc:abc123")
	if !ok || action != "confirm" || id != "abc123" {
		t.Fatalf("got action=%q id=%q ok=%v", action, id, ok)
	}
}
