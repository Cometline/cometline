package mcp

import (
	"context"
	"net/http"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestMCPHTTPTransportDisablesHTTP2(t *testing.T) {
	tr := mcpHTTPTransport()
	if tr.ForceAttemptHTTP2 {
		t.Fatal("ForceAttemptHTTP2 = true, want false")
	}
	if tr.TLSClientConfig == nil || len(tr.TLSClientConfig.NextProtos) != 1 || tr.TLSClientConfig.NextProtos[0] != "http/1.1" {
		t.Fatalf("TLS NextProtos = %v, want [http/1.1]", tr.TLSClientConfig)
	}
	if tr.MaxIdleConnsPerHost < 16 {
		t.Fatalf("MaxIdleConnsPerHost = %d, want >= 16", tr.MaxIdleConnsPerHost)
	}
	if tr.DialContext == nil {
		t.Fatal("DialContext is nil, want a 30s net.Dialer")
	}
}

func TestHTTPClientWithHeadersHasNoClientTimeout(t *testing.T) {
	client := httpClientWithHeaders(nil, nil, nil, "demo", false)
	if client.Timeout != 0 {
		t.Fatalf("Timeout = %s, want 0 (SSE / streamable must not use a client-wide deadline)", client.Timeout)
	}
	if _, ok := client.Transport.(*headerTransport); !ok {
		t.Fatalf("Transport type = %T, want *headerTransport", client.Transport)
	}
}

func TestStreamableTransportUsesNormalizedEndpoint(t *testing.T) {
	got := streamableTransport(ServerConfig{
		ID:        "atlassian",
		Transport: TransportHTTP,
		URL:       "https://mcp.atlassian.com/v1/mcp",
	})
	if got.Endpoint != "https://mcp.atlassian.com/v1/mcp/authv2" {
		t.Fatalf("Endpoint = %q", got.Endpoint)
	}
	if got.HTTPClient == nil || got.HTTPClient.Timeout != 0 {
		t.Fatal("expected dedicated HTTP client with Timeout=0")
	}
	if got.MaxRetries != 2 {
		t.Fatalf("MaxRetries = %d, want 2 (SDK treats 0 as default 5 SSE reconnects)", got.MaxRetries)
	}
	_ = http.StatusOK
}

func TestHeaderTransportBoundsSessionDelete(t *testing.T) {
	previous := mcpSessionCloseTimeout
	mcpSessionCloseTimeout = 20 * time.Millisecond
	t.Cleanup(func() { mcpSessionCloseTimeout = previous })

	transport := &headerTransport{base: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		<-req.Context().Done()
		return nil, req.Context().Err()
	})}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodDelete, "https://mcp.example.com", nil)
	if err != nil {
		t.Fatal(err)
	}
	started := time.Now()
	if _, err := transport.RoundTrip(req); err == nil {
		t.Fatal("DELETE succeeded, want deadline error")
	}
	if elapsed := time.Since(started); elapsed > 250*time.Millisecond {
		t.Fatalf("DELETE took %s, want bounded cleanup", elapsed)
	}
}
