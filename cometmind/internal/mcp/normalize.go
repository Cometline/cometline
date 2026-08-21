package mcp

import (
	"net/url"
	"strings"
)

// hostAdapter rewrites a known remote MCP URL. Unknown hosts are left alone.
type hostAdapter struct {
	match   func(host string) bool
	rewrite func(*url.URL) *url.URL
}

var mcpHostAdapters = []hostAdapter{
	{
		match: func(host string) bool {
			return host == "mcp.atlassian.com" || strings.HasSuffix(host, ".mcp.atlassian.com")
		},
		rewrite: rewriteAtlassianMCPURL,
	},
}

// NormalizeServerURL applies host adapters to a remote MCP URL. Unknown hosts
// are returned trimmed and otherwise unchanged. Empty input stays empty.
func NormalizeServerURL(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	u, err := url.Parse(trimmed)
	if err != nil || u.Host == "" {
		return trimmed
	}
	host := strings.ToLower(u.Hostname())
	for _, adapter := range mcpHostAdapters {
		if adapter.match(host) {
			return adapter.rewrite(u).String()
		}
	}
	return trimmed
}

func rewriteAtlassianMCPURL(u *url.URL) *url.URL {
	out := *u
	path := strings.TrimSuffix(out.Path, "/")
	switch path {
	case "", "/v1/mcp", "/v1/sse", "/sse":
		out.Path = "/v1/mcp/authv2"
	}
	return &out
}
