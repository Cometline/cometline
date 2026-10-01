package server

import (
	"context"

	"github.com/Cometline/cometline/cometmind/internal/apigen"
)

func (s *strictServer) RunStorageBackup(ctx context.Context, _ apigen.RunStorageBackupRequestObject) (apigen.RunStorageBackupResponseObject, error) {
	delegateGin(ctx, s.app.handleRunBackup)
	return nil, nil
}

func (s *strictServer) RunStorageRetention(ctx context.Context, _ apigen.RunStorageRetentionRequestObject) (apigen.RunStorageRetentionResponseObject, error) {
	delegateGin(ctx, s.app.handleRunStorageRetention)
	return nil, nil
}

func (s *strictServer) ListUsageEvents(ctx context.Context, _ apigen.ListUsageEventsRequestObject) (apigen.ListUsageEventsResponseObject, error) {
	delegateGin(ctx, s.app.handleListUsageEvents)
	return nil, nil
}

func (s *strictServer) GetUsageSeries(ctx context.Context, _ apigen.GetUsageSeriesRequestObject) (apigen.GetUsageSeriesResponseObject, error) {
	delegateGin(ctx, s.app.handleGetUsageSeries)
	return nil, nil
}

func (s *strictServer) GetUsageSummary(ctx context.Context, _ apigen.GetUsageSummaryRequestObject) (apigen.GetUsageSummaryResponseObject, error) {
	delegateGin(ctx, s.app.handleGetUsageSummary)
	return nil, nil
}

func (s *strictServer) ListWikiFiles(ctx context.Context, _ apigen.ListWikiFilesRequestObject) (apigen.ListWikiFilesResponseObject, error) {
	delegateGin(ctx, s.app.handleListWikiFiles)
	return nil, nil
}

func (s *strictServer) ListWikiFileBacklinks(ctx context.Context, _ apigen.ListWikiFileBacklinksRequestObject) (apigen.ListWikiFileBacklinksResponseObject, error) {
	delegateGin(ctx, s.app.handleListWikiFileBacklinks)
	return nil, nil
}

func (s *strictServer) ListWikiFileChildren(ctx context.Context, _ apigen.ListWikiFileChildrenRequestObject) (apigen.ListWikiFileChildrenResponseObject, error) {
	delegateGin(ctx, s.app.handleListWikiFileChildren)
	return nil, nil
}

func (s *strictServer) ReadWikiFileContent(ctx context.Context, _ apigen.ReadWikiFileContentRequestObject) (apigen.ReadWikiFileContentResponseObject, error) {
	delegateGin(ctx, s.app.handleReadWikiFileContent)
	return nil, nil
}

func (s *strictServer) WriteWikiFileContent(ctx context.Context, _ apigen.WriteWikiFileContentRequestObject) (apigen.WriteWikiFileContentResponseObject, error) {
	delegateGin(ctx, s.app.handleWriteWikiFileContent)
	return nil, nil
}

func (s *strictServer) DeleteWorkspace(ctx context.Context, _ apigen.DeleteWorkspaceRequestObject) (apigen.DeleteWorkspaceResponseObject, error) {
	delegateGin(ctx, s.app.handleDeleteWorkspace)
	return nil, nil
}

func (s *strictServer) ListWorkspaces(ctx context.Context, _ apigen.ListWorkspacesRequestObject) (apigen.ListWorkspacesResponseObject, error) {
	delegateGin(ctx, s.app.handleListWorkspaces)
	return nil, nil
}

func (s *strictServer) CreateWorkspace(ctx context.Context, _ apigen.CreateWorkspaceRequestObject) (apigen.CreateWorkspaceResponseObject, error) {
	delegateGin(ctx, s.app.handleCreateWorkspace)
	return nil, nil
}

func (s *strictServer) ListWorkspaceFiles(ctx context.Context, _ apigen.ListWorkspaceFilesRequestObject) (apigen.ListWorkspaceFilesResponseObject, error) {
	delegateGin(ctx, s.app.handleListWorkspaceFiles)
	return nil, nil
}

func (s *strictServer) ListWorkspaceFileChildren(ctx context.Context, _ apigen.ListWorkspaceFileChildrenRequestObject) (apigen.ListWorkspaceFileChildrenResponseObject, error) {
	delegateGin(ctx, s.app.handleListWorkspaceFileChildren)
	return nil, nil
}

func (s *strictServer) ReadWorkspaceFileContent(ctx context.Context, _ apigen.ReadWorkspaceFileContentRequestObject) (apigen.ReadWorkspaceFileContentResponseObject, error) {
	delegateGin(ctx, s.app.handleReadWorkspaceFileContent)
	return nil, nil
}

func (s *strictServer) WriteWorkspaceFileContent(ctx context.Context, _ apigen.WriteWorkspaceFileContentRequestObject) (apigen.WriteWorkspaceFileContentResponseObject, error) {
	delegateGin(ctx, s.app.handleWriteWorkspaceFileContent)
	return nil, nil
}

func (s *strictServer) CommitWorkspaceGit(ctx context.Context, _ apigen.CommitWorkspaceGitRequestObject) (apigen.CommitWorkspaceGitResponseObject, error) {
	delegateGin(ctx, s.app.handleWorkspaceGitCommit)
	return nil, nil
}

func (s *strictServer) GetWorkspaceGitDiff(ctx context.Context, _ apigen.GetWorkspaceGitDiffRequestObject) (apigen.GetWorkspaceGitDiffResponseObject, error) {
	delegateGin(ctx, s.app.handleWorkspaceGitDiff)
	return nil, nil
}

func (s *strictServer) DiscardWorkspaceGitPaths(ctx context.Context, _ apigen.DiscardWorkspaceGitPathsRequestObject) (apigen.DiscardWorkspaceGitPathsResponseObject, error) {
	delegateGin(ctx, s.app.handleWorkspaceGitDiscard)
	return nil, nil
}

func (s *strictServer) StageWorkspaceGitPaths(ctx context.Context, _ apigen.StageWorkspaceGitPathsRequestObject) (apigen.StageWorkspaceGitPathsResponseObject, error) {
	delegateGin(ctx, s.app.handleWorkspaceGitStage)
	return nil, nil
}

func (s *strictServer) GetWorkspaceGitStatus(ctx context.Context, _ apigen.GetWorkspaceGitStatusRequestObject) (apigen.GetWorkspaceGitStatusResponseObject, error) {
	delegateGin(ctx, s.app.handleWorkspaceGitStatus)
	return nil, nil
}

func (s *strictServer) UnstageWorkspaceGitPaths(ctx context.Context, _ apigen.UnstageWorkspaceGitPathsRequestObject) (apigen.UnstageWorkspaceGitPathsResponseObject, error) {
	delegateGin(ctx, s.app.handleWorkspaceGitUnstage)
	return nil, nil
}

func (s *strictServer) PruneWorkspaces(ctx context.Context, _ apigen.PruneWorkspacesRequestObject) (apigen.PruneWorkspacesResponseObject, error) {
	delegateGin(ctx, s.app.handlePruneWorkspaces)
	return nil, nil
}
