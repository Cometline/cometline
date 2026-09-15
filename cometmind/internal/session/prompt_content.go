package session

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// MaxToolResultPromptRunes caps each tool result block when building LLM prompts.
// Full output remains in SQLite for UI and diagnostics.
const MaxToolResultPromptRunes = 4000

// RecentWindowMaxRatio is the share of available input budget reserved for verbatim recent history.
const RecentWindowMaxRatio = 0.45

// MinRecentUserTurns is the minimum number of user turns kept verbatim even when over budget.
const MinRecentUserTurns = 1

// ClearedToolResultStub replaces pruned tool output in model prompts (UI keeps the full result).
const ClearedToolResultStub = "[Old tool result content cleared]"

// PruneProtectTokens is how much recent older-turn tool output stays verbatim.
const PruneProtectTokens = 40_000

// PruneMinimumTokens is the minimum amount of tool output that must be dropped
// before a prune pass writes compacted markers.
const PruneMinimumTokens = 20_000

var pruneProtectedTools = map[string]struct{}{
	"skill": {},
}

// ToolResultPromptContent is the model-visible tool output.
func ToolResultPromptContent(content string, compacted bool) string {
	if compacted {
		return ClearedToolResultStub
	}
	return TruncateToolResultForPrompt(content, MaxToolResultPromptRunes)
}

// EstimateTokens returns a conservative chars/4 token estimate (matches agent budget).
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

// TruncateToolResultForPrompt shortens tool output for model prompts.
func TruncateToolResultForPrompt(content string, maxRunes int) string {
	if maxRunes <= 0 {
		return content
	}
	content = strings.TrimSpace(content)
	if content == "" {
		return content
	}
	runes := []rune(content)
	if len(runes) <= maxRunes {
		return content
	}
	return string(runes[:maxRunes]) + fmt.Sprintf("\n\n[tool output truncated for context; %d chars total]", len(runes))
}
