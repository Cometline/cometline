package event

import (
	"encoding/json"
	"reflect"
	"testing"

	cometsdk "github.com/Cometline/cometline/comet-sdk"
)

func TestEventWireGolden(t *testing.T) {
	cases := []struct {
		name string
		ev   Event
		want string
	}{
		{"text_delta", TextDelta("hi"), `{"type":"text_delta","delta":"hi"}`},
		{"reasoning_start", ReasoningStart(), `{"type":"reasoning_start"}`},
		{"reasoning_delta", ReasoningDelta("think"), `{"type":"reasoning_delta","text":"think"}`},
		{"tool_call", ToolCall("c1", "read", []byte(`{"path":"a"}`)), `{"type":"tool_call","id":"c1","tool":"read","input":{"path":"a"}}`},
		{"tool_call_empty_input", ToolCall("c1", "read", nil), `{"type":"tool_call","id":"c1","tool":"read","input":{}}`},
		{"tool_result", ToolResult("c1", "read", "ok", ""), `{"type":"tool_result","id":"c1","tool":"read","output":"ok"}`},
		{"tool_result_error", ToolResult("c1", "read", "", "boom"), `{"type":"tool_result","id":"c1","tool":"read","output":"","error":"boom"}`},
		{
			"step_finish",
			StepFinish(cometsdk.TokenUsage{InputTokens: 1, OutputTokens: 2, CacheRead: 3, CacheWrite: 4}),
			`{"type":"step_finish","usage":{"input_tokens":1,"output_tokens":2,"cache_read":3,"cache_write":4}}`,
		},
		{"subagent_started", SubagentStarted("child", "purpose", "agent"), `{"type":"subagent_started","child_session_id":"child","purpose":"purpose","agent_name":"agent"}`},
		{"subagent_progress", SubagentProgress("child", "tool", "reading"), `{"type":"subagent_progress","child_session_id":"child","progress_kind":"tool","progress_text":"reading"}`},
		{"subagent_finished", SubagentFinished("child", "completed", "sum"), `{"type":"subagent_finished","child_session_id":"child","delegation_status":"completed","summary":"sum"}`},
		{
			"memory_injected",
			MemoryInjected([]MemoryWire{{ID: "m1", Content: "c", Kind: "fact", Bucket: MemoryBucketSemantic, Similarity: 0.5, EffectiveWeight: 0.25}}),
			`{"type":"memory_injected","memories":[{"id":"m1","content":"c","kind":"fact","bucket":"semantic","similarity":0.5,"effective_weight":0.25}]}`,
		},
		{"memory_injected_nil", MemoryInjected(nil), `{"type":"memory_injected","memories":null}`},
		{
			"memory_updated",
			MemoryUpdated([]MemoryChangeWire{{Action: "add", Kind: "fact", Content: "c", ID: "m1"}, {Action: "delete", Kind: "fact", Content: "d"}}),
			`{"type":"memory_updated","changes":[{"action":"add","kind":"fact","content":"c","id":"m1"},{"action":"delete","kind":"fact","content":"d"}]}`,
		},
		{"memory_compaction_completed", MemoryCompactionCompleted(10, 4, "manual"), `{"type":"memory_compaction_completed","before":10,"after":4,"trigger":"manual"}`},
		{"context_budget", ContextBudget(100, 200, 300, false), `{"type":"context_budget","estimated":100,"available":200,"context_window":300}`},
		{"context_budget_compacted", ContextBudget(100, 200, 300, true), `{"type":"context_budget","estimated":100,"available":200,"context_window":300,"compacted":true}`},
		{"inbox_message_created", InboxMessageCreated("i1", 3), `{"type":"inbox_message_created","id":"i1","open_count":3}`},
		{"inbox_message_archived", InboxMessageArchived("i1", 2, "replied"), `{"type":"inbox_message_archived","id":"i1","open_count":2,"archive_reason":"replied"}`},
		{"run_started", RunStarted("s1"), `{"type":"run_started","session_id":"s1"}`},
		{"run_finished", RunFinished("s1"), `{"type":"run_finished","session_id":"s1"}`},
		{"session_cleared", SessionCleared("s1"), `{"type":"session_cleared","session_id":"s1"}`},
		{"turn_status", TurnStatus(PhaseContactingModel, "calling"), `{"type":"turn_status","phase":"contacting_model","message":"calling"}`},
		{"turn_status_blank_message", TurnStatus(PhaseRunningTools, "  "), `{"type":"turn_status","phase":"running_tools"}`},
		{"turn_recover", TurnRecover(5, 7), `{"type":"turn_recover","text_chars":5,"reasoning_chars":7}`},
		{"assistant_image", AssistantImage("img", "image/png", "alt", "data:x"), `{"type":"assistant_image","id":"img","media_type":"image/png","alt":"alt","data_url":"data:x"}`},
		{"assistant_video", AssistantVideo("vid", "video/mp4", ""), `{"type":"assistant_video","id":"vid","media_type":"video/mp4"}`},
		{"error", Errorf("bad", "E1"), `{"type":"error","message":"bad","code":"E1"}`},
		{"error_no_code", Errorf("bad", ""), `{"type":"error","message":"bad"}`},
		{"done", Done(), `{"type":"done"}`},
		{"unknown_kind", Event{Kind: "future_kind", Delta: "ignored"}, `{"type":"future_kind"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := json.Marshal(tc.ev)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != tc.want {
				t.Fatalf("Marshal() = %s\nwant      %s", got, tc.want)
			}
			var decoded Event
			if err := json.Unmarshal(got, &decoded); err != nil {
				t.Fatal(err)
			}
			again, err := json.Marshal(decoded)
			if err != nil {
				t.Fatal(err)
			}
			if string(again) != tc.want {
				t.Fatalf("round-trip Marshal() = %s\nwant                %s", again, tc.want)
			}
		})
	}
}

func TestEventUnmarshalMapsSharedWireFields(t *testing.T) {
	var got Event
	if err := json.Unmarshal([]byte(`{"type":"error","id":"x","message":"m","code":"c","input":{"a":1}}`), &got); err != nil {
		t.Fatal(err)
	}
	want := Event{
		Kind:           KindError,
		ID:             "x",
		InboxMessageID: "x",
		ImageID:        "x",
		Input:          []byte(`{"a":1}`),
		StatusMessage:  "m",
		Message:        "m",
		Code:           "c",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Unmarshal() = %+v\nwant        %+v", got, want)
	}
}

func TestEventUnmarshalRequiresType(t *testing.T) {
	var got Event
	if err := json.Unmarshal([]byte(`{"type":"  "}`), &got); err == nil {
		t.Fatal("Unmarshal() error = nil, want missing type error")
	}
}
