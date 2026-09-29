package mcp

import (
	"regexp"
	"strings"
)

// TransportKind identifies how to reach an MCP server.
type TransportKind string

const (
	TransportStdio TransportKind = "stdio"
	TransportHTTP  TransportKind = "http"
)

// OAuthConfig holds OAuth client metadata (tokens live outside settings).
type OAuthConfig struct {
	ClientID         string
	Scopes           []string
	AuthorizationURL string
	TokenURL         string
}

// ServerConfig is one configured MCP server entry.
type ServerConfig struct {
	ID           string
	Name         string
	Enabled      bool
	Transport    TransportKind
	Command      string
	Args         []string
	Env          map[string]string
	URL          string
	Headers      map[string]string
	OAuth        *OAuthConfig
	AllowedTools []string
}

// Config is the runtime MCP settings slice.
type Config struct {
	Enabled bool
	Servers []ServerConfig
}

// toolNamePartPattern matches OpenAI-compatible tool name rules (also enforced by DeepSeek).
var toolNamePartPattern = regexp.MustCompile(`[^a-zA-Z0-9_-]+`)

func sanitizeToolNamePart(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "unknown"
	}
	s = toolNamePartPattern.ReplaceAllString(s, "_")
	s = strings.Trim(s, "_")
	if s == "" {
		return "unknown"
	}
	return s
}

// ToolName returns the registry-visible tool name for an MCP tool.
// Names must match ^[a-zA-Z0-9_-]+$ for OpenAI-compatible providers.
func ToolName(serverID, toolName string) string {
	return "mcp_" + sanitizeToolNamePart(serverID) + "_" + sanitizeToolNamePart(toolName)
}
