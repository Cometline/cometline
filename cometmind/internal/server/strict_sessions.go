package server

import (
	"context"

	"github.com/Cometline/cometline/cometmind/internal/apigen"
)

func (s *strictServer) ListSessions(ctx context.Context, _ apigen.ListSessionsRequestObject) (apigen.ListSessionsResponseObject, error) {
	delegateGin(ctx, s.app.handleListSessions)
	return nil, nil
}

func (s *strictServer) CreateSession(ctx context.Context, _ apigen.CreateSessionRequestObject) (apigen.CreateSessionResponseObject, error) {
	delegateGin(ctx, s.app.handleCreateSession)
	return nil, nil
}

func (s *strictServer) DeleteSession(ctx context.Context, _ apigen.DeleteSessionRequestObject) (apigen.DeleteSessionResponseObject, error) {
	delegateGin(ctx, s.app.handleDeleteSession)
	return nil, nil
}

func (s *strictServer) GetSession(ctx context.Context, _ apigen.GetSessionRequestObject) (apigen.GetSessionResponseObject, error) {
	delegateGin(ctx, s.app.handleGetSession)
	return nil, nil
}

func (s *strictServer) PatchSession(ctx context.Context, _ apigen.PatchSessionRequestObject) (apigen.PatchSessionResponseObject, error) {
	delegateGin(ctx, s.app.handlePatchSession)
	return nil, nil
}

func (s *strictServer) ListChildSessions(ctx context.Context, _ apigen.ListChildSessionsRequestObject) (apigen.ListChildSessionsResponseObject, error) {
	delegateGin(ctx, s.app.handleListChildSessions)
	return nil, nil
}

func (s *strictServer) IngestSessionEvent(ctx context.Context, _ apigen.IngestSessionEventRequestObject) (apigen.IngestSessionEventResponseObject, error) {
	delegateGin(ctx, s.app.handleIngestSessionEvent)
	return nil, nil
}

func (s *strictServer) ForkSession(ctx context.Context, _ apigen.ForkSessionRequestObject) (apigen.ForkSessionResponseObject, error) {
	delegateGin(ctx, s.app.handleForkSession)
	return nil, nil
}

func (s *strictServer) ClearSession(ctx context.Context, _ apigen.ClearSessionRequestObject) (apigen.ClearSessionResponseObject, error) {
	delegateGin(ctx, s.app.handleClearSession)
	return nil, nil
}

func (s *strictServer) GetSessionMessages(ctx context.Context, _ apigen.GetSessionMessagesRequestObject) (apigen.GetSessionMessagesResponseObject, error) {
	delegateGin(ctx, s.app.handleGetMessages)
	return nil, nil
}

func (s *strictServer) AbortSession(ctx context.Context, _ apigen.AbortSessionRequestObject) (apigen.AbortSessionResponseObject, error) {
	delegateGin(ctx, s.app.handleAbortSession)
	return nil, nil
}

func (s *strictServer) ListSkillDrafts(ctx context.Context, _ apigen.ListSkillDraftsRequestObject) (apigen.ListSkillDraftsResponseObject, error) {
	delegateGin(ctx, s.app.handleListSkillDrafts)
	return nil, nil
}

func (s *strictServer) RejectSkillDraft(ctx context.Context, _ apigen.RejectSkillDraftRequestObject) (apigen.RejectSkillDraftResponseObject, error) {
	delegateGin(ctx, s.app.handleRejectSkillDraft)
	return nil, nil
}

func (s *strictServer) GetSkillDraft(ctx context.Context, _ apigen.GetSkillDraftRequestObject) (apigen.GetSkillDraftResponseObject, error) {
	delegateGin(ctx, s.app.handleGetSkillDraft)
	return nil, nil
}

func (s *strictServer) UpdateSkillDraft(ctx context.Context, _ apigen.UpdateSkillDraftRequestObject) (apigen.UpdateSkillDraftResponseObject, error) {
	delegateGin(ctx, s.app.handleUpdateSkillDraft)
	return nil, nil
}

func (s *strictServer) PromoteSkillDraft(ctx context.Context, _ apigen.PromoteSkillDraftRequestObject) (apigen.PromoteSkillDraftResponseObject, error) {
	delegateGin(ctx, s.app.handlePromoteSkillDraft)
	return nil, nil
}

func (s *strictServer) ListSkills(ctx context.Context, _ apigen.ListSkillsRequestObject) (apigen.ListSkillsResponseObject, error) {
	delegateGin(ctx, s.app.handleListSkills)
	return nil, nil
}

func (s *strictServer) SyncSkills(ctx context.Context, _ apigen.SyncSkillsRequestObject) (apigen.SyncSkillsResponseObject, error) {
	delegateGin(ctx, s.app.handleSyncSkills)
	return nil, nil
}

func (s *strictServer) DeleteSkill(ctx context.Context, _ apigen.DeleteSkillRequestObject) (apigen.DeleteSkillResponseObject, error) {
	delegateGin(ctx, s.app.handleDeleteSkill)
	return nil, nil
}

func (s *strictServer) GetSkill(ctx context.Context, _ apigen.GetSkillRequestObject) (apigen.GetSkillResponseObject, error) {
	delegateGin(ctx, s.app.handleGetSkill)
	return nil, nil
}

func (s *strictServer) UpdateSkill(ctx context.Context, _ apigen.UpdateSkillRequestObject) (apigen.UpdateSkillResponseObject, error) {
	delegateGin(ctx, s.app.handleUpdateSkill)
	return nil, nil
}
