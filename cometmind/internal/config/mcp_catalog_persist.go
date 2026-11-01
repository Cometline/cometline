package config

import (
	"encoding/json"
	"os"

	"github.com/Cometline/cometline/cometmind/internal/mcp/catalog"
)

// rewriteSettingsMCPServers persists catalog-synced MCP server rows back into
// cometline-settings.json so UI and runtime share one on-disk truth.
func rewriteSettingsMCPServers(path string, synced MCPConfig) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var raw cometlineSettingsJSON
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	next := mcpConfigToJSON(synced)
	if jsonEqual(raw.Cometmind.MCP.Servers, next.Servers) &&
		raw.Cometmind.MCP.Enabled == next.Enabled {
		return nil
	}
	raw.Cometmind.MCP = next
	out, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return err
	}
	out = append(out, '\n')
	return os.WriteFile(path, out, 0o600)
}

func mcpConfigToJSON(cfg MCPConfig) cometlineMCPJSON {
	out := cometlineMCPJSON{Enabled: cfg.Enabled}
	for _, srv := range cfg.Servers {
		row := cometlineMCPServerJSON{
			ID:             srv.ID,
			Name:           srv.Name,
			Enabled:        srv.Enabled,
			Transport:      string(srv.Transport),
			Command:        srv.Command,
			Args:           append([]string(nil), srv.Args...),
			Env:            copyStringMapGo(srv.Env),
			URL:            srv.URL,
			Headers:        copyStringMapGo(srv.Headers),
			AllowedTools:   append([]string(nil), srv.AllowedTools...),
			Source:         string(srv.Source),
			CatalogID:      srv.CatalogID,
			CatalogVersion: srv.CatalogVersion,
		}
		if srv.OAuth != nil {
			row.OAuth = &cometlineMCPOAuthJSON{
				ClientID:         srv.OAuth.ClientID,
				Scopes:           append([]string(nil), srv.OAuth.Scopes...),
				AuthorizationURL: srv.OAuth.AuthorizationURL,
				TokenURL:         srv.OAuth.TokenURL,
			}
		}
		out.Servers = append(out.Servers, row)
	}
	return out
}

func jsonEqual(a, b any) bool {
	ab, err := json.Marshal(a)
	if err != nil {
		return false
	}
	bb, err := json.Marshal(b)
	if err != nil {
		return false
	}
	return string(ab) == string(bb)
}

// catalogHostKey mirrors frontend dedupe (NormalizeServerURL + host/path).
func catalogHostKey(rawURL string) string {
	return catalog.HostKey(rawURL)
}
