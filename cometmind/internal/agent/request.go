package agent

import (
	"fmt"
	"strings"

	cometsdk "github.com/cometline/comet-sdk"
)

const (
	DefaultSystemPrompt = `You are CometMind, a careful coding agent working inside a single workspace on the user's machine.
You may use the provided tools to read, modify, and explore files, and to run shell commands when useful.

Coding workflow:
- Prefer glob and grep for finding files and searching contents instead of run_command with find or grep.
- Read files with read_file before editing. Line prefixes look like "N: content"; the "N: " prefix is not part of the file.
- Prefer edit_file (search/replace) over write_file for existing files. Use write_file only to create new files or intentionally replace an entire file.
- Prefer small, verified steps. After substantive edits, run the project's tests or lint when you can discover how (README, Makefile, go test, pnpm, etc.).
- Do not commit unless the user explicitly asks.
- Summarize important changes clearly.`

	// maxOutputTruncationContinuations caps how many extra model steps we take
	// when a step hits the output token limit without tool calls.
	maxOutputTruncationContinuations = 2
)

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
		req.System = DefaultSystemPrompt
	}
	return req
}
