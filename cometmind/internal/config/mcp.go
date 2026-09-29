package config

import (
	"strings"

	mcppkg "github.com/cometline/cometmind/internal/mcp"
)

// MCPTransport identifies how to reach an MCP server.
type MCPTransport string

const (
	MCPTransportStdio MCPTransport = "stdio"
	MCPTransportHTTP  MCPTransport = "http"
)

// MCPOAuthConfig holds OAuth client metadata (tokens stored separately).
type MCPOAuthConfig struct {
	ClientID         string   `mapstructure:"client_id" json:"clientId"`
	Scopes           []string `mapstructure:"scopes" json:"scopes"`
	AuthorizationURL string   `mapstructure:"authorization_url" json:"authorizationUrl"`
	TokenURL         string   `mapstructure:"token_url" json:"tokenUrl"`
}

// MCPServerConfig is one configured MCP server.
type MCPServerConfig struct {
	ID           string            `mapstructure:"id" json:"id"`
	Name         string            `mapstructure:"name" json:"name"`
	Enabled      bool              `mapstructure:"enabled" json:"enabled"`
	Transport    MCPTransport      `mapstructure:"transport" json:"transport"`
	Command      string            `mapstructure:"command" json:"command"`
	Args         []string          `mapstructure:"args" json:"args"`
	Env          map[string]string `mapstructure:"env" json:"env"`
	URL          string            `mapstructure:"url" json:"url"`
	Headers      map[string]string `mapstructure:"headers" json:"headers"`
	OAuth        *MCPOAuthConfig   `mapstructure:"oauth" json:"oauth"`
	AllowedTools []string          `mapstructure:"allowed_tools" json:"allowedTools"`
}

// MCPConfig groups MCP client settings.
type MCPConfig struct {
	Enabled bool              `mapstructure:"enabled" json:"enabled"`
	Servers []MCPServerConfig `mapstructure:"servers" json:"servers"`
}

// MCPSettings converts config to runtime MCP settings.
func (c *Config) MCPSettings() mcppkg.Config {
	out := mcppkg.Config{}
	if c == nil {
		return out
	}
	out.Enabled = c.MCP.Enabled
	for _, s := range c.MCP.Servers {
		entry := mcppkg.ServerConfig{
			ID:           strings.TrimSpace(s.ID),
			Name:         strings.TrimSpace(s.Name),
			Enabled:      s.Enabled,
			Transport:    mcppkg.TransportKind(strings.TrimSpace(string(s.Transport))),
			Command:      strings.TrimSpace(s.Command),
			Args:         append([]string(nil), s.Args...),
			Env:          copyStringMapConfig(s.Env),
			URL:          strings.TrimSpace(s.URL),
			Headers:      copyStringMapConfig(s.Headers),
			AllowedTools: append([]string(nil), s.AllowedTools...),
		}
		if s.OAuth != nil {
			entry.OAuth = &mcppkg.OAuthConfig{
				ClientID:         strings.TrimSpace(s.OAuth.ClientID),
				Scopes:           append([]string(nil), s.OAuth.Scopes...),
				AuthorizationURL: strings.TrimSpace(s.OAuth.AuthorizationURL),
				TokenURL:         strings.TrimSpace(s.OAuth.TokenURL),
			}
		}
		out.Servers = append(out.Servers, entry)
	}
	return out
}

func copyStringMapConfig(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
