package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cometline/cometmind/internal/memory"
)

// RecallTaskOutcome retrieves recent or matching task outcome memories.
type RecallTaskOutcome struct {
	Memory *memory.Service
}

func (RecallTaskOutcome) Spec() ToolSpec {
	return ToolSpec{
		Name:        "recall_task_outcome",
		Description: "Recall recent task outcome memories, optionally filtered by a query about prior work.",
		Parameters: json.RawMessage(`{
			"type":"object",
			"properties":{
				"query":{"type":"string","description":"Optional search query. When omitted, returns recent task outcomes."},
				"limit":{"type":"integer","minimum":1,"maximum":10,"description":"Maximum outcomes to return. Defaults to 5."}
			}
		}`),
	}
}

func (t RecallTaskOutcome) Execute(ctx context.Context, input json.RawMessage) (Result, error) {
	if t.Memory == nil {
		return Result{OK: false, Output: "memory service unavailable"}, nil
	}
	var in struct {
		Query string `json:"query"`
		Limit int    `json:"limit"`
	}
	_ = json.Unmarshal(input, &in)
	limit := in.Limit
	if limit <= 0 {
		limit = 5
	}
	if limit > 10 {
		limit = 10
	}

	query := strings.TrimSpace(in.Query)
	var (
		items []memory.ScoredMemory
		err   error
	)
	if query == "" {
		items, err = t.Memory.RecentTaskOutcomes(ctx, limit)
	} else {
		items, err = t.Memory.SearchTaskOutcomes(ctx, query, limit)
	}
	if err != nil {
		return Result{OK: false, Output: err.Error()}, nil
	}
	if len(items) == 0 {
		return Result{OK: true, Output: "No task outcomes found."}, nil
	}

	var b strings.Builder
	for i, item := range items {
		fmt.Fprintf(&b, "%d. %s", i+1, strings.TrimSpace(item.Content))
		if item.SourceSessionID != "" {
			fmt.Fprintf(&b, " (session %s)", item.SourceSessionID)
		}
		b.WriteString("\n")
	}
	return Result{OK: true, Output: strings.TrimSpace(b.String())}, nil
}
