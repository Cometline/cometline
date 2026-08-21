package mcp

import "testing"

func TestNormalizeServerURLAtlassian(t *testing.T) {
	cases := map[string]string{
		"https://mcp.atlassian.com/v1/mcp":                   "https://mcp.atlassian.com/v1/mcp/authv2",
		"https://mcp.atlassian.com/v1/mcp?tenant=acme#tools": "https://mcp.atlassian.com/v1/mcp/authv2?tenant=acme#tools",
		"https://mcp.atlassian.com/v1/sse":                   "https://mcp.atlassian.com/v1/mcp/authv2",
		"https://mcp.atlassian.com/v1/mcp/authv2":            "https://mcp.atlassian.com/v1/mcp/authv2",
		"https://mcp.example.com/v1/mcp":                     "https://mcp.example.com/v1/mcp",
		"":                                                   "",
	}
	for in, want := range cases {
		if got := NormalizeServerURL(in); got != want {
			t.Errorf("NormalizeServerURL(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestValidateRemoteURL(t *testing.T) {
	for _, rawURL := range []string{"", "/mcp", "ftp://example.com/mcp", "https:///mcp"} {
		err := validateRemoteURL("demo", rawURL)
		if err == nil || errorCodeOf(err) != CodeBadURL {
			t.Errorf("validateRemoteURL(%q) = %v, want bad_url", rawURL, err)
		}
	}
	if err := validateRemoteURL("demo", "https://example.com/mcp"); err != nil {
		t.Fatalf("valid URL rejected: %v", err)
	}
}

func TestConnectTimeoutForTransport(t *testing.T) {
	if got := connectTimeoutFor(ServerConfig{Transport: TransportHTTP}); got != handshakeTimeoutHTTP {
		t.Fatalf("http timeout = %s, want %s", got, handshakeTimeoutHTTP)
	}
	if got := connectTimeoutFor(ServerConfig{Transport: TransportStdio}); got != defaultConnectTimeout {
		t.Fatalf("stdio timeout = %s, want %s", got, defaultConnectTimeout)
	}
	if got := listToolsTimeoutFor(ServerConfig{Transport: TransportHTTP}); got != listToolsTimeoutHTTP {
		t.Fatalf("http list-tools timeout = %s, want %s", got, listToolsTimeoutHTTP)
	}
}
