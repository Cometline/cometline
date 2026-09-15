package session

import (
	"github.com/cometline/cometmind/internal/db"
)

func toolCallIsCompacted(callsByMessage map[string][]db.ToolCall, toolCallID string) bool {
	for _, calls := range callsByMessage {
		for _, tc := range calls {
			if tc.ID == toolCallID {
				return tc.CompactedAt.Valid
			}
		}
	}
	return false
}

// SelectToolCallsToPrune returns tool call IDs whose prompt output should be
// replaced with ClearedToolResultStub. The current user turn and the previous
// assistant turn are left intact (OpenCode turns < 2). Older completed outputs
// beyond PruneProtectTokens are selected when the dropped amount exceeds
// PruneMinimumTokens.
func SelectToolCallsToPrune(rows []db.Message, callsByMessage map[string][]db.ToolCall) []string {
	turns := 0
	total := 0
	pruned := 0
	var ids []string
	for i := len(rows) - 1; i >= 0; i-- {
		row := rows[i]
		if row.Role == "user" {
			turns++
		}
		if turns < 2 {
			continue
		}
		if row.Role != "assistant" {
			continue
		}
		calls := callsByMessage[row.ID]
		for j := len(calls) - 1; j >= 0; j-- {
			tc := calls[j]
			if tc.CompactedAt.Valid {
				if pruned > PruneMinimumTokens {
					return ids
				}
				return nil
			}
			if _, ok := pruneProtectedTools[tc.ToolName]; ok {
				continue
			}
			if tc.Result == "" {
				continue
			}
			est := EstimateTokens(tc.Result)
			total += est
			if total <= PruneProtectTokens {
				continue
			}
			pruned += est
			ids = append(ids, tc.ID)
		}
	}
	if pruned <= PruneMinimumTokens {
		return nil
	}
	return ids
}
