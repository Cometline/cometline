package session

import (
	"context"
	"encoding/json"
	"strings"

	cometsdk "github.com/Cometline/cometline/comet-sdk"
	"github.com/Cometline/cometline/cometmind/internal/db"
	"github.com/Cometline/cometline/cometmind/internal/logging"
	"github.com/Cometline/cometline/cometmind/internal/usage"
	"go.uber.org/zap"
)

// SaveTokenUsage accumulates session token totals and appends a ledger row.
// Ledger failures are logged and never fail the caller: the model has already
// been billed and the assistant step still needs to persist. providerID/modelID
// are the step-time ids; empty values fall back to the session's current model.
func (s *Service) SaveTokenUsage(ctx context.Context, sessionID string, u cometsdk.TokenUsage, providerID, modelID string) error {
	sess, err := s.GetSession(ctx, sessionID)
	if err != nil {
		logging.L().Warn("usage.session_lookup_failed", zap.String("session", sessionID), zap.Error(err))
		s.recordAgentStep(ctx, sessionID, "", providerID, modelID, u)
		return nil
	}
	var current cometsdk.TokenUsage
	if strings.TrimSpace(sess.TokenUsage) != "" && sess.TokenUsage != "{}" {
		if unmarshalErr := json.Unmarshal([]byte(sess.TokenUsage), &current); unmarshalErr != nil {
			logging.L().Warn("usage.token_usage_json_invalid", zap.String("session", sessionID), zap.Error(unmarshalErr))
			current = cometsdk.TokenUsage{}
		}
	}
	current.InputTokens += u.InputTokens
	current.OutputTokens += u.OutputTokens
	current.CacheRead += u.CacheRead
	current.CacheWrite += u.CacheWrite
	b, err := json.Marshal(current)
	if err != nil {
		return err
	}
	if err := s.q.UpdateSessionTokenUsage(ctx, db.UpdateSessionTokenUsageParams{
		TokenUsage: string(b),
		ID:         sessionID,
	}); err != nil {
		return err
	}
	if providerID == "" {
		providerID = sess.ProviderID
	}
	if modelID == "" {
		modelID = sess.ModelID
	}
	s.recordAgentStep(ctx, sessionID, sess.WorkspaceID, providerID, modelID, u)
	return nil
}

func (s *Service) recordAgentStep(ctx context.Context, sessionID, workspaceID, providerID, modelID string, u cometsdk.TokenUsage) {
	if s.usage == nil {
		return
	}
	if err := s.usage.Record(ctx, usage.Event{
		WorkspaceID: workspaceID,
		SessionID:   sessionID,
		ProviderID:  providerID,
		ModelID:     modelID,
		CallKind:    usage.KindAgentStep,
		Usage:       u,
	}); err != nil {
		logging.L().Warn("usage.record_failed", zap.String("kind", usage.KindAgentStep), zap.String("session", sessionID), zap.Error(err))
	}
}
