package catalog

import (
	"strings"

	mcppkg "github.com/Cometline/cometline/cometmind/internal/mcp"
)

// MaterializedServer is the settings shape for an installed catalog connector.
type MaterializedServer struct {
	ID           string
	Name         string
	Enabled      bool
	Transport    string
	URL          string
	Env          map[string]string
	Headers      map[string]string
	OAuthScopes  []string
	AllowedTools []string
	Source       string
	CatalogID    string
	CatalogVersion string
}

// Materialize builds a server config from a catalog entry and optional required inputs.
func Materialize(entry Entry, inputs map[string]string) MaterializedServer {
	env := map[string]string{}
	headers := map[string]string{}
	for _, req := range entry.RequiredInputs {
		val := strings.TrimSpace(inputs[req.Key])
		if val == "" {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(req.Kind)) {
		case "header":
			headers[req.Key] = val
		default:
			env[req.Key] = val
		}
	}
	url := mcppkg.NormalizeServerURL(entry.URL)
	out := MaterializedServer{
		ID:             entry.ID,
		Name:           entry.Name,
		Enabled:        true,
		Transport:      "http",
		URL:            url,
		Env:            env,
		Headers:        headers,
		AllowedTools:   append([]string(nil), entry.DefaultAllowedTools...),
		Source:         "catalog",
		CatalogID:      entry.ID,
		CatalogVersion: entry.Version,
	}
	if entry.AuthKind == AuthOAuth && len(entry.OAuthScopes) > 0 {
		out.OAuthScopes = append([]string(nil), entry.OAuthScopes...)
	}
	if t := strings.TrimSpace(entry.Transport); t != "" {
		out.Transport = strings.ToLower(t)
	}
	return out
}
