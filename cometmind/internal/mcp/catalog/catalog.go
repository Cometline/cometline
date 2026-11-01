// Package catalog holds the embedded MCP connector recipes (Connectors P0).
package catalog

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

//go:embed entries.json
var entriesJSON []byte

// AuthKind describes how a catalog connector authenticates.
type AuthKind string

const (
	AuthOAuth  AuthKind = "oauth"
	AuthAPIKey AuthKind = "api_key"
	AuthNone   AuthKind = "none"
)

// RequiredInput declares a user-supplied credential field materialized into env or headers.
type RequiredInput struct {
	Key         string `json:"key"`
	Label       string `json:"label"`
	Kind        string `json:"kind"` // "env" | "header"
	Secret      bool   `json:"secret"`
	Placeholder string `json:"placeholder,omitempty"`
}

// Entry is one catalog connector recipe (no secrets).
type Entry struct {
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

// List returns embedded catalog entries.
func List() ([]Entry, error) {
	var raw []Entry
	if err := json.Unmarshal(entriesJSON, &raw); err != nil {
		return nil, fmt.Errorf("parse catalog: %w", err)
	}
	out := make([]Entry, 0, len(raw))
	for _, e := range raw {
		if strings.TrimSpace(e.ID) == "" {
			continue
		}
		out = append(out, e)
	}
	return out, nil
}

// ByID finds one catalog entry.
func ByID(id string) (Entry, bool, error) {
	id = strings.TrimSpace(id)
	list, err := List()
	if err != nil {
		return Entry{}, false, err
	}
	for _, e := range list {
		if e.ID == id {
			return e, true, nil
		}
	}
	return Entry{}, false, nil
}

// HostKey normalizes an MCP URL for deduplication (host + path, lowercased).
func HostKey(rawURL string) string {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return ""
	}
	u, err := url.Parse(trimmed)
	if err != nil || u.Host == "" {
		return strings.ToLower(trimmed)
	}
	path := strings.TrimSuffix(u.EscapedPath(), "/")
	if path == "" {
		path = "/"
	}
	return strings.ToLower(u.Hostname()) + path
}
