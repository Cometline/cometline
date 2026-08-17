package session

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cometline/cometmind/internal/db"
)

// RecentWindowStartIndex returns the index of the first message to keep verbatim
// when preserving the last preserveUserTurns user turns.
func RecentWindowStartIndex(rows []db.Message, preserveUserTurns int) int {
	if preserveUserTurns <= 0 || len(rows) == 0 {
		return len(rows)
	}
	usersSeen := 0
	for i := len(rows) - 1; i >= 0; i-- {
		if rows[i].Role == "user" {
			usersSeen++
			if usersSeen >= preserveUserTurns {
				return i
			}
		}
	}
	return 0
}

// CompactionPrefixRange returns [start, end) slice indices for messages that
// should be folded into the rolling summary on this compaction pass.
// end is the recent-window start index.
func CompactionPrefixRange(rows []db.Message, compactedUntilID string, recentStart int) (start, end int) {
	end = recentStart
	if end <= 0 {
		return 0, 0
	}
	start = 0
	if compactedUntilID != "" {
		for i, row := range rows {
			if row.ID == compactedUntilID {
				start = i + 1
				break
			}
		}
	}
	if start >= end {
		return 0, 0
	}
	return start, end
}

// FilterMessagesAfterCompacted returns rows strictly after compactedUntilID.
func FilterMessagesAfterCompacted(rows []db.Message, compactedUntilID string) []db.Message {
	if compactedUntilID == "" {
		return rows
	}
	for i, row := range rows {
		if row.ID == compactedUntilID {
			return append([]db.Message(nil), rows[i+1:]...)
		}
	}
	return rows
}

// FormatTranscriptForSummary renders message rows as plain text for the summarizer.
func FormatTranscriptForSummary(rows []db.Message) string {
	var b strings.Builder
	for _, row := range rows {
		switch row.Role {
		case "user":
			fmt.Fprintf(&b, "User: %s\n", formatUserContentForSummary(row.Content))
		case "assistant":
			if strings.TrimSpace(row.Content) != "" {
				fmt.Fprintf(&b, "Assistant: %s\n", strings.TrimSpace(row.Content))
			}
		case "tool_result":
			var p toolResultPayload
			if err := json.Unmarshal([]byte(row.Content), &p); err == nil {
				content := strings.TrimSpace(p.Content)
				if len(content) > 2000 {
					content = content[:2000] + "…"
				}
				fmt.Fprintf(&b, "Tool result: %s\n", content)
			}
		case "system":
			continue
		}
	}
	return strings.TrimSpace(b.String())
}

func formatUserContentForSummary(content string) string {
	blocks, err := DecodeMessageContent(content)
	if err != nil {
		return strings.TrimSpace(content)
	}
	text := strings.TrimSpace(PlainTextFromContent(blocks))
	if n := countImageBlocks(blocks); n > 0 {
		note := fmt.Sprintf("[user attached %d image(s)]", n)
		if text == "" {
			return note
		}
		return text + " " + note
	}
	return text
}

func plainTextFromStoredContent(content string) (string, error) {
	if strings.HasPrefix(content, contentEnvelopePrefix) {
		blocks, err := DecodeMessageContent(content)
		if err != nil {
			return "", err
		}
		return PlainTextFromContent(blocks), nil
	}
	return content, nil
}
