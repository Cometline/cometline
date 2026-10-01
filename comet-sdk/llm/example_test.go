package llm_test

import (
	"context"
	"encoding/json"
	"fmt"

	cometsdk "github.com/Cometline/cometline/comet-sdk"
	"github.com/Cometline/cometline/comet-sdk/llm"
)

// scriptedProvider replays a fixed event sequence, standing in for a real
// provider such as anthropic.New or openai.New.
type scriptedProvider struct {
	events []cometsdk.Event
}

func (scriptedProvider) ID() string { return "scripted" }

func (p scriptedProvider) Stream(context.Context, *cometsdk.Request) (<-chan cometsdk.Event, error) {
	ch := make(chan cometsdk.Event, len(p.events))
	for _, ev := range p.events {
		ch <- ev
	}
	close(ch)
	return ch, nil
}

func textReply(chunks ...string) scriptedProvider {
	var events []cometsdk.Event
	for _, c := range chunks {
		events = append(events, cometsdk.TextDeltaEvent{Text: c})
	}
	events = append(events,
		cometsdk.StepFinishEvent{
			FinishReason: cometsdk.FinishStop,
			Usage:        cometsdk.TokenUsage{InputTokens: 12, OutputTokens: 4},
		},
		cometsdk.DoneEvent{},
	)
	return scriptedProvider{events: events}
}

func userRequest(prompt string) *cometsdk.Request {
	return &cometsdk.Request{
		Model:     "example-model",
		MaxTokens: 256,
		Messages: []cometsdk.Message{{
			Role:    cometsdk.RoleUser,
			Content: []cometsdk.Block{cometsdk.TextBlock{Text: prompt}},
		}},
	}
}

func ExampleGenerateText() {
	p := textReply("Paris", ".")

	result, err := llm.GenerateText(context.Background(), p, userRequest("What is the capital of France?"))
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Println(result.Text, result.FinishReason, result.Usage.OutputTokens)
	// Output: Paris. stop 4
}

func ExampleQuickText() {
	text, err := llm.QuickText(context.Background(), textReply("Hi there!"), "example-model", "Say hi.")
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Println(text)
	// Output: Hi there!
}

// GenerateMessage returns tool calls instead of executing them; the caller
// runs the tool and sends the result back in the next request.
func ExampleGenerateMessage() {
	p := scriptedProvider{events: []cometsdk.Event{
		cometsdk.ToolCallStartEvent{ID: "call_1", Name: "read_file"},
		cometsdk.ToolCallDoneEvent{ID: "call_1", Name: "read_file", Input: json.RawMessage(`{"path":"main.go"}`)},
		cometsdk.StepFinishEvent{FinishReason: cometsdk.FinishToolUse},
		cometsdk.DoneEvent{},
	}}

	req := userRequest("What does main.go do?")
	req.Tools = []cometsdk.Tool{{
		Name:        "read_file",
		Description: "Read the contents of a file",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"}},"required":["path"]}`),
	}}

	result, err := llm.GenerateMessage(context.Background(), p, req)
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Println("finish:", result.FinishReason)
	for _, tc := range result.ToolCalls {
		fmt.Printf("tool: %s %s\n", tc.Name, tc.Input)
	}
	// Output:
	// finish: tool_use
	// tool: read_file {"path":"main.go"}
}

// Drain Events before calling Result; Result blocks until the stream ends.
func ExampleStreamMessage() {
	p := textReply("Hello", ", ", "world")

	stream := llm.StreamMessage(context.Background(), p, userRequest("Greet the world."))
	for ev := range stream.Events() {
		if e, ok := ev.(cometsdk.TextDeltaEvent); ok {
			fmt.Printf("[%s]", e.Text)
		}
	}
	fmt.Println()

	result, err := stream.Result()
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Println("finish:", result.FinishReason)
	// Output:
	// [Hello][, ][world]
	// finish: stop
}

func ExampleGenerateJSON() {
	p := textReply("```json\n", `{"city":"Paris","country":"France"}`, "\n```")

	var out struct {
		City    string `json:"city"`
		Country string `json:"country"`
	}
	if _, err := llm.GenerateJSON(context.Background(), p, userRequest("Where is the Eiffel Tower?"), &out); err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Println(out.City, out.Country)
	// Output: Paris France
}
