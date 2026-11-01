package config

import (
	"strings"

	"github.com/Cometline/cometline/cometmind/internal/mcp/catalog"
)

func syncMCPCatalogServers(servers []MCPServerConfig) []MCPServerConfig {
	if len(servers) == 0 {
		return servers
	}
	out := make([]MCPServerConfig, len(servers))
	for i, srv := range servers {
		out[i] = applyCatalogSyncToServer(srv)
	}
	return out
}

func applyCatalogSyncToServer(srv MCPServerConfig) MCPServerConfig {
	var oauthScopes []string
	if srv.OAuth != nil {
		oauthScopes = append([]string(nil), srv.OAuth.Scopes...)
	}
	synced := catalog.SyncLockedServer(catalog.SyncLockedServerInput{
		Fields: catalog.ServerCatalogFields{
			Source:         string(srv.Source),
			CatalogID:      srv.CatalogID,
			CatalogVersion: srv.CatalogVersion,
		},
		Current: catalog.LockedServerRecipe{
			Name:           srv.Name,
			Transport:      string(srv.Transport),
			URL:            srv.URL,
			OAuthScopes:    oauthScopes,
			AllowedTools:   srv.AllowedTools,
			CatalogVersion: srv.CatalogVersion,
		},
	})
	if !synced.Changed {
		return srv
	}
	next := synced.Next
	srv.Name = next.Name
	srv.Transport = MCPTransport(normalizeMCPTransport(next.Transport))
	srv.URL = next.URL
	srv.AllowedTools = next.AllowedTools
	srv.CatalogVersion = next.CatalogVersion
	if len(next.OAuthScopes) > 0 || srv.OAuth != nil {
		if srv.OAuth == nil {
			srv.OAuth = &MCPOAuthConfig{}
		}
		srv.OAuth.Scopes = next.OAuthScopes
	}
	return srv
}

func catalogSourceFromJSON(raw string) MCPServerSource {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case string(MCPServerSourceCatalog):
		return MCPServerSourceCatalog
	case string(MCPServerSourceImport):
		return MCPServerSourceImport
	case string(MCPServerSourceUser):
		return MCPServerSourceUser
	default:
		return ""
	}
}
