package gateway

import (
	"context"
	"fmt"
	"strings"

	"github.com/cometline/cometmind/internal/jobs"
)

// FormatReadyJobsList returns a human-readable list of ready jobs.
func FormatReadyJobsList(items []jobs.Job) string {
	if len(items) == 0 {
		return "No ready jobs."
	}
	var b strings.Builder
	for _, j := range items {
		prefix := ""
		if j.Priority > 0 {
			prefix = fmt.Sprintf("(p=%d) ", j.Priority)
		}
		fmt.Fprintf(&b, "• %s%s\n", prefix, j.Description)
	}
	return strings.TrimSpace(b.String())
}

// HandleJobsSlash lists ready jobs or claims one and returns the execution prompt.
func (r *Router) HandleJobsSlash(ctx context.Context, msg InboundMessage, jobID string) (reply string, runPrompt string, err error) {
	if r == nil || r.Jobs == nil {
		return "", "", fmt.Errorf("jobs service is not configured")
	}
	jobID = strings.TrimSpace(jobID)
	if jobID == "" {
		items, err := r.Jobs.ListReady(ctx)
		if err != nil {
			return "", "", err
		}
		return FormatReadyJobsList(items), "", nil
	}

	sessID, _, err := r.sessionForInbound(ctx, msg)
	if err != nil {
		return "", "", err
	}
	job, err := r.Jobs.Claim(ctx, jobID, sessID)
	if err != nil {
		return "", "", err
	}
	_ = r.Jobs.Heartbeat(ctx, job.ID, sessID)
	return fmt.Sprintf("Claimed: %s. Starting work…", job.Description), jobs.ExecutionPrompt(job), nil
}

func (r *Router) sessionForInbound(ctx context.Context, msg InboundMessage) (sessID string, wsPath string, err error) {
	wsPath = r.Config.Gateway.Discord.WorkspacePath
	if wsPath == "" {
		return "", "", fmt.Errorf("gateway workspace_path is not configured")
	}
	ws, err := r.Sessions.EnsureWorkspace(ctx, wsPath)
	if err != nil {
		return "", "", err
	}
	sessID, err = r.resolveSession(ctx, msg, ws)
	if err != nil {
		return "", "", err
	}
	return sessID, wsPath, nil
}
