package contract_test

import (
	"encoding/json"
	"testing"

	cometsdk "github.com/cometline/comet-sdk"
	"github.com/cometline/cometmind/internal/config"
	"github.com/cometline/cometmind/internal/contract"
	"github.com/cometline/cometmind/internal/event"
)

func loadOpenAPI(t *testing.T) {
	t.Helper()
	if _, err := contract.OpenAPI(); err != nil {
		t.Fatalf("OpenAPI() error = %v", err)
	}
}

func validateStreamEventJSON(t *testing.T, raw []byte) {
	t.Helper()
	if err := contract.ValidateStreamEventJSON(raw); err != nil {
		t.Fatalf("ValidateStreamEventJSON() error = %v\npayload: %s", err, raw)
	}
}

func TestStreamEventMarshalJSONMatchesOpenAPI(t *testing.T) {
	t.Parallel()

	loadOpenAPI(t)

	cases := []struct {
		name string
		ev   event.Event
	}{
		{name: "text_delta", ev: event.TextDelta("hello")},
		{name: "reasoning_start", ev: event.ReasoningStart()},
		{name: "reasoning_delta", ev: event.ReasoningDelta("thinking")},
		{name: "tool_call", ev: event.ToolCall("tc-1", "read_file", []byte(`{"path":"main.go"}`))},
		{name: "tool_result", ev: event.ToolResult("tc-1", "read_file", "package main", "")},
		{name: "tool_result_error", ev: event.ToolResult("tc-1", "read_file", "", "permission denied")},
		{name: "step_finish", ev: event.StepFinish(cometsdk.TokenUsage{InputTokens: 1, OutputTokens: 2, CacheRead: 3, CacheWrite: 4})},
		{name: "subagent_started", ev: event.SubagentStarted("child-1", "refactor", "opencode")},
		{name: "subagent_progress", ev: event.SubagentProgress("child-1", "stream", "working")},
		{name: "subagent_awaiting_input", ev: event.SubagentAwaitingInput("child-1", "permission", "allow?", []event.PermissionOptionWire{{ID: "yes", Kind: "allow", Name: "Allow"}})},
		{name: "subagent_finished", ev: event.SubagentFinished("child-1", "completed", "done")},
		{name: "memory_injected", ev: event.MemoryInjected([]event.MemoryWire{{ID: "m1", Content: "fact", Kind: "preference", Similarity: 0.9, EffectiveWeight: 1.2}})},
		{name: "memory_updated", ev: event.MemoryUpdated([]event.MemoryChangeWire{{Action: "create", Kind: "preference", Content: "likes tea"}})},
		{name: "error", ev: event.Errorf("boom", "llm")},
		{name: "done", ev: event.Done()},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			raw, err := json.Marshal(tc.ev)
			if err != nil {
				t.Fatalf("json.Marshal() error = %v", err)
			}
			validateStreamEventJSON(t, raw)
		})
	}
}

func TestMemoryConfigMatchesOpenAPIMemorySettings(t *testing.T) {
	t.Parallel()

	doc, err := contract.OpenAPI()
	if err != nil {
		t.Fatalf("OpenAPI() error = %v", err)
	}
	schemaRef := doc.Components.Schemas["MemorySettings"]
	if schemaRef == nil || schemaRef.Value == nil {
		t.Fatal("MemorySettings schema missing from openapi.yaml")
	}

	cfg := config.MemoryConfig{
		Enabled:             true,
		AutoExtract:         true,
		AutoRetrieve:        true,
		MaxRetrieved:        5,
		SimilarityThreshold: 0.5,
		ExtractionModel:     "claude-sonnet-4-5",
		Lifecycle: config.MemoryLifecycleConfig{
			DecayHalfLifeDays:     30,
			ForgetThreshold:       0.1,
			UsageBoostFactor:      0.15,
			MaxUsageBoost:         2,
			MaxMemories:           500,
			CompactionTargetRatio: 0.8,
			CompactionOnExtract:   true,
		},
		Embedding: config.MemoryEmbeddingConfig{
			ProviderID: "openai",
			Provider:   "openai",
			Model:      "text-embedding-3-small",
			BaseURL:    "https://api.openai.com/v1",
			APIKey:     "sk-test",
		},
	}

	raw, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	var payload any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if err := schemaRef.Value.VisitJSON(payload); err != nil {
		t.Fatalf("schema.VisitJSON() error = %v\npayload: %s", err, raw)
	}
}
