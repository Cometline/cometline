package agent

import (
	"encoding/json"
	"testing"

	cometsdk "github.com/cometline/comet-sdk"
)

func TestIsDoomLoop_RequiresThresholdIdenticalCalls(t *testing.T) {
	t.Parallel()
	same := fingerprint("read_file", `{"path":"a.pdf"}`)
	other := fingerprint("read_file", `{"path":"b.pdf"}`)

	if IsDoomLoop([]ToolFingerprint{same, same}, DoomLoopThreshold) {
		t.Fatal("two identical calls must not trip")
	}
	if !IsDoomLoop([]ToolFingerprint{same, same, same}, DoomLoopThreshold) {
		t.Fatal("three identical calls must trip")
	}
	if IsDoomLoop([]ToolFingerprint{same, same, other}, DoomLoopThreshold) {
		t.Fatal("different input must not trip")
	}
	if IsDoomLoop([]ToolFingerprint{same, fingerprint("list_dir", `{"path":"a.pdf"}`), same}, DoomLoopThreshold) {
		t.Fatal("different tool name must not trip")
	}
}

func TestIsDoomLoop_IgnoresJSONKeyOrder(t *testing.T) {
	t.Parallel()
	a := fingerprint("read_file", `{"path":"x","offset":1}`)
	b := fingerprint("read_file", `{"offset":1,"path":"x"}`)
	c := fingerprint("read_file", `{"path":"x","offset":1}`)
	if !IsDoomLoop([]ToolFingerprint{a, b, c}, DoomLoopThreshold) {
		t.Fatal("canonical JSON equality should treat key order as identical")
	}
}

func TestToolFingerprintsSinceLastUser_ResetsOnUser(t *testing.T) {
	t.Parallel()
	msgs := []cometsdk.Message{
		{Role: cometsdk.RoleUser, Content: []cometsdk.Block{cometsdk.TextBlock{Text: "old"}}},
		{Role: cometsdk.RoleAssistant, Content: []cometsdk.Block{
			cometsdk.ToolCallBlock{Name: "read_file", Input: json.RawMessage(`{"path":"old.txt"}`)},
		}},
		{Role: cometsdk.RoleUser, Content: []cometsdk.Block{cometsdk.TextBlock{Text: "new"}}},
		{Role: cometsdk.RoleAssistant, Content: []cometsdk.Block{
			cometsdk.ToolCallBlock{Name: "read_file", Input: json.RawMessage(`{"path":"new.txt"}`)},
		}},
	}
	got := toolFingerprintsSinceLastUser(msgs)
	if len(got) != 1 || got[0].Name != "read_file" {
		t.Fatalf("got %#v, want the post-user tool only", got)
	}
	if !toolInputsEqual(mustRaw(`{"path":"new.txt"}`), mustRaw(`{"path":"new.txt"}`)) {
		t.Fatal("expected matching input")
	}
	other := fingerprint("read_file", `{"path":"old.txt"}`)
	if SameTool(got[0], other) {
		t.Fatal("should not include tools from before the latest user message")
	}
}

func fingerprint(name, input string) ToolFingerprint {
	return FingerprintTool(name, json.RawMessage(input))
}

func mustRaw(s string) json.RawMessage {
	return json.RawMessage(s)
}
