package codex

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"

	cometsdk "github.com/cometline/comet-sdk"
	"github.com/cometline/comet-sdk/internal/sse"
)

type codexStreamState struct {
	sawTool bool
}

type codexStreamEvent struct {
	Type  string          `json:"type"`
	Delta string          `json:"delta"`
	Text  string          `json:"text"`
	Item  codexOutputItem `json:"item"`
	Usage *struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
	Response *struct {
		Usage *struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	} `json:"response"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

type codexOutputItem struct {
	Type      string          `json:"type"`
	CallID    string          `json:"call_id"`
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

func parseLoop(ctx context.Context, providerID string, body io.ReadCloser, ch chan<- cometsdk.Event, log *slog.Logger) {
	defer close(ch)
	defer body.Close()

	scanner := sse.NewScanner(body)
	state := &codexStreamState{}
	for scanner.Next() {
		select {
		case <-ctx.Done():
			ch <- cometsdk.ErrorEvent{Err: &cometsdk.StreamError{ProviderID: providerID, Cause: ctx.Err()}}
			return
		default:
		}

		ev := scanner.Event()
		log.DebugContext(ctx, "sse.event", "event", ev.Type, "data", ev.Data)
		events, err := toSDKEvents(ev.Data, state)
		if err != nil {
			ch <- cometsdk.ErrorEvent{Err: &cometsdk.StreamError{ProviderID: providerID, Cause: err}}
			return
		}
		for _, e := range events {
			ch <- e
			if _, ok := e.(cometsdk.DoneEvent); ok {
				return
			}
		}
	}
	if err := scanner.Err(); err != nil {
		ch <- cometsdk.ErrorEvent{Err: &cometsdk.StreamError{ProviderID: providerID, Cause: err}}
		return
	}
	ch <- cometsdk.StepFinishEvent{FinishReason: cometsdk.FinishStop}
	ch <- cometsdk.DoneEvent{}
}

func toSDKEvents(data string, state *codexStreamState) ([]cometsdk.Event, error) {
	if data == "" || data == "[DONE]" {
		return []cometsdk.Event{cometsdk.DoneEvent{}}, nil
	}
	var ev codexStreamEvent
	if err := json.Unmarshal([]byte(data), &ev); err != nil {
		return nil, fmt.Errorf("codex: parse event: %w", err)
	}
	if ev.Error != nil && ev.Error.Message != "" {
		return nil, fmt.Errorf("codex: %s", ev.Error.Message)
	}

	switch ev.Type {
	case "response.output_text.delta", "response.output_text.annotation.added":
		if ev.Delta == "" {
			ev.Delta = ev.Text
		}
		if ev.Delta == "" {
			return nil, nil
		}
		return []cometsdk.Event{cometsdk.TextDeltaEvent{Text: ev.Delta}}, nil
	case "response.reasoning_text.delta", "response.reasoning_summary_text.delta":
		if ev.Delta == "" {
			ev.Delta = ev.Text
		}
		if ev.Delta == "" {
			return nil, nil
		}
		return []cometsdk.Event{cometsdk.ReasoningContentEvent{Text: ev.Delta}}, nil
	case "response.output_item.done":
		if ev.Item.Type != "function_call" {
			return nil, nil
		}
		state.sawTool = true
		id := ev.Item.CallID
		if id == "" {
			id = ev.Item.ID
		}
		args := ev.Item.Arguments
		if len(args) == 0 {
			args = json.RawMessage(`{}`)
		}
		return []cometsdk.Event{
			cometsdk.ToolCallStartEvent{ID: id, Name: ev.Item.Name},
			cometsdk.ToolCallDeltaEvent{ID: id, Delta: string(args)},
			cometsdk.ToolCallDoneEvent{ID: id, Name: ev.Item.Name, Input: args},
		}, nil
	case "response.completed":
		usage := cometsdk.TokenUsage{}
		if ev.Usage != nil {
			usage.InputTokens = ev.Usage.InputTokens
			usage.OutputTokens = ev.Usage.OutputTokens
		} else if ev.Response != nil && ev.Response.Usage != nil {
			usage.InputTokens = ev.Response.Usage.InputTokens
			usage.OutputTokens = ev.Response.Usage.OutputTokens
		}
		finish := cometsdk.FinishStop
		if state.sawTool {
			finish = cometsdk.FinishToolUse
		}
		return []cometsdk.Event{cometsdk.StepFinishEvent{FinishReason: finish, Usage: usage}, cometsdk.DoneEvent{}}, nil
	case "response.failed", "error":
		if ev.Error != nil && ev.Error.Message != "" {
			return nil, fmt.Errorf("codex: %s", ev.Error.Message)
		}
		return nil, fmt.Errorf("codex: stream failed")
	default:
		return nil, nil
	}
}
