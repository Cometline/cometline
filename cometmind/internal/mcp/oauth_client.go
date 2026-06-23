package mcp

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/oauth2"
)

// oauthClientInfo is the persisted, non-token side of an MCP OAuth connection.
// It captures everything needed to refresh an access token headlessly at runtime
// (i.e. without re-running discovery or interactive authorization):
//   - the discovered authorization-server token endpoint
//   - the client identity obtained via Dynamic Client Registration (or a
//     preregistered/manual client id)
//   - the resource indicator and scopes used during the original grant
//
// It is written next to the token file as {serverID}.client.json (mode 0600).
type oauthClientInfo struct {
	// AuthorizationEndpoint is the discovered OAuth authorization endpoint.
	// Stored for completeness/debuggability; refresh only needs the token endpoint.
	AuthorizationEndpoint string `json:"authorizationEndpoint,omitempty"`
	// TokenEndpoint is the discovered OAuth token endpoint used for refresh.
	TokenEndpoint string `json:"tokenEndpoint"`
	// ClientID is the registered (DCR) or configured client identifier.
	ClientID string `json:"clientId"`
	// ClientSecret is set only for confidential clients (rare for DCR public clients).
	ClientSecret string `json:"clientSecret,omitempty"`
	// Scopes used for the grant; replayed on refresh.
	Scopes []string `json:"scopes,omitempty"`
	// Resource is the RFC 8707 resource indicator (the canonical MCP server URI).
	Resource string `json:"resource,omitempty"`
	// AuthStyle records how client credentials are sent to the token endpoint.
	// 0 = auto-detect, 1 = in-params (client_secret_post), 2 = in-header (client_secret_basic).
	AuthStyle oauth2.AuthStyle `json:"authStyle,omitempty"`
}

func oauthClientInfoPath(serverID string) (string, error) {
	dir, err := OAuthTokenDir()
	if err != nil {
		return "", err
	}
	id := strings.TrimSpace(serverID)
	if id == "" {
		return "", fmt.Errorf("empty MCP server id")
	}
	return filepath.Join(dir, id+".client.json"), nil
}

// loadOAuthClientInfo reads the persisted client info for one MCP server.
func loadOAuthClientInfo(serverID string) (*oauthClientInfo, error) {
	path, err := oauthClientInfoPath(serverID)
	if err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var info oauthClientInfo
	if err := json.Unmarshal(raw, &info); err != nil {
		return nil, fmt.Errorf("parse oauth client info: %w", err)
	}
	if strings.TrimSpace(info.TokenEndpoint) == "" || strings.TrimSpace(info.ClientID) == "" {
		return nil, fmt.Errorf("oauth client info incomplete for server %q", serverID)
	}
	return &info, nil
}

// saveOAuthClientInfo writes the client info for one MCP server (mode 0600).
func saveOAuthClientInfo(serverID string, info *oauthClientInfo) error {
	if info == nil || strings.TrimSpace(info.TokenEndpoint) == "" || strings.TrimSpace(info.ClientID) == "" {
		return fmt.Errorf("incomplete oauth client info")
	}
	path, err := oauthClientInfoPath(serverID)
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}
