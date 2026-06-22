package gateway

import (
	"context"
	"fmt"
	"path/filepath"
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
		fmt.Fprintf(&b, "• %s\n", j.Description)
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

// HandleCreateJobSlash creates a todo job from a Discord slash command.
func (r *Router) HandleCreateJobSlash(ctx context.Context, msg InboundMessage, description, definitionOfDone, workspacePath string) (string, error) {
	if r == nil || r.Jobs == nil {
		return "", fmt.Errorf("jobs service is not configured")
	}
	description = strings.TrimSpace(description)
	if description == "" {
		return "", fmt.Errorf("description is required")
	}
	sessID, defaultWS, err := r.sessionWorkspaceForInbound(ctx, msg)
	if err != nil {
		return "", err
	}
	ws := strings.TrimSpace(workspacePath)
	if ws == "" {
		ws = defaultWS
	}
	sourceChannelID := deliveryChannelID(msg)
	job, err := r.Jobs.Create(ctx, jobs.CreateInput{
		Description:      description,
		DefinitionOfDone: strings.TrimSpace(definitionOfDone),
		WorkspacePath:    ws,
		CreatedBy:        jobs.CreatedByUser,
		SourceSessionID:  sessID,
		SourcePlatform:   jobs.PlatformDiscord,
		SourceChannelID:  sourceChannelID,
	})
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("Created job `%s` in workspace `%s`.", job.ID, job.WorkspacePath), nil
}

// HandleJobProposalSelect records the workspace chosen in a Discord select menu.
func (r *Router) HandleJobProposalSelect(proposalID, workspace string) (string, error) {
	if r == nil || r.JobProposals == nil {
		return "", fmt.Errorf("job proposals are not configured")
	}
	workspace = strings.TrimSpace(workspace)
	if workspace == "" {
		return "", fmt.Errorf("workspace is required")
	}
	if !r.JobProposals.SetWorkspace(proposalID, workspace) {
		return "", fmt.Errorf("this job proposal expired; ask again or use /create-job")
	}
	return fmt.Sprintf("Workspace set to `%s`. Click **Confirm** to create the job.", workspace), nil
}

// HandleJobProposalConfirm creates the job from a pending Discord proposal.
func (r *Router) HandleJobProposalConfirm(ctx context.Context, msg InboundMessage, proposalID string) (string, error) {
	if r == nil || r.Jobs == nil || r.JobProposals == nil {
		return "", fmt.Errorf("job proposals are not configured")
	}
	pending, ok := r.JobProposals.Get(proposalID)
	if !ok {
		return "", fmt.Errorf("this job proposal expired; ask again or use /create-job")
	}
	if pending.UserID != msg.UserID {
		return "", fmt.Errorf("only the user who started this proposal can confirm it")
	}
	ws := strings.TrimSpace(pending.SelectedWorkspace)
	if ws == "" {
		ws = strings.TrimSpace(pending.DefaultWorkspace)
	}
	job, err := r.Jobs.Create(ctx, jobs.CreateInput{
		Description:      pending.Description,
		DefinitionOfDone: pending.DefinitionOfDone,
		WorkspacePath:    ws,
		CreatedBy:        jobs.CreatedByUser,
		SourceSessionID:  pending.SessionID,
		SourcePlatform:   jobs.PlatformDiscord,
		SourceChannelID:  pending.SourceChannelID,
	})
	if err != nil {
		return "", err
	}
	r.JobProposals.Remove(proposalID)
	return fmt.Sprintf("Created job `%s` in workspace `%s`.", job.ID, job.WorkspacePath), nil
}

// HandleJobProposalCancel dismisses a pending Discord proposal.
func (r *Router) HandleJobProposalCancel(proposalID string) error {
	if r == nil || r.JobProposals == nil {
		return fmt.Errorf("job proposals are not configured")
	}
	if _, ok := r.JobProposals.Get(proposalID); !ok {
		return fmt.Errorf("this job proposal expired")
	}
	r.JobProposals.Remove(proposalID)
	return nil
}

func (r *Router) sessionForInbound(ctx context.Context, msg InboundMessage) (sessID string, wsPath string, err error) {
	return r.sessionWorkspaceForInbound(ctx, msg)
}

func (r *Router) sessionWorkspaceForInbound(ctx context.Context, msg InboundMessage) (sessID string, wsPath string, err error) {
	configWS := r.Config.Gateway.Discord.WorkspacePath
	if configWS == "" {
		return "", "", fmt.Errorf("gateway workspace_path is not configured")
	}
	ws, err := r.Sessions.EnsureWorkspace(ctx, configWS)
	if err != nil {
		return "", "", err
	}
	sessID, err = r.resolveSession(ctx, msg, ws)
	if err != nil {
		return "", "", err
	}
	sess, err := r.Sessions.GetSession(ctx, sessID)
	if err != nil {
		return "", "", err
	}
	runPath, err := r.Sessions.WorkspacePath(ctx, sess.WorkspaceID)
	if err != nil {
		return "", "", err
	}
	return sessID, filepath.Clean(runPath), nil
}
