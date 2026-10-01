package server

import (
	"context"

	"github.com/Cometline/cometline/cometmind/internal/apigen"
)

func (s *strictServer) GetHealth(ctx context.Context, _ apigen.GetHealthRequestObject) (apigen.GetHealthResponseObject, error) {
	delegateGin(ctx, s.app.handleHealth)
	return nil, nil
}

func (s *strictServer) ListInboxMessages(ctx context.Context, _ apigen.ListInboxMessagesRequestObject) (apigen.ListInboxMessagesResponseObject, error) {
	delegateGin(ctx, s.app.handleListInboxMessages)
	return nil, nil
}

func (s *strictServer) DismissInboxMessage(ctx context.Context, _ apigen.DismissInboxMessageRequestObject) (apigen.DismissInboxMessageResponseObject, error) {
	delegateGin(ctx, s.app.handleDismissInboxMessage)
	return nil, nil
}

func (s *strictServer) ReplyInboxMessage(ctx context.Context, _ apigen.ReplyInboxMessageRequestObject) (apigen.ReplyInboxMessageResponseObject, error) {
	delegateGin(ctx, s.app.handleReplyInboxMessage)
	return nil, nil
}

func (s *strictServer) GetInboxSummary(ctx context.Context, _ apigen.GetInboxSummaryRequestObject) (apigen.GetInboxSummaryResponseObject, error) {
	delegateGin(ctx, s.app.handleGetInboxSummary)
	return nil, nil
}

func (s *strictServer) ListJobs(ctx context.Context, _ apigen.ListJobsRequestObject) (apigen.ListJobsResponseObject, error) {
	delegateGin(ctx, s.app.handleListJobs)
	return nil, nil
}

func (s *strictServer) CreateJob(ctx context.Context, _ apigen.CreateJobRequestObject) (apigen.CreateJobResponseObject, error) {
	delegateGin(ctx, s.app.handleCreateJob)
	return nil, nil
}

func (s *strictServer) GetJobSettings(ctx context.Context, _ apigen.GetJobSettingsRequestObject) (apigen.GetJobSettingsResponseObject, error) {
	delegateGin(ctx, s.app.handleGetJobSettings)
	return nil, nil
}

func (s *strictServer) PutJobSettings(ctx context.Context, _ apigen.PutJobSettingsRequestObject) (apigen.PutJobSettingsResponseObject, error) {
	delegateGin(ctx, s.app.handlePutJobSettings)
	return nil, nil
}

func (s *strictServer) DeleteJob(ctx context.Context, _ apigen.DeleteJobRequestObject) (apigen.DeleteJobResponseObject, error) {
	delegateGin(ctx, s.app.handleDeleteJob)
	return nil, nil
}

func (s *strictServer) GetJob(ctx context.Context, _ apigen.GetJobRequestObject) (apigen.GetJobResponseObject, error) {
	delegateGin(ctx, s.app.handleGetJob)
	return nil, nil
}

func (s *strictServer) UpdateJob(ctx context.Context, _ apigen.UpdateJobRequestObject) (apigen.UpdateJobResponseObject, error) {
	delegateGin(ctx, s.app.handleUpdateJob)
	return nil, nil
}

func (s *strictServer) UnarchiveJob(ctx context.Context, _ apigen.UnarchiveJobRequestObject) (apigen.UnarchiveJobResponseObject, error) {
	delegateGin(ctx, s.app.handleUnarchiveJob)
	return nil, nil
}

func (s *strictServer) ArchiveJob(ctx context.Context, _ apigen.ArchiveJobRequestObject) (apigen.ArchiveJobResponseObject, error) {
	delegateGin(ctx, s.app.handleArchiveJob)
	return nil, nil
}

func (s *strictServer) CompleteJob(ctx context.Context, _ apigen.CompleteJobRequestObject) (apigen.CompleteJobResponseObject, error) {
	delegateGin(ctx, s.app.handleCompleteJob)
	return nil, nil
}

func (s *strictServer) ListJobEvents(ctx context.Context, _ apigen.ListJobEventsRequestObject) (apigen.ListJobEventsResponseObject, error) {
	delegateGin(ctx, s.app.handleListJobEvents)
	return nil, nil
}

func (s *strictServer) ReleaseJob(ctx context.Context, _ apigen.ReleaseJobRequestObject) (apigen.ReleaseJobResponseObject, error) {
	delegateGin(ctx, s.app.handleReleaseJob)
	return nil, nil
}

func (s *strictServer) HeartbeatJob(ctx context.Context, _ apigen.HeartbeatJobRequestObject) (apigen.HeartbeatJobResponseObject, error) {
	delegateGin(ctx, s.app.handleHeartbeatJob)
	return nil, nil
}

func (s *strictServer) ClaimJob(ctx context.Context, _ apigen.ClaimJobRequestObject) (apigen.ClaimJobResponseObject, error) {
	delegateGin(ctx, s.app.handleClaimJob)
	return nil, nil
}

func (s *strictServer) UnblockJob(ctx context.Context, _ apigen.UnblockJobRequestObject) (apigen.UnblockJobResponseObject, error) {
	delegateGin(ctx, s.app.handleUnblockJob)
	return nil, nil
}

func (s *strictServer) ListScheduledJobs(ctx context.Context, _ apigen.ListScheduledJobsRequestObject) (apigen.ListScheduledJobsResponseObject, error) {
	delegateGin(ctx, s.app.handleListScheduledJobs)
	return nil, nil
}

func (s *strictServer) CreateScheduledJob(ctx context.Context, _ apigen.CreateScheduledJobRequestObject) (apigen.CreateScheduledJobResponseObject, error) {
	delegateGin(ctx, s.app.handleCreateScheduledJob)
	return nil, nil
}

func (s *strictServer) DeleteScheduledJob(ctx context.Context, _ apigen.DeleteScheduledJobRequestObject) (apigen.DeleteScheduledJobResponseObject, error) {
	delegateGin(ctx, s.app.handleDeleteScheduledJob)
	return nil, nil
}

func (s *strictServer) GetScheduledJob(ctx context.Context, _ apigen.GetScheduledJobRequestObject) (apigen.GetScheduledJobResponseObject, error) {
	delegateGin(ctx, s.app.handleGetScheduledJob)
	return nil, nil
}

func (s *strictServer) UpdateScheduledJob(ctx context.Context, _ apigen.UpdateScheduledJobRequestObject) (apigen.UpdateScheduledJobResponseObject, error) {
	delegateGin(ctx, s.app.handlePatchScheduledJob)
	return nil, nil
}
