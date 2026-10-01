package agent

import (
	"context"
	"fmt"
	"strings"
)

func (r *Runner) hasActiveSubagents(parentSessionID string) bool {
	return r.SubagentOrchestrator != nil && r.SubagentOrchestrator.ActiveCount(parentSessionID) > 0
}

func (r *Runner) collectActiveSubagentResults(ctx context.Context, parentSessionID string) (string, bool, error) {
	if !r.hasActiveSubagents(parentSessionID) {
		return "", false, nil
	}
	if r.SubagentOrchestrator == nil {
		return "", false, fmt.Errorf("subagent waiting is not configured")
	}

	results, err := r.SubagentOrchestrator.Wait(ctx, parentSessionID, nil)
	if err != nil {
		return "", false, err
	}
	var b strings.Builder
	for _, res := range results {
		writeCollectedSubagentResult(&b, res.ChildSessionID, string(res.Kind), res.Status, res.Summary)
	}
	return strings.TrimSpace(b.String()), true, nil
}

func writeCollectedSubagentResult(b *strings.Builder, id, kind, status, summary string) {
	if b.Len() > 0 {
		b.WriteString("\n\n")
	}
	fmt.Fprintf(b, "child_session_id: %s\nkind: %s\nstatus: %s\n\n%s", id, kind, status, summary)
}
