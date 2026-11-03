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
	ClientSecret     string
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

// DenyAllTools is the allow-list sentinel for "expose nothing". An empty
// allow-list still means "expose every tool", including ones discovered later,
// so a real deny-all cannot be stored as [].
const DenyAllTools = "-"

// ToolExposeMode reports how an allow-list should be applied.
// "all" is the empty-list default, "none" is DenyAllTools, and "list" is an
// explicit subset.
func ToolExposeMode(names []string) string {
	if len(names) == 0 {
		return "all"
	}
	if len(names) == 1 && strings.TrimSpace(names[0]) == DenyAllTools {
		return "none"
	}
	return "list"
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
