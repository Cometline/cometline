package session

import (
	"context"

	"github.com/Cometline/cometline/cometmind/internal/db"
	"github.com/Cometline/cometline/cometmind/internal/id"
)

// UpsertGatewaySession maps an external chat surface to a CometMind session.
func (s *Service) UpsertGatewaySession(ctx context.Context, platform, userID, channelID, threadID, sessionID, workspaceID string) (db.GatewaySession, error) {
	return s.q.UpsertGatewaySession(ctx, db.UpsertGatewaySessionParams{
		ID:                 id.New(),
		Platform:           platform,
		PlatformUserID:     userID,
		PlatformChannelID:  channelID,
		ThreadID:           threadID,
		CometmindSessionID: sessionID,
		WorkspaceID:        workspaceID,
	})
}

// LookupGatewaySession finds a mapped CometMind session for a platform identity.
func (s *Service) LookupGatewaySession(ctx context.Context, platform, userID, channelID, threadID string) (db.GatewaySession, error) {
	return s.q.GetGatewaySession(ctx, db.GetGatewaySessionParams{
		Platform:          platform,
		PlatformUserID:    userID,
		PlatformChannelID: channelID,
		ThreadID:          threadID,
	})
}
