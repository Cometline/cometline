package catalog

import (
	"strings"

	mcppkg "github.com/Cometline/cometline/cometmind/internal/mcp"
)

// ServerCatalogFields are the catalog metadata fields carried on MCP server settings.
type ServerCatalogFields struct {
	Source         string
	CatalogID      string
	CatalogVersion string
}

// SyncLockedServer refreshes recipe fields for a catalog-locked server. User-owned
// servers (source "user" or "import") are returned unchanged.
func SyncLockedServer(
	fields ServerCatalogFields,
	name, transport, url string,
	oauthScopes []string,
	allowedTools []string,
) (string, string, string, []string, []string, string, bool) {
	if strings.TrimSpace(fields.Source) != "catalog" {
		return name, transport, url, oauthScopes, allowedTools, fields.CatalogVersion, false
	}
	catalogID := strings.TrimSpace(fields.CatalogID)
	if catalogID == "" {
		return name, transport, url, oauthScopes, allowedTools, fields.CatalogVersion, false
	}
	entry, ok, err := ByID(catalogID)
	if err != nil || !ok {
		return name, transport, url, oauthScopes, allowedTools, fields.CatalogVersion, false
	}
	if strings.TrimSpace(fields.CatalogVersion) == strings.TrimSpace(entry.Version) {
		return name, transport, url, oauthScopes, allowedTools, fields.CatalogVersion, false
	}
	nextURL := mcppkg.NormalizeServerURL(entry.URL)
	nextScopes := oauthScopes
	if entry.AuthKind == AuthOAuth {
		if len(entry.OAuthScopes) > 0 {
			nextScopes = append([]string(nil), entry.OAuthScopes...)
		}
	}
	nextAllowed := allowedTools
	if len(entry.DefaultAllowedTools) > 0 && len(allowedTools) == 0 {
		nextAllowed = append([]string(nil), entry.DefaultAllowedTools...)
	}
	return entry.Name, strings.ToLower(strings.TrimSpace(entry.Transport)), nextURL, nextScopes, nextAllowed, entry.Version, true
}
