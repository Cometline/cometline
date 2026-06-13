package agent

import (
	"strings"

	cometsdk "github.com/cometline/comet-sdk"
)

const DefaultSystemPrompt = `You are CometMind, a careful coding agent working inside a single workspace on the user's machine.
You may use the provided tools to read, modify, and explore files, and to run shell commands when useful.
Prefer small, verified steps. Summarize important changes clearly.`

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
