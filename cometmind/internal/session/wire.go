package session

import (
	"encoding/json"
	"fmt"
	"strings"

	cometsdk "github.com/Cometline/cometline/comet-sdk"
)

// WireSession is the session list shape shared by the CLI and the HTTP layer.
// Field names follow the OpenAPI Session schema. The server package maps this
// view onto generated response types.
type WireSession struct {
	AgentMode        string         `json:"agent_mode"`
	CreatedAt        int64          `json:"created_at"`
	DelegationStatus *string        `json:"delegation_status,omitempty"`
	Gateway          *WireGateway   `json:"gateway,omitempty"`
	Id               string         `json:"id"`
	ModelId          string         `json:"model_id"`
	Origin           string         `json:"origin"`
	OutputSummary    *string        `json:"output_summary,omitempty"`
	ParentSessionId  *string        `json:"parent_session_id,omitempty"`
	Pinned           bool           `json:"pinned"`
	ProviderId       string         `json:"provider_id"`
	Purpose          *string        `json:"purpose,omitempty"`
	Running          bool           `json:"running"`
	Status           string         `json:"status"`
	SubagentKind     *string        `json:"subagent_kind,omitempty"`
	Title            string         `json:"title"`
	TokenUsage       WireTokenUsage `json:"token_usage"`
	UpdatedAt        int64          `json:"updated_at"`
	WorkspaceId      string         `json:"workspace_id"`
	WorkspacePath    string         `json:"workspace_path"`
}

// WireTokenUsage matches the OpenAPI token_usage object.
type WireTokenUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
	CacheRead    int `json:"cache_read"`
	CacheWrite   int `json:"cache_write"`
}

// WireGateway matches the OpenAPI session gateway object.
// Field names are the schema's, including the Id suffix.
type WireGateway struct {
	ChannelId *string `json:"channel_id,omitempty"`
	Platform  *string `json:"platform,omitempty"`
	ThreadId  *string `json:"thread_id,omitempty"`
}

// APISession converts a persisted session into the wire session shape.
func APISession(sess Session, workspacePath string) (WireSession, error) {
	usage, err := decodeAPITokenUsage(sess.TokenUsage)
	if err != nil {
		return WireSession{}, err
	}
	out := WireSession{
		Id:            sess.ID,
		WorkspaceId:   sess.WorkspaceID,
		WorkspacePath: workspacePath,
		Title:         sess.Title,
		ModelId:       sess.ModelID,
		ProviderId:    sess.ProviderID,
		Status:        sess.Status,
		Origin:        sess.Origin,
		TokenUsage:    usage,
		AgentMode:     sess.AgentMode,
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
		status := sess.DelegationStatus.String()
		out.DelegationStatus = &status
	}
	if sess.OutputSummary != "" {
		out.OutputSummary = &sess.OutputSummary
	}
	if sess.SubagentKind != "" {
		kind := sess.SubagentKind
		out.SubagentKind = &kind
	}
	if gw := apiGateway(sess.Gateway); gw != nil {
		out.Gateway = gw
	}
	return out, nil
}

// APISessionList converts sessions using pathByWorkspaceID to fill workspace_path.
func APISessionList(sessions []Session, pathByWorkspaceID map[string]string) ([]WireSession, error) {
	out := make([]WireSession, 0, len(sessions))
	for _, sess := range sessions {
		wire, err := APISession(sess, pathByWorkspaceID[sess.WorkspaceID])
		if err != nil {
			return nil, err
		}
		out = append(out, wire)
	}
	return out, nil
}

func decodeAPITokenUsage(raw string) (WireTokenUsage, error) {
	if strings.TrimSpace(raw) == "" {
		return WireTokenUsage{}, nil
	}
	var usage cometsdk.TokenUsage
	if err := json.Unmarshal([]byte(raw), &usage); err != nil {
		return WireTokenUsage{}, fmt.Errorf("decode token usage: %w", err)
	}
	return WireTokenUsage{
		InputTokens:  usage.InputTokens,
		OutputTokens: usage.OutputTokens,
		CacheRead:    usage.CacheRead,
		CacheWrite:   usage.CacheWrite,
	}, nil
}

func apiGateway(gw *SessionGateway) *WireGateway {
	if gw == nil || gw.Platform == "" {
		return nil
	}
	platform := gw.Platform
	out := &WireGateway{Platform: &platform}
	if gw.ChannelID != "" {
		out.ChannelId = &gw.ChannelID
	}
	if gw.ThreadID != "" {
		out.ThreadId = &gw.ThreadID
	}
	return out
}
