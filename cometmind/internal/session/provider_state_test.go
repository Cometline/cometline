package session

import (
	"context"
	"testing"

	cometsdk "github.com/cometline/comet-sdk"
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
