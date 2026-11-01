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
	fields := catalog.ServerCatalogFields{
		Source:         string(srv.Source),
		CatalogID:      srv.CatalogID,
		CatalogVersion: srv.CatalogVersion,
	}
	var oauthScopes []string
	if srv.OAuth != nil {
		oauthScopes = append([]string(nil), srv.OAuth.Scopes...)
	}
	name, transport, url, scopes, allowed, version, changed := catalog.SyncLockedServer(
		fields,
		srv.Name,
		string(srv.Transport),
		srv.URL,
		oauthScopes,
		srv.AllowedTools,
	)
	if !changed {
		return srv
	}
	srv.Name = name
	srv.Transport = MCPTransport(normalizeMCPTransport(transport))
	srv.URL = url
	srv.AllowedTools = allowed
	srv.CatalogVersion = version
	if len(scopes) > 0 || srv.OAuth != nil {
		if srv.OAuth == nil {
			srv.OAuth = &MCPOAuthConfig{}
		}
		srv.OAuth.Scopes = scopes
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
