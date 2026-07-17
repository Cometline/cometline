package codex

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"

	cometsdk "github.com/cometline/comet-sdk"
	"github.com/cometline/comet-sdk/internal/sse"
)

type codexStreamState struct {
	sawTool           bool
	sawReasoning      bool
	tools             map[string]*codexToolCallState
	suppressToolStart bool
}

type codexToolCallState struct {
	callID  string
	itemID  string
	name    string
	args    strings.Builder
	started bool
	done    bool
}

type codexStreamEvent struct {
	Type      string          `json:"type"`
	Delta     string          `json:"delta"`
	Text      string          `json:"text"`
	CallID    string          `json:"call_id"`
	ItemID    string          `json:"item_id"`
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
	Item      codexOutputItem `json:"item"`
	Usage     *struct {
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
	Type             string             `json:"type"`
	CallID           string             `json:"call_id"`
	ID               string             `json:"id"`
	Name             string             `json:"name"`
	Arguments        json.RawMessage    `json:"arguments"`
	Summary          []codexContentPart `json:"summary"`
	EncryptedContent string             `json:"encrypted_content"`
}

func parseLoop(ctx context.Context, providerID, modelID string, emitToolStart bool, body io.ReadCloser, ch chan<- cometsdk.Event, log *slog.Logger, idleTimeout time.Duration) {
	defer close(ch)
	defer body.Close()

	scanner := sse.NewIdleScanner(body, idleTimeout)
	defer scanner.Close()
	state := &codexStreamState{suppressToolStart: !emitToolStart}
	for scanner.Next() {
		select {
		case <-ctx.Done():
			ch <- cometsdk.ErrorEvent{Err: &cometsdk.StreamError{ProviderID: providerID, Cause: ctx.Err()}}
			return
		default:
		}

		ev := scanner.Event()
		log.DebugContext(ctx, "sse.event", "event", ev.Type, "data", redactEncryptedReasoning(ev.Data))
		events, err := toSDKEvents(ev.Type, ev.Data, state)
		if err != nil {
			ch <- cometsdk.ErrorEvent{Err: &cometsdk.StreamError{ProviderID: providerID, Cause: err}}
			return
		}
		for _, e := range events {
			if providerState, ok := e.(cometsdk.ProviderStateEvent); ok {
				providerState.State.ModelID = modelID
				e = providerState
			}
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

func redactEncryptedReasoning(data string) string {
	var value any
	if err := json.Unmarshal([]byte(data), &value); err != nil {
		return data
	}
	redactEncryptedValue(value)
	redacted, err := json.Marshal(value)
	if err != nil {
		return data
	}
	return string(redacted)
}

func redactEncryptedValue(value any) {
	switch v := value.(type) {
	case map[string]any:
		for key, child := range v {
			if key == "encrypted_content" {
				v[key] = "[redacted]"
				continue
			}
			redactEncryptedValue(child)
		}
	case []any:
		for _, child := range v {
			redactEncryptedValue(child)
		}
	}
}

func toSDKEvents(eventType, data string, state *codexStreamState) ([]cometsdk.Event, error) {
	if eventType == "[DONE]" || data == "[DONE]" {
		return []cometsdk.Event{cometsdk.DoneEvent{}}, nil
	}
	// Codex does not always repeat the event name inside the JSON payload.
	// If we ignore the SSE `event:` field here, the stream can look like a
	// clean no-op turn and we end up persisting an empty assistant step.
	ev := codexStreamEvent{Type: eventType}
	if data != "" {
		if err := json.Unmarshal([]byte(data), &ev); err != nil {
			return nil, fmt.Errorf("codex: parse event: %w", err)
		}
	}
	if ev.Type == "" {
		ev.Type = eventType
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
		return reasoningEvents(ev.Delta, state), nil
	case "response.output_item.added":
		if ev.Item.Type == "function_call" {
			tool := state.rememberTool(ev.Item.CallID, ev.Item.ID, ev.Item.Name)
			if !tool.started && !state.suppressToolStart {
				tool.started = true
				id := ev.Item.CallID
				if id == "" {
					id = ev.Item.ID
				}
				return []cometsdk.Event{cometsdk.ToolCallStartEvent{ID: id, Name: tool.name}}, nil
			}
		}
		return nil, nil
	case "response.function_call_arguments.delta":
		if ev.Delta != "" {
			state.rememberTool(ev.CallID, ev.ItemID, ev.Name).args.WriteString(ev.Delta)
		}
		return nil, nil
	case "response.output_item.done":
		if ev.Item.Type == "function_call" {
			return toolCallDoneEvents(ev.Item.CallID, ev.Item.ID, ev.Item.Name, ev.Item.Arguments, state)
		}
		if ev.Item.Type == "reasoning" {
			events := reasoningEvents(reasoningSummaryText(ev.Item.Summary), state)
			if ev.Item.EncryptedContent != "" {
				events = append(events, cometsdk.ProviderStateEvent{State: cometsdk.ProviderState{
					ProviderID: providerID,
					Data:       ev.Item.EncryptedContent,
				}})
			}
			return events, nil
		}
		return nil, nil
	case "response.function_call_arguments.done":
		// Some Responses-compatible streams emit the tool call completion as a
		// top-level event instead of nesting it under `item`.
		return toolCallDoneEvents(ev.CallID, ev.ItemID, ev.Name, ev.Arguments, state)
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

func (s *codexStreamState) rememberTool(callID, itemID, name string) *codexToolCallState {
	if s.tools == nil {
		s.tools = make(map[string]*codexToolCallState)
	}
	var tool *codexToolCallState
	if callID != "" {
		tool = s.tools[callID]
	}
	if tool == nil && itemID != "" {
		tool = s.tools[itemID]
	}
	if tool == nil {
		tool = &codexToolCallState{}
	}
	if callID != "" {
		tool.callID = callID
		s.tools[callID] = tool
	}
	if itemID != "" {
		tool.itemID = itemID
		s.tools[itemID] = tool
	}
	if name != "" {
		tool.name = name
	}
	return tool
}

func toolCallDoneEvents(callID, itemID, name string, args json.RawMessage, state *codexStreamState) ([]cometsdk.Event, error) {
	tool := state.rememberTool(callID, itemID, name)
	if tool.done {
		return nil, nil
	}
	if callID == "" {
		callID = tool.callID
	}
	id := callID
	if id == "" {
		id = itemID
	}
	if id == "" {
		id = tool.itemID
	}
	if name == "" {
		name = tool.name
	}

	normalizedArgs, err := normalizeToolArguments(args, tool.args.String())
	if err != nil {
		return nil, err
	}

	tool.done = true
	state.sawTool = true
	events := make([]cometsdk.Event, 0, 3)
	if !tool.started {
		tool.started = true
		events = append(events, cometsdk.ToolCallStartEvent{ID: id, Name: name})
	}
	events = append(events,
		cometsdk.ToolCallDeltaEvent{ID: id, Delta: string(normalizedArgs)},
		cometsdk.ToolCallDoneEvent{ID: id, Name: name, Input: normalizedArgs},
	)
	return events, nil
}

func reasoningEvents(text string, state *codexStreamState) []cometsdk.Event {
	if text == "" {
		return nil
	}
	events := make([]cometsdk.Event, 0, 2)
	if !state.sawReasoning {
		state.sawReasoning = true
		events = append(events, cometsdk.ReasoningStartEvent{})
	}
	return append(events, cometsdk.ReasoningContentEvent{Text: text})
}

func reasoningSummaryText(summary []codexContentPart) string {
	var text strings.Builder
	for _, part := range summary {
		if part.Type == "summary_text" {
			text.WriteString(part.Text)
		}
	}
	return text.String()
}

func normalizeToolArguments(args json.RawMessage, fallback string) (json.RawMessage, error) {
	raw := strings.TrimSpace(string(args))
	if raw == "" {
		raw = strings.TrimSpace(fallback)
	}
	if raw == "" {
		return json.RawMessage(`{}`), nil
	}

	// Codex can report function-call arguments as a JSON string containing the
	// actual object. Passing that through makes Go tools receive a string instead
	// of an object and fail to unmarshal their structured inputs.
	var encoded string
	if err := json.Unmarshal([]byte(raw), &encoded); err == nil {
		raw = strings.TrimSpace(encoded)
		if raw == "" {
			return json.RawMessage(`{}`), nil
		}
	}
	if !json.Valid([]byte(raw)) {
		return nil, fmt.Errorf("codex: invalid tool arguments JSON")
	}
	return json.RawMessage(raw), nil
}
