package server

import (
	"context"

	"github.com/Cometline/cometline/cometmind/internal/apigen"
)

func (s *strictServer) ListMcpServers(ctx context.Context, _ apigen.ListMcpServersRequestObject) (apigen.ListMcpServersResponseObject, error) {
	delegateGin(ctx, s.app.handleListMCPServers)
	return nil, nil
}

func (s *strictServer) TestMcpServer(ctx context.Context, _ apigen.TestMcpServerRequestObject) (apigen.TestMcpServerResponseObject, error) {
	delegateGin(ctx, s.app.handleTestMCPServer)
	return nil, nil
}

func (s *strictServer) StartMcpOAuth(ctx context.Context, _ apigen.StartMcpOAuthRequestObject) (apigen.StartMcpOAuthResponseObject, error) {
	delegateGin(ctx, s.app.handleStartMCPOAuth)
	return nil, nil
}

func (s *strictServer) ReconnectMcpServer(ctx context.Context, _ apigen.ReconnectMcpServerRequestObject) (apigen.ReconnectMcpServerResponseObject, error) {
	delegateGin(ctx, s.app.handleReconnectMCPServer)
	return nil, nil
}

func (s *strictServer) ListMcpTools(ctx context.Context, _ apigen.ListMcpToolsRequestObject) (apigen.ListMcpToolsResponseObject, error) {
	delegateGin(ctx, s.app.handleListMCPTools)
	return nil, nil
}

func (s *strictServer) ListMedia(ctx context.Context, _ apigen.ListMediaRequestObject) (apigen.ListMediaResponseObject, error) {
	delegateGin(ctx, s.app.handleListMedia)
	return nil, nil
}

func (s *strictServer) DeleteMedia(ctx context.Context, _ apigen.DeleteMediaRequestObject) (apigen.DeleteMediaResponseObject, error) {
	delegateGin(ctx, s.app.handleDeleteMedia)
	return nil, nil
}

func (s *strictServer) ImportMedia(ctx context.Context, _ apigen.ImportMediaRequestObject) (apigen.ImportMediaResponseObject, error) {
	delegateGin(ctx, s.app.handleImportMedia)
	return nil, nil
}

func (s *strictServer) ListMemories(ctx context.Context, _ apigen.ListMemoriesRequestObject) (apigen.ListMemoriesResponseObject, error) {
	delegateGin(ctx, s.app.handleListMemories)
	return nil, nil
}

func (s *strictServer) CreateMemory(ctx context.Context, _ apigen.CreateMemoryRequestObject) (apigen.CreateMemoryResponseObject, error) {
	delegateGin(ctx, s.app.handleCreateMemory)
	return nil, nil
}

func (s *strictServer) CompactMemoryPreview(ctx context.Context, _ apigen.CompactMemoryPreviewRequestObject) (apigen.CompactMemoryPreviewResponseObject, error) {
	delegateGin(ctx, s.app.handleCompactPreview)
	return nil, nil
}

func (s *strictServer) CompactMemory(ctx context.Context, _ apigen.CompactMemoryRequestObject) (apigen.CompactMemoryResponseObject, error) {
	delegateGin(ctx, s.app.handleCompactMemory)
	return nil, nil
}

func (s *strictServer) PurgeArchivedMemory(ctx context.Context, _ apigen.PurgeArchivedMemoryRequestObject) (apigen.PurgeArchivedMemoryResponseObject, error) {
	delegateGin(ctx, s.app.handlePurgeMemory)
	return nil, nil
}

func (s *strictServer) GetMemoryReembedJob(ctx context.Context, _ apigen.GetMemoryReembedJobRequestObject) (apigen.GetMemoryReembedJobResponseObject, error) {
	delegateGin(ctx, s.app.handleGetMemoryReembedJob)
	return nil, nil
}

func (s *strictServer) StartMemoryReembed(ctx context.Context, _ apigen.StartMemoryReembedRequestObject) (apigen.StartMemoryReembedResponseObject, error) {
	delegateGin(ctx, s.app.handleStartMemoryReembed)
	return nil, nil
}

func (s *strictServer) CancelMemoryReembed(ctx context.Context, _ apigen.CancelMemoryReembedRequestObject) (apigen.CancelMemoryReembedResponseObject, error) {
	delegateGin(ctx, s.app.handleCancelMemoryReembed)
	return nil, nil
}

func (s *strictServer) PreviewMemoryReembed(ctx context.Context, _ apigen.PreviewMemoryReembedRequestObject) (apigen.PreviewMemoryReembedResponseObject, error) {
	delegateGin(ctx, s.app.handlePreviewMemoryReembed)
	return nil, nil
}

func (s *strictServer) SearchMemories(ctx context.Context, _ apigen.SearchMemoriesRequestObject) (apigen.SearchMemoriesResponseObject, error) {
	delegateGin(ctx, s.app.handleSearchMemories)
	return nil, nil
}

func (s *strictServer) GetMemorySettings(ctx context.Context, _ apigen.GetMemorySettingsRequestObject) (apigen.GetMemorySettingsResponseObject, error) {
	delegateGin(ctx, s.app.handleGetMemorySettings)
	return nil, nil
}

func (s *strictServer) PutMemorySettings(ctx context.Context, _ apigen.PutMemorySettingsRequestObject) (apigen.PutMemorySettingsResponseObject, error) {
	delegateGin(ctx, s.app.handlePutMemorySettings)
	return nil, nil
}

func (s *strictServer) DeleteMemory(ctx context.Context, _ apigen.DeleteMemoryRequestObject) (apigen.DeleteMemoryResponseObject, error) {
	delegateGin(ctx, s.app.handleDeleteMemory)
	return nil, nil
}

func (s *strictServer) LookupModelCatalog(ctx context.Context, _ apigen.LookupModelCatalogRequestObject) (apigen.LookupModelCatalogResponseObject, error) {
	delegateGin(ctx, s.app.handleLookupModelCatalog)
	return nil, nil
}
