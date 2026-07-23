package agent

import (
	"encoding/json"
	"strings"
	"unicode/utf8"

	cometsdk "github.com/cometline/comet-sdk"
)

const (
	// Compact only once estimated input exceeds the full configured budget (minus output reserve).
	compactionThresholdRatio = 1.0
	recentTurnPreserveCount  = 10
)

// EstimateTokens returns a conservative character-based token estimate (chars/4).
func EstimateTokens(text string) int {
	n := utf8.RuneCountInString(text)
	if n <= 0 {
		return 0
	}
	tokens := n / 4
	if tokens < 1 && n > 0 {
		return 1
	}
	return tokens
}

// EstimateMessageTokens estimates tokens for one SDK message.
func EstimateMessageTokens(msg cometsdk.Message) int {
	total := 0
	for _, block := range msg.Content {
		switch b := block.(type) {
		case cometsdk.TextBlock:
			total += EstimateTokens(b.Text)
		case cometsdk.ToolCallBlock:
			total += EstimateTokens(b.Name)
			total += EstimateTokens(string(b.Input))
		case cometsdk.ToolResultBlock:
			total += EstimateTokens(b.Content)
		default:
			if raw, err := json.Marshal(block); err == nil {
				total += EstimateTokens(string(raw))
			}
		}
	}
	for _, block := range msg.ReasoningContent {
		switch b := block.(type) {
		case cometsdk.ReasoningBlock:
			total += EstimateTokens(b.Text)
		}
	}
	return total
}

// EstimateMessagesTokens sums token estimates for a message slice.
func EstimateMessagesTokens(msgs []cometsdk.Message) int {
	total := 0
	for _, msg := range msgs {
		total += EstimateMessageTokens(msg)
	}
	return total
}

// EstimateToolsTokens estimates tool schema overhead.
func EstimateToolsTokens(tools []cometsdk.Tool) int {
	total := 0
	for _, tool := range tools {
		total += EstimateTokens(tool.Name)
		total += EstimateTokens(tool.Description)
		if len(tool.Parameters) > 0 {
			total += EstimateTokens(string(tool.Parameters))
		}
	}
	return total
}

// PromptBudgetInput carries prompt components for budget estimation.
type PromptBudgetInput struct {
	System       string
	Summary      string
	Messages     []cometsdk.Message
	Tools        []cometsdk.Tool
	OutputBudget int
}

// EstimatePromptTokens estimates total input-side tokens for a request.
func EstimatePromptTokens(in PromptBudgetInput) int {
	total := EstimateTokens(in.System)
	if s := strings.TrimSpace(in.Summary); s != "" {
		total += EstimateTokens(FormatSummaryPromptBlock(s))
	}
	total += EstimateMessagesTokens(in.Messages)
	total += EstimateToolsTokens(in.Tools)
	return total
}

// ShouldCompact reports whether estimated input exceeds the available prompt budget.
// available is typically context − max(effectiveMaxTokens, 20k).
func ShouldCompact(estimatedInput, available int) bool {
	if available <= 0 {
		return true
	}
	return estimatedInput > int(float64(available)*compactionThresholdRatio)
}

// FormatSummaryPromptBlock wraps a session summary for the system prompt.
func FormatSummaryPromptBlock(summary string) string {
	summary = strings.TrimSpace(summary)
	if summary == "" {
		return ""
	}
	return "Earlier conversation summary (for context only):\n" + summary
}
