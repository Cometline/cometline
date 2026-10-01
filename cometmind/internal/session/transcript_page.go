package session

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/Cometline/cometline/cometmind/internal/db"
)

// LoadTranscriptPage returns a recent (or older-than-before) window of transcript
// entries using keyset pagination on (created_at, id). limit is clamped to
// [1, MaxTranscriptPageLimit]; zero/negative uses DefaultTranscriptPageLimit.
// before is an opaque cursor from a prior page's NextBefore; empty means the
// most recent page.
func (s *Service) LoadTranscriptPage(ctx context.Context, sessionID string, limit int, before string) (TranscriptPage, error) {
	limit = clampTranscriptPageLimit(limit)
	cursor, err := parseTranscriptBefore(before)
	if err != nil {
		return TranscriptPage{}, err
	}

	fetch := int64(limit) + 1 // one extra to detect HasMore
	descRows, err := s.listTranscriptMessagesDesc(ctx, sessionID, cursor, fetch)
	if err != nil {
		return TranscriptPage{}, err
	}

	hasMore := len(descRows) > limit
	if hasMore {
		descRows = descRows[:limit]
	}
	if len(descRows) == 0 {
		return TranscriptPage{Items: []TranscriptEntry{}, HasMore: false}, nil
	}

	// descRows are newest-first; reverse to chronological ASC for the UI.
	rows := reverseMessages(descRows)

	// Turn-boundary snap: if the window starts mid-turn (leading assistant/system),
	// walk back to the anchoring user so the client does not drop orphan body.
	rows, hasMore, err = s.snapTranscriptPageToTurnBoundary(ctx, sessionID, rows, hasMore)
	if err != nil {
		return TranscriptPage{}, err
	}

	callsByMessage, err := s.toolCallsForMessages(ctx, rows)
	if err != nil {
		return TranscriptPage{}, err
	}
	toolResults, err := s.listPageToolResults(ctx, sessionID, rows[0].CreatedAt, cursor)
	if err != nil {
		return TranscriptPage{}, err
	}

	// Merge tool_results into the row set so buildTranscriptEntries can map errors.
	// They are skipped as visible rows but populate toolErr.
	merged := mergeMessagesChronological(rows, toolResults)

	items, err := buildTranscriptEntries(merged, callsByMessage)
	if err != nil {
		return TranscriptPage{}, err
	}

	page := TranscriptPage{Items: items, HasMore: hasMore}
	if hasMore {
		// Oldest visible message in this page — client passes it as before for older history.
		oldest := rows[0]
		page.NextBefore = encodeTranscriptCursor(oldest.CreatedAt, oldest.ID)
	}
	return page, nil
}

func clampTranscriptPageLimit(limit int) int {
	if limit <= 0 {
		return DefaultTranscriptPageLimit
	}
	if limit > MaxTranscriptPageLimit {
		return MaxTranscriptPageLimit
	}
	return limit
}

// transcriptCursor is a decoded (created_at, id) keyset position.
type transcriptCursor struct {
	createdAt int64
	id        string
}

// parseTranscriptBefore decodes an opaque before cursor. A blank cursor means
// the most recent page and yields nil.
func parseTranscriptBefore(before string) (*transcriptCursor, error) {
	before = strings.TrimSpace(before)
	if before == "" {
		return nil, nil
	}
	createdAt, id, err := decodeTranscriptCursor(before)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidTranscriptCursor, err)
	}
	return &transcriptCursor{createdAt: createdAt, id: id}, nil
}

// listTranscriptMessagesDesc returns up to fetch visible rows newest-first,
// strictly older than cursor when it is set.
func (s *Service) listTranscriptMessagesDesc(ctx context.Context, sessionID string, cursor *transcriptCursor, fetch int64) ([]db.Message, error) {
	if cursor == nil {
		return s.q.ListTranscriptMessagesRecent(ctx, db.ListTranscriptMessagesRecentParams{
			SessionID: sessionID,
			RowLimit:  fetch,
		})
	}
	return s.q.ListTranscriptMessagesBefore(ctx, db.ListTranscriptMessagesBeforeParams{
		SessionID:       sessionID,
		BeforeCreatedAt: cursor.createdAt,
		BeforeID:        cursor.id,
		RowLimit:        fetch,
	})
}

func (s *Service) toolCallsForMessages(ctx context.Context, rows []db.Message) (map[string][]db.ToolCall, error) {
	messageIDs := make([]string, 0, len(rows))
	for _, m := range rows {
		messageIDs = append(messageIDs, m.ID)
	}
	var allCalls []db.ToolCall
	if len(messageIDs) > 0 {
		var err error
		allCalls, err = s.q.ListToolCallsByMessageIDs(ctx, messageIDs)
		if err != nil {
			return nil, err
		}
	}
	return GroupToolCallsByMessage(allCalls), nil
}

// listPageToolResults loads the tool_result rows that carry error flags for a
// page: those between the oldest visible message and the before-cursor, or
// unbounded above for the recent page.
func (s *Service) listPageToolResults(ctx context.Context, sessionID string, minCreatedAt int64, cursor *transcriptCursor) ([]db.Message, error) {
	if cursor == nil {
		return s.q.ListToolResultMessagesAfter(ctx, db.ListToolResultMessagesAfterParams{
			SessionID:    sessionID,
			MinCreatedAt: minCreatedAt,
		})
	}
	return s.q.ListToolResultMessagesBetween(ctx, db.ListToolResultMessagesBetweenParams{
		SessionID:    sessionID,
		MinCreatedAt: minCreatedAt,
		MaxCreatedAt: cursor.createdAt,
		MaxID:        cursor.id,
	})
}

// ErrInvalidTranscriptCursor is returned when before cannot be parsed.
var ErrInvalidTranscriptCursor = errors.New("invalid transcript cursor")

func encodeTranscriptCursor(createdAt int64, id string) string {
	return fmt.Sprintf("%d:%s", createdAt, id)
}

func decodeTranscriptCursor(raw string) (createdAt int64, id string, err error) {
	raw = strings.TrimSpace(raw)
	idx := strings.IndexByte(raw, ':')
	if idx <= 0 || idx == len(raw)-1 {
		return 0, "", fmt.Errorf("want created_at:id")
	}
	createdAt, err = strconv.ParseInt(raw[:idx], 10, 64)
	if err != nil {
		return 0, "", err
	}
	id = raw[idx+1:]
	if id == "" {
		return 0, "", fmt.Errorf("empty id")
	}
	return createdAt, id, nil
}

// snapTranscriptMaxExtra caps how far we walk back for an anchoring user.
const snapTranscriptMaxExtra = MaxTranscriptPageLimit

// snapTranscriptPageToTurnBoundary extends a chronological page backward so it
// does not start mid-turn. When rows[0] is not a user and older history exists,
// prepend messages through the anchoring user. hasMore/next_before stay honest
// relative to the expanded window (caller recomputes NextBefore from rows[0]).
func (s *Service) snapTranscriptPageToTurnBoundary(
	ctx context.Context,
	sessionID string,
	rows []db.Message,
	hasMore bool,
) ([]db.Message, bool, error) {
	if len(rows) == 0 || rows[0].Role == "user" || !hasMore {
		return rows, hasMore, nil
	}

	var (
		prefix     []db.Message // chronological older messages to prepend
		collected  []db.Message // newest-first walk buffer toward the user
		overfetch  = int64(32)
		extraCount = 0
		cursorAt   = rows[0].CreatedAt
		cursorID   = rows[0].ID
		stillMore  = true
	)

	for extraCount < snapTranscriptMaxExtra && stillMore {
		batch, err := s.q.ListTranscriptMessagesBefore(ctx, db.ListTranscriptMessagesBeforeParams{
			SessionID:       sessionID,
			BeforeCreatedAt: cursorAt,
			BeforeID:        cursorID,
			RowLimit:        overfetch,
		})
		if err != nil {
			return nil, false, err
		}
		if len(batch) == 0 {
			stillMore = false
			break
		}

		foundUser := false
		for i, m := range batch {
			collected = append(collected, m)
			extraCount++
			if m.Role == "user" {
				foundUser = true
				// Honest has_more relative to the anchoring user (new page oldest).
				more, probeErr := s.historyBeforeAnchor(ctx, sessionID, batch, i, overfetch)
				if probeErr != nil {
					return nil, false, probeErr
				}
				stillMore = more
				break
			}
			if extraCount >= snapTranscriptMaxExtra {
				break
			}
		}

		if foundUser {
			break
		}
		if int64(len(batch)) < overfetch {
			stillMore = false
			break
		}
		oldest := batch[len(batch)-1]
		cursorAt = oldest.CreatedAt
		cursorID = oldest.ID
	}

	if len(collected) == 0 {
		return rows, hasMore, nil
	}
	prefix = reverseMessages(collected)
	out := make([]db.Message, 0, len(prefix)+len(rows))
	out = append(out, prefix...)
	out = append(out, rows...)
	return out, stillMore, nil
}

// historyBeforeAnchor reports whether any message is older than the anchoring
// user at batch[anchor]. It only probes the store when the batch was full and
// the anchor is its oldest row.
func (s *Service) historyBeforeAnchor(ctx context.Context, sessionID string, batch []db.Message, anchor int, overfetch int64) (bool, error) {
	if anchor+1 < len(batch) {
		return true, nil
	}
	if int64(len(batch)) < overfetch {
		return false, nil
	}
	m := batch[anchor]
	probe, err := s.q.ListTranscriptMessagesBefore(ctx, db.ListTranscriptMessagesBeforeParams{
		SessionID:       sessionID,
		BeforeCreatedAt: m.CreatedAt,
		BeforeID:        m.ID,
		RowLimit:        1,
	})
	if err != nil {
		return false, err
	}
	return len(probe) > 0, nil
}

func reverseMessages(in []db.Message) []db.Message {
	out := make([]db.Message, len(in))
	for i := range in {
		out[len(in)-1-i] = in[i]
	}
	return out
}

func mergeMessagesChronological(visible, toolResults []db.Message) []db.Message {
	if len(toolResults) == 0 {
		return visible
	}
	out := make([]db.Message, 0, len(visible)+len(toolResults))
	i, j := 0, 0
	for i < len(visible) && j < len(toolResults) {
		a, b := visible[i], toolResults[j]
		if a.CreatedAt < b.CreatedAt || (a.CreatedAt == b.CreatedAt && a.ID <= b.ID) {
			out = append(out, a)
			i++
		} else {
			out = append(out, b)
			j++
		}
	}
	out = append(out, visible[i:]...)
	out = append(out, toolResults[j:]...)
	return out
}
