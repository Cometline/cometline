package jobs

import (
	"fmt"
	"strings"
)

// ExecutionPrompt builds the agent prompt for running a claimed job.
func ExecutionPrompt(job Job) string {
	dod := strings.TrimSpace(job.DefinitionOfDone)
	if dod == "" {
		dod = "(none specified)"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Please work on: %s\n\nDefinition of done: %s\n", job.Description, dod)
	if progress := strings.TrimSpace(job.Progress); progress != "" {
		b.WriteString("\nPrevious progress (authoritative resume state):\n")
		b.WriteString(progress)
		b.WriteString("\n\nResume protocol (mandatory order):\n")
		b.WriteString("1. If previous progress already satisfies Definition of Done, call `complete_job` immediately with a final summary. Do NOT re-do finished work and stop on text only.\n")
		b.WriteString("2. Otherwise continue ONLY remaining work, call `update_job` with `progress` after each meaningful milestone, then `complete_job` or `release_job`.\n")
		b.WriteString("3. Ending without `complete_job` or `release_job` is a protocol violation.\n")
	} else {
		fmt.Fprintf(
			&b,
			"\nWhile working, call `update_job` with `progress` after each meaningful milestone (and before long tool runs) so another session can resume if this one stops. When finished, call `complete_job` with a final progress summary. Ending without `complete_job` or `release_job` is a protocol violation.\n\n(Use job_id %q when calling job tools.)",
			job.ID,
		)
		return b.String()
	}
	fmt.Fprintf(
		&b,
		"\n(Use job_id %q when calling job tools.)",
		job.ID,
	)
	return b.String()
}
