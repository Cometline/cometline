package session

import (
	"encoding/json"

	cometsdk "github.com/cometline/comet-sdk"
	"github.com/cometline/cometmind/internal/db"
)

// RecentWindowStartForBudget chooses the recent verbatim window start index.
// It preserves up to maxUserTurns, but shrinks the window when estimated tokens
// would exceed RecentWindowMaxRatio of the available input budget.
func RecentWindowStartForBudget(
	rows []db.Message,
	callsByMessage map[string][]db.ToolCall,
	maxUserTurns int,
	contextWindow int,
	outputBudget int,
) int {
	if len(rows) == 0 {
		return 0
	}
	available := contextWindow - outputBudget
	if available <= 0 {
		available = contextWindow / 2
	}
	maxRecentTokens := int(float64(available) * RecentWindowMaxRatio)

	recentStart := RecentWindowStartIndex(rows, maxUserTurns)
	for {
		slice := rows[recentStart:]
		tokens := EstimateRowsTokens(slice, callsByMessage)
		if tokens <= maxRecentTokens {
			return recentStart
		}
		userTurns := countUserTurns(slice)
		if userTurns > MinRecentUserTurns {
			next := nextUserMessageIndex(rows, recentStart+1)
			if next < len(rows) && next > recentStart {
				recentStart = next
				continue
			}
		}
		return splitTurnStart(rows, recentStart, callsByMessage, maxRecentTokens)
	}
}

// splitTurnStart walks into a single oversized turn and returns the earliest
// assistant/user index whose tail fits budget. It never starts on a tool_result
// so providers do not see an orphan tool output.
func splitTurnStart(rows []db.Message, turnStart int, callsByMessage map[string][]db.ToolCall, budget int) int {
	if turnStart < 0 || turnStart >= len(rows) {
		return 0
	}
	for start := turnStart + 1; start < len(rows); start++ {
		if rows[start].Role == "tool_result" {
			continue
		}
		if EstimateRowsTokens(rows[start:], callsByMessage) <= budget {
			return start
		}
	}
	for i := len(rows) - 1; i > turnStart; i-- {
		if rows[i].Role != "tool_result" {
			return i
		}
	}
	return turnStart
}

func countUserTurns(rows []db.Message) int {
	n := 0
	for _, row := range rows {
		if row.Role == "user" {
			n++
		}
	}
	return n
}

func nextUserMessageIndex(rows []db.Message, from int) int {
	for i := from; i < len(rows); i++ {
		if rows[i].Role == "user" {
			return i
		}
	}
	return len(rows)
}

// EstimateRowsTokens estimates prompt tokens for persisted rows using the same
// truncation rules as BuildSDKMessages.
func EstimateRowsTokens(rows []db.Message, callsByMessage map[string][]db.ToolCall) int {
	total := 0
	for _, row := range rows {
		switch row.Role {
		case "user":
			blocks, err := DecodeMessageContent(row.Content)
			if err != nil {
				total += EstimateTokens(row.Content)
				break
			}
			total += EstimateContentBlocksTokens(blocks)
		case "assistant":
			total += EstimateTokens(row.Content)
			for _, tc := range callsByMessage[row.ID] {
				total += EstimateTokens(tc.ToolName)
				total += EstimateTokens(tc.Arguments)
			}
			if blocks, err := unmarshalReasoningContent(row.ReasoningContent); err == nil {
				for _, block := range blocks {
					if rb, ok := block.(cometsdk.ReasoningBlock); ok {
						total += EstimateTokens(rb.Text)
					}
				}
			}
		case "tool_result":
			var p toolResultPayload
			if err := json.Unmarshal([]byte(row.Content), &p); err == nil {
				total += EstimateTokens(ToolResultPromptContent(p.Content, toolCallIsCompacted(callsByMessage, p.ToolCallID)))
			}
		}
	}
	return total
}
