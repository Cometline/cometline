package cometsdk

import (
	"net/http"
	"testing"
	"time"
)

func TestDefaultProviderConfigMaxRetries(t *testing.T) {
	t.Parallel()

	if got := DefaultProviderConfig().MaxRetries; got != 5 {
		t.Fatalf("MaxRetries = %d, want 5", got)
	}
}

func TestDefaultProviderConfigStreamIdleTimeout(t *testing.T) {
	if got := DefaultProviderConfig().StreamIdleTimeout; got != 10*time.Minute {
		t.Fatalf("StreamIdleTimeout = %s, want %s", got, 10*time.Minute)
	}
}

func TestStreamingHTTPClientUsesHeaderDeadlineWithoutBodyDeadline(t *testing.T) {
	client := StreamingHTTPClient(DefaultProviderConfig())
	if client.Timeout != 0 {
		t.Fatalf("Timeout = %s, want no body deadline", client.Timeout)
	}
	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("Transport = %T, want *http.Transport", client.Transport)
	}
	if transport.ResponseHeaderTimeout != 30*time.Second {
		t.Fatalf("ResponseHeaderTimeout = %s, want %s", transport.ResponseHeaderTimeout, 30*time.Second)
	}
}

func TestStreamingHTTPClientHonorsExplicitTimeout(t *testing.T) {
	cfg := DefaultProviderConfig()
	WithTimeout(15 * time.Second)(&cfg)

	if got := StreamingHTTPClient(cfg).Timeout; got != 15*time.Second {
		t.Fatalf("Timeout = %s, want %s", got, 15*time.Second)
	}
}

func TestStreamingHTTPClientHonorsExplicitHeaderTimeout(t *testing.T) {
	cfg := DefaultProviderConfig()
	WithResponseHeaderTimeout(5 * time.Second)(&cfg)

	transport := StreamingHTTPClient(cfg).Transport.(*http.Transport)
	if transport.ResponseHeaderTimeout != 5*time.Second {
		t.Fatalf("ResponseHeaderTimeout = %s, want %s", transport.ResponseHeaderTimeout, 5*time.Second)
	}
}
