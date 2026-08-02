package session

import (
	"encoding/json"
	"fmt"
	"strings"

	cometsdk "github.com/cometline/comet-sdk"
	"github.com/cometline/cometmind/internal/apigen"
)

// APISession converts a persisted session into the OpenAPI Session wire shape.
func APISession(sess Session, workspacePath string) (apigen.Session, error) {
	usage, err := decodeAPITokenUsage(sess.TokenUsage)
	if err != nil {
		return apigen.Session{}, err
	}

	out := apigen.Session{
		Id:            sess.ID,
		WorkspaceId:   sess.WorkspaceID,
		WorkspacePath: workspacePath,
		Title:         sess.Title,
		ModelId:       sess.ModelID,
		ProviderId:    sess.ProviderID,
		Status:        apigen.SessionStatus(sess.Status),
		Origin:        apigen.SessionOrigin(sess.Origin),
		TokenUsage:    usage,
		AgentMode:     apigen.AgentMode(sess.AgentMode),
		Pinned:        sess.Pinned,
		CreatedAt:     sess.CreatedAt,
		UpdatedAt:     sess.UpdatedAt,
	}

	if sess.ParentSessionID != "" {
		out.ParentSessionId = &sess.ParentSessionID
	}
	if sess.Purpose != "" {
		out.Purpose = &sess.Purpose
	}
	if sess.DelegationStatus != "" {
		status := apigen.SessionDelegationStatus(sess.DelegationStatus.String())
		out.DelegationStatus = &status
	}
	if sess.OutputSummary != "" {
		out.OutputSummary = &sess.OutputSummary
	}
	if sess.ACPSessionID != "" {
		out.AcpSessionId = &sess.ACPSessionID
	}
	if sess.PendingQuestion != "" {
		out.PendingQuestion = &sess.PendingQuestion
	}
	if sess.SubagentKind != "" {
		kind := apigen.SessionSubagentKind(sess.SubagentKind)
		out.SubagentKind = &kind
	}
	if gw := apiGateway(sess.Gateway); gw != nil {
		out.Gateway = gw
	}

	return out, nil
}

// APISessionList converts sessions using pathByWorkspaceID to fill workspace_path.
func APISessionList(sessions []Session, pathByWorkspaceID map[string]string) ([]apigen.Session, error) {
	out := make([]apigen.Session, 0, len(sessions))
	for _, sess := range sessions {
		wire, err := APISession(sess, pathByWorkspaceID[sess.WorkspaceID])
		if err != nil {
			return nil, err
		}
		out = append(out, wire)
	}
	return out, nil
}

func decodeAPITokenUsage(raw string) (apigen.TokenUsage, error) {
	if strings.TrimSpace(raw) == "" {
		return apigen.TokenUsage{}, nil
	}

	var usage cometsdk.TokenUsage
	if err := json.Unmarshal([]byte(raw), &usage); err != nil {
		return apigen.TokenUsage{}, fmt.Errorf("decode token usage: %w", err)
	}
	return apigen.TokenUsage{
		InputTokens:  usage.InputTokens,
		OutputTokens: usage.OutputTokens,
		CacheRead:    usage.CacheRead,
		CacheWrite:   usage.CacheWrite,
	}, nil
}

func apiGateway(gw *SessionGateway) *struct {
	ChannelId *string                        `json:"channel_id,omitempty"`
	Platform  *apigen.SessionGatewayPlatform `json:"platform,omitempty"`
	ThreadId  *string                        `json:"thread_id,omitempty"`
} {
	if gw == nil || gw.Platform == "" {
		return nil
	}
	platform := apigen.SessionGatewayPlatform(gw.Platform)
	out := &struct {
		ChannelId *string                        `json:"channel_id,omitempty"`
		Platform  *apigen.SessionGatewayPlatform `json:"platform,omitempty"`
		ThreadId  *string                        `json:"thread_id,omitempty"`
	}{
		Platform: &platform,
	}
	if gw.ChannelID != "" {
		out.ChannelId = &gw.ChannelID
	}
	if gw.ThreadID != "" {
		out.ThreadId = &gw.ThreadID
	}
	return out
}
