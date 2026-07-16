package agent

import (
	"fmt"
	"strings"

	cometsdk "github.com/cometline/comet-sdk"
)

const (
	// maxOutputTruncationContinuations caps how many extra model steps we take
	// when a step hits the output token limit without tool calls.
	maxOutputTruncationContinuations = 2
)

// DefaultSystemPrompt is persona + coding policy (shared mount docs via CodingPolicyPrompt).
func DefaultSystemPrompt() string {
	return DefaultPersonaPrompt + "\n\n" + CodingPolicyPrompt()
}

// FormatOutputBudgetPromptBlock reminds the model of the per-step output cap.
func FormatOutputBudgetPromptBlock(maxTokens int) string {
	if maxTokens <= 0 {
		return ""
	}
	return fmt.Sprintf(
		"Each assistant step is capped at roughly %d output tokens. After tool results arrive, respond concisely—summarize findings and next steps instead of repeating large tool output or full file contents.",
		maxTokens,
	)
}

// FormatOutputTruncationContinueBlock nudges the model to finish after a truncated step.
func FormatOutputTruncationContinueBlock() string {
	return "Your previous assistant message in this turn was cut off at the output token limit. Continue from where you stopped. Do not repeat text already written. Be concise and finish the thought, or give a brief closing summary if the work is complete."
}

// FormatWaitForSubagentsBlock reminds the model to join any in-flight
// subagents before ending the parent turn.
func FormatWaitForSubagentsBlock() string {
	return "You still have active subagents running for this turn. Before you finish, call wait_subagents to collect all remaining subagent results yourself. Do not ask the user whether you should wait unless they explicitly asked you to leave subagents running in the background."
}

// FormatCollectedSubagentResultsBlock injects runtime-collected child results
// back into the next parent model step for final synthesis.
func FormatCollectedSubagentResultsBlock(results string) string {
	results = strings.TrimSpace(results)
	if results == "" {
		return ""
	}
	return "The runtime waited for your active subagents before allowing this turn to finish. Here are their collected results. Synthesize them into your final answer and do not ask the user whether they want you to wait.\n\n" + results
}

// BuildRequest constructs the outbound LLM request from history and runtime settings.
func BuildRequest(model string, system string, messages []cometsdk.Message, tools []cometsdk.Tool, maxTokens int) *cometsdk.Request {
	req := &cometsdk.Request{
		Model:     model,
		System:    system,
		Messages:  messages,
		Tools:     tools,
		MaxTokens: maxTokens,
	}
	if strings.TrimSpace(req.System) == "" {
		req.System = DefaultSystemPrompt()
	}
	return req
}
