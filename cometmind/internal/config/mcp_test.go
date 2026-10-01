package config

import "testing"

func TestNormalizeMCPTransport(t *testing.T) {
	tests := []struct {
		raw  string
		want MCPTransport
	}{
		{raw: "http", want: MCPTransportHTTP},
		{raw: "  HTTP \n", want: MCPTransportHTTP},
		{raw: "sse", want: MCPTransportHTTP},
		{raw: "SSE", want: MCPTransportHTTP},
		{raw: "stdio", want: MCPTransportStdio},
		{raw: "", want: MCPTransportStdio},
		{raw: "websocket", want: MCPTransportStdio},
	}
	for _, tt := range tests {
		if got := normalizeMCPTransport(tt.raw); got != tt.want {
			t.Errorf("normalizeMCPTransport(%q) = %q, want %q", tt.raw, got, tt.want)
		}
	}
}

func TestAdaptMCPJSONRewritesSSETransportToHTTP(t *testing.T) {
	got := adaptMCPJSON(cometlineMCPJSON{
		Enabled: true,
		Servers: []cometlineMCPServerJSON{{ID: "remote", Enabled: true, Transport: "sse", URL: "https://example.com/mcp"}},
	})
	if len(got.Servers) != 1 {
		t.Fatalf("servers = %d, want 1", len(got.Servers))
	}
	if got.Servers[0].Transport != MCPTransportHTTP {
		t.Fatalf("transport = %q, want %q", got.Servers[0].Transport, MCPTransportHTTP)
	}
}
