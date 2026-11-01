package catalog

// PublicEntry is the wire shape for GET /api/v1/mcp/catalog (no secrets).
type PublicEntry struct {
	ID                  string          `json:"id"`
	Version             string          `json:"version"`
	Name                string          `json:"name"`
	Description         string          `json:"description"`
	Transport           string          `json:"transport"`
	URL                 string          `json:"url"`
	AuthKind            AuthKind        `json:"authKind"`
	OAuthScopes         []string        `json:"oauthScopes"`
	DefaultAllowedTools []string        `json:"defaultAllowedTools"`
	RequiredInputs      []RequiredInput `json:"requiredInputs,omitempty"`
}

// PublicList returns catalog entries safe for Settings and agents.
func PublicList() ([]PublicEntry, error) {
	raw, err := List()
	if err != nil {
		return nil, err
	}
	out := make([]PublicEntry, 0, len(raw))
	for _, e := range raw {
		out = append(out, PublicEntry{
			ID:                  e.ID,
			Version:             e.Version,
			Name:                e.Name,
			Description:         e.Description,
			Transport:           e.Transport,
			URL:                 e.URL,
			AuthKind:            e.AuthKind,
			OAuthScopes:         append([]string(nil), e.OAuthScopes...),
			DefaultAllowedTools: append([]string(nil), e.DefaultAllowedTools...),
			RequiredInputs:      append([]RequiredInput(nil), e.RequiredInputs...),
		})
	}
	return out, nil
}
