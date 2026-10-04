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
	if r.SubagentOrchestrator == nil {
		return "", false, nil
	}

	// Wait also returns children that finished before this check. Sampling
	// ActiveCount first drops those results and skips the synthesis step.
	results, err := r.SubagentOrchestrator.Wait(ctx, parentSessionID, nil)
	if err != nil {
		return "", false, err
	}
	if len(results) == 0 {
		return "", false, nil
	}
	var b strings.Builder
	for _, res := range results {
		writeCollectedSubagentResult(&b, res.ChildSessionID, string(res.Kind), res.Status, res.Summary)
	}
	collected := strings.TrimSpace(b.String())
	if collected == "" {
		return "", false, nil
	}
	return collected, true, nil
}

func writeCollectedSubagentResult(b *strings.Builder, id, kind, status, summary string) {
	if b.Len() > 0 {
		b.WriteString("\n\n")
	}
	fmt.Fprintf(b, "child_session_id: %s\nkind: %s\nstatus: %s\n\n%s", id, kind, status, summary)
}
