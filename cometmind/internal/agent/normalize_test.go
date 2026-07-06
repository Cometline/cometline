package agent

import (
	"encoding/json"
	"testing"

	cometsdk "github.com/cometline/comet-sdk"
)

func assistantWithReasoning() cometsdk.Message {
	return cometsdk.Message{
		Role: cometsdk.RoleAssistant,
		Content: []cometsdk.Block{
			cometsdk.TextBlock{Text: "final answer"},
			cometsdk.ToolCallBlock{ID: "c1", Name: "read", Input: json.RawMessage(`{"path":"main.go"}`)},
		},
		ReasoningContent: []cometsdk.Block{
			cometsdk.ReasoningBlock{Text: "let me think\nstep by step"},
		},
	}
}

func TestNormalizeHistory_PreservesReasoning(t *testing.T) {
	in := []cometsdk.Message{assistantWithReasoning()}
	out, degradations := NormalizeHistory(in)
	if len(degradations) != 0 {
		t.Fatalf("expected no degradations, got %v", degradations)
	}
	if len(out[0].ReasoningContent) != 1 {
		t.Fatalf("reasoning content was modified")
	}
	if len(out[0].Content) != 2 {
		t.Fatalf("content was modified, got %d blocks", len(out[0].Content))
	}
}

func TestNormalizeHistory_DropsEmptyAssistant(t *testing.T) {
	in := []cometsdk.Message{
		{Role: cometsdk.RoleUser, Content: []cometsdk.Block{cometsdk.TextBlock{Text: "hi"}}},
		{Role: cometsdk.RoleAssistant},
		{Role: cometsdk.RoleAssistant, Content: []cometsdk.Block{cometsdk.TextBlock{Text: "hello"}}},
	}

	out, degradations := NormalizeHistory(in)
	if len(out) != 2 {
		t.Fatalf("expected empty assistant dropped, got %d messages", len(out))
	}
	if len(degradations) == 0 || degradations[len(degradations)-1].Kind != "empty_assistant_dropped" {
		t.Fatalf("expected empty_assistant_dropped degradation, got %+v", degradations)
	}
}

func TestNormalizeHistory_DoesNotMutateInput(t *testing.T) {
	in := []cometsdk.Message{assistantWithReasoning()}
	NormalizeHistory(in)
	if len(in[0].ReasoningContent) != 1 {
		t.Fatalf("input slice was mutated: reasoning len = %d", len(in[0].ReasoningContent))
	}
	if len(in[0].Content) != 2 {
		t.Fatalf("input slice was mutated: content len = %d", len(in[0].Content))
	}
}

func TestNormalizeHistory_EmptyReasoningPreserved(t *testing.T) {
	in := []cometsdk.Message{{
		Role:             cometsdk.RoleAssistant,
		Content:          []cometsdk.Block{cometsdk.TextBlock{Text: "hi"}},
		ReasoningContent: []cometsdk.Block{cometsdk.ReasoningBlock{Text: "   "}},
	}}
	out, degradations := NormalizeHistory(in)
	if len(degradations) != 0 {
		t.Fatalf("expected no degradation, got %v", degradations)
	}
	if len(out[0].ReasoningContent) != 1 {
		t.Fatalf("expected reasoning preserved")
	}
	if len(out[0].Content) != 1 {
		t.Fatalf("expected content preserved, got %d", len(out[0].Content))
	}
}

func TestNormalizeHistory_NonAssistantUntouched(t *testing.T) {
	in := []cometsdk.Message{
		{Role: cometsdk.RoleUser, Content: []cometsdk.Block{cometsdk.TextBlock{Text: "q"}}},
	}
	out, degradations := NormalizeHistory(in)
	if len(degradations) != 0 {
		t.Fatalf("expected no degradations for user message, got %v", degradations)
	}
	if len(out[0].Content) != 1 {
		t.Fatalf("user message altered")
	}
}
