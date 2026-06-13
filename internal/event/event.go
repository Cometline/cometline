package event

import cometsdk "github.com/cometline/comet-sdk"

// Kind identifies a CometMind-native runtime event (for CLI, future SSE, TUI).
type Kind string

const (
	KindTextDelta      Kind = "text_delta"
	KindReasoningStart Kind = "reasoning_start"
	KindReasoningDelta Kind = "reasoning_delta"
	KindToolCall       Kind = "tool_call"
	KindToolResult     Kind = "tool_result"
	KindStepFinish     Kind = "step_finish"
	KindError          Kind = "error"
	KindDone           Kind = "done"
)

// Event is a small discriminated union for terminal and adapter rendering.
type Event struct {
	Kind Kind

	TextDelta      *TextDelta
	ReasoningStart *ReasoningStart
	ReasoningDelta *ReasoningDelta
	ToolCall       *ToolCall
	ToolOut        *ToolResult
	Step           *StepFinish
	Err            *Error
}

type TextDelta struct {
	Delta string
}

type ReasoningStart struct{}

type ReasoningDelta struct {
	Text string
}

type ToolCall struct {
	ID    string
	Tool  string
	Input []byte // JSON object bytes
}

type ToolResult struct {
	ID     string
	Tool   string
	Output string
	Err    string // empty if success
}

type StepFinish struct {
	Usage cometsdk.TokenUsage
}

type Error struct {
	Message string
	Code    string
}
