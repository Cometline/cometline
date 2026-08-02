package agent

import (
	"fmt"
	"strings"

	cometsdk "github.com/cometline/comet-sdk"
	"github.com/cometline/cometmind/internal/session"
)

const (
	// maxOutputTruncationContinuations caps how many extra model steps we take
	// when a step hits the output token limit without tool calls.
	maxOutputTruncationContinuations = 2
	// maxIncompleteToolTruncationContinuations caps retries after tool-call
	// arguments were cut off mid-stream (started but never completed).
	maxIncompleteToolTruncationContinuations = 2
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
		"Each assistant step is capped at roughly %d output tokens. After tool results arrive, respond concisely; summarize findings and next steps instead of repeating large tool output or full file contents. Emit only one large tool call (or a few small tool calls) per step so the full arguments fit under this cap; never start many large parallel tool calls that may be truncated unfinished.",
		maxTokens,
	)
}

// FormatAgentModePrompt describes how the agent should use its mode-specific
// tool surface. Tool registration remains the enforcement boundary.
func FormatAgentModePrompt(mode session.AgentMode) string {
	switch mode {
	case session.AgentModeAuto:
		return "Auto mode is active. Complete the user's request end to end using the tools available to you. When implementation is requested, inspect the relevant code, make the necessary changes, verify them, and report the result. Do not stop at a plan unless the user explicitly asks for planning only. For large file writes, emit one complete write_file call per step so its arguments fit within the output limit."
	case session.AgentModePlan:
		return "Plan mode is active. Research and design only; do not attempt to edit or write files, run commands, mutate settings, jobs, or memory, or delegate coding work. Use the available read, network, skill, and research-agent tools to understand the request. Return a concrete implementation plan with affected files, key decisions, risks, and verification steps. Do not claim that changes were made. These mode restrictions supersede generic implementation instructions."
	default:
		return ""
	}
}

// FormatOutputTruncationContinueBlock nudges the model to finish after a truncated step.
func FormatOutputTruncationContinueBlock() string {
	return "Your previous assistant message in this turn was cut off at the output token limit. Stop long prose. Continue unfinished work with exactly one complete, smaller tool call if needed, or finish with a concise response. Do not repeat text already written. Do not start multiple large parallel tool calls."
}

// FormatIncompleteToolTruncationContinueBlock nudges after tool-call args were truncated.
func FormatIncompleteToolTruncationContinueBlock() string {
	return "Your previous tool call(s) were cut off at the output token limit before they finished, so they were not executed. Retry with exactly one smaller, complete tool call whose full arguments fit under the output cap. Do not repeat assistant text already written. Do not re-issue the truncated parallel batch."
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

// FinalAnswerNudgeMessages requests a best-effort answer after the agent has
// exhausted its work-step budget. It is intentionally not persisted.
func FinalAnswerNudgeMessages() []cometsdk.Message {
	return []cometsdk.Message{{
		Role:    cometsdk.RoleUser,
		Content: []cometsdk.Block{cometsdk.TextBlock{Text: "You have exhausted the work-step budget for this turn. Do not call tools. Give the user your best final answer based on the work and tool results already available."}},
	}}
}

// ContinueUserNudgeMessages builds in-memory user turns for agent-loop
// continuations. Claude 4.6+ rejects requests whose messages end with an
// assistant role (prefill). These nudges keep continue steps ending on user
// without persisting into the transcript.
func ContinueUserNudgeMessages(
	truncationContinue, incompleteToolTruncationContinue, jobProgressNudge, jobCompletionGate bool,
	jobID string,
	subagentWaitNudge bool,
	pendingSubagentResults string,
) []cometsdk.Message {
	var parts []string
	// Incomplete-tool truncation supersedes the generic text truncation nudge.
	if incompleteToolTruncationContinue {
		parts = append(parts, FormatIncompleteToolTruncationContinueBlock())
	} else if truncationContinue {
		parts = append(parts, FormatOutputTruncationContinueBlock())
	}
	if jobProgressNudge {
		if block := FormatJobProgressNudgeBlock(jobID); block != "" {
			parts = append(parts, block)
		}
	}
	if jobCompletionGate {
		if block := FormatJobCompletionGateBlock(jobID); block != "" {
			parts = append(parts, block)
		}
	}
	if subagentWaitNudge {
		parts = append(parts, FormatWaitForSubagentsBlock())
	}
	if block := FormatCollectedSubagentResultsBlock(pendingSubagentResults); block != "" {
		parts = append(parts, block)
	}
	if len(parts) == 0 {
		return nil
	}
	return []cometsdk.Message{{
		Role:    cometsdk.RoleUser,
		Content: []cometsdk.Block{cometsdk.TextBlock{Text: strings.Join(parts, "\n\n")}},
	}}
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
