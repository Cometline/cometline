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

// LockedServerRecipe is the MCP server fields a catalog sync may refresh.
type LockedServerRecipe struct {
	Name           string
	Transport      string
	URL            string
	OAuthScopes    []string
	AllowedTools   []string
	CatalogVersion string
}

// SyncLockedServerInput carries provenance plus the current server recipe.
type SyncLockedServerInput struct {
	Fields  ServerCatalogFields
	Current LockedServerRecipe
}

// SyncLockedServerResult is the recipe after sync and whether anything changed.
type SyncLockedServerResult struct {
	Next    LockedServerRecipe
	Changed bool
}

// SyncLockedServer refreshes recipe fields for a catalog-locked server. User-owned
// servers (source user or import) are returned unchanged.
func SyncLockedServer(in SyncLockedServerInput) SyncLockedServerResult {
	current := in.Current
	out := SyncLockedServerResult{Next: current}
	if strings.TrimSpace(in.Fields.Source) != ServerSourceCatalog {
		return out
	}
	catalogID := strings.TrimSpace(in.Fields.CatalogID)
	if catalogID == "" {
		return out
	}
	entry, ok, err := ByID(catalogID)
	if err != nil || !ok {
		return out
	}
	if strings.TrimSpace(in.Fields.CatalogVersion) == strings.TrimSpace(entry.Version) {
		return out
	}
	next := current
	next.Name = entry.Name
	next.Transport = strings.ToLower(strings.TrimSpace(entry.Transport))
	if next.Transport == "" {
		next.Transport = "http"
	}
	next.URL = mcppkg.NormalizeServerURL(entry.URL)
	if entry.AuthKind == AuthOAuth && len(entry.OAuthScopes) > 0 {
		next.OAuthScopes = append([]string(nil), entry.OAuthScopes...)
	}
	if len(entry.DefaultAllowedTools) > 0 && len(current.AllowedTools) == 0 {
		next.AllowedTools = append([]string(nil), entry.DefaultAllowedTools...)
	}
	next.CatalogVersion = entry.Version
	out.Next = next
	out.Changed = true
	return out
}
