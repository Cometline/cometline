package session

import (
	"context"
	"testing"

	cometsdk "github.com/Cometline/cometline/comet-sdk"
	"github.com/Cometline/cometline/cometmind/internal/db"
)

func TestAssistantProviderStateReplaysOnlyThroughSDKMessages(t *testing.T) {
	ctx := context.Background()
	svc, _ := newForkTestService(t)
	workspace, err := svc.EnsureWorkspace(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	sess, err := svc.NewSession(ctx, workspace.ID, "gpt-5.6-luna", "custom-codex")
	if err != nil {
		t.Fatal(err)
	}
	assistant, _, err := svc.AppendAssistantStep(ctx, sess.ID, "Working on it.", nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.SaveAssistantProviderState(ctx, assistant.ID, []cometsdk.ProviderState{{
		ProviderID: "custom-codex",
		ModelID:    "gpt-5.6-luna",
		Data:       "opaque-state",
	}}); err != nil {
		t.Fatal(err)
	}

	messages, err := svc.BuildSDKMessages(ctx, sess.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) != 1 || len(messages[0].ProviderState) != 1 {
		t.Fatalf("provider state was not rebuilt: %#v", messages)
	}
	if messages[0].ProviderState[0].Data != "opaque-state" {
		t.Fatalf("provider state = %q", messages[0].ProviderState[0].Data)
	}

	if err := svc.ClearAssistantProviderState(ctx, sess.ID); err != nil {
		t.Fatal(err)
	}
	messages, err = svc.BuildSDKMessages(ctx, sess.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(messages[0].ProviderState) != 0 {
		t.Fatalf("provider state survived cleanup: %#v", messages[0].ProviderState)
	}
}

func TestProviderStatesByMessageKeepsOnlyActiveProvider(t *testing.T) {
	got := providerStatesByMessage([]db.ListAssistantProviderStatesBySessionRow{
		{MessageID: "m1", ProviderID: "codex", ModelID: "gpt", State: "a"},
		{MessageID: "m1", ProviderID: "other", ModelID: "x", State: "skip"},
		{MessageID: "m1", ProviderID: "codex", ModelID: "gpt", State: "b"},
		{MessageID: "m2", ProviderID: "other", ModelID: "x", State: "skip"},
	}, "codex")
	if len(got) != 1 || len(got["m1"]) != 2 || got["m1"][0].Data != "a" || got["m1"][1].Data != "b" {
		t.Fatalf("states = %#v, want m1:[a b]", got)
	}
}
