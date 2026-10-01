package providerbase

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	cometsdk "github.com/Cometline/cometline/comet-sdk"
	"github.com/Cometline/cometline/comet-sdk/internal/retry"
)

// SSERequest describes one streaming POST to a provider endpoint.
type SSERequest struct {
	// ProviderID tags typed errors built from non-200 responses.
	ProviderID string
	// ErrPrefix prefixes request-construction and transport errors, e.g. "codex".
	ErrPrefix string
	URL       string
	Body      []byte
	// Header holds provider headers such as Authorization. Content-Type and
	// Accept are always set for a JSON request with an SSE response.
	Header http.Header
}

// PostSSE sends r and returns the response on HTTP 200, or a typed SDK error
// (see ClassifyHTTPError) for any other status.
func PostSSE(ctx context.Context, client *http.Client, r SSERequest) (*http.Response, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, r.URL, bytes.NewReader(r.Body))
	if err != nil {
		return nil, fmt.Errorf("%s: build request: %w", r.ErrPrefix, err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")
	for key, values := range r.Header {
		httpReq.Header[key] = values
	}

	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("%s: http: %w", r.ErrPrefix, err)
	}
	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		return nil, ClassifyHTTPError(r.ProviderID, resp, body)
	}
	return resp, nil
}

// RetryHTTP runs send under the standard retry policy (see IsRetryable),
// logging every retry and failed attempt.
func RetryHTTP(ctx context.Context, maxRetries int, log *slog.Logger, model string, send func() (*http.Response, error)) (*http.Response, error) {
	attempt := 0
	var httpResp *http.Response
	err := retry.Do(ctx, maxRetries, func() error {
		attempt++
		if attempt > 1 {
			log.DebugContext(ctx, "stream.retry", "attempt", attempt, "model", model)
		}
		r, err := send()
		if err != nil {
			log.DebugContext(ctx, "stream.request_error", "attempt", attempt, "error", err)
			return err
		}
		httpResp = r
		return nil
	}, IsRetryable)
	return httpResp, err
}

// CapabilityFallback is an optional request capability that a provider drops,
// and retries without, when the endpoint explicitly rejects it.
type CapabilityFallback struct {
	Capability cometsdk.Capability
	// LogEvent is the debug message logged when the fallback triggers.
	LogEvent string
	// Rejected reports whether err means the endpoint rejected this
	// capability for req.
	Rejected func(req *cometsdk.Request, err error) bool
}

// DisabledCapabilities records which fallback capabilities are switched off
// for the next request attempt.
type DisabledCapabilities map[cometsdk.Capability]bool

// FallbackStream opens a provider's SSE response, retrying transient failures
// and falling back past optional capabilities the endpoint rejects.
type FallbackStream struct {
	MaxRetries int
	Log        *slog.Logger
	// Fallbacks are checked in order; the first one that is still enabled and
	// matches the error is disabled before the next attempt.
	Fallbacks []CapabilityFallback
	// Send performs one HTTP attempt with the given capabilities disabled and
	// returns the HTTP 200 response.
	Send func(ctx context.Context, req *cometsdk.Request, disabled DisabledCapabilities) (*http.Response, error)
}

// Open sends req until it succeeds or fails with an error no remaining
// fallback can address. Capabilities the caller already marked unsupported
// start disabled; each newly rejected capability is reported back to the
// caller before retrying without it.
func (s FallbackStream) Open(ctx context.Context, req *cometsdk.Request) (*http.Response, error) {
	disabled := make(DisabledCapabilities, len(s.Fallbacks))
	for _, fallback := range s.Fallbacks {
		disabled[fallback.Capability] = req.CapabilityDisabled(fallback.Capability)
	}
	for {
		httpResp, err := RetryHTTP(ctx, s.MaxRetries, s.Log, req.Model, func() (*http.Response, error) {
			return s.Send(ctx, req, disabled)
		})
		if err == nil {
			return httpResp, nil
		}
		fallback, ok := s.rejectedFallback(req, disabled, err)
		if !ok {
			s.Log.DebugContext(ctx, "stream.failed", "error", err)
			return nil, err
		}
		s.Log.DebugContext(ctx, fallback.LogEvent, "error", err, "model", req.Model)
		disabled[fallback.Capability] = true
		req.ReportUnsupportedCapability(fallback.Capability)
	}
}

func (s FallbackStream) rejectedFallback(req *cometsdk.Request, disabled DisabledCapabilities, err error) (CapabilityFallback, bool) {
	for _, fallback := range s.Fallbacks {
		if !disabled[fallback.Capability] && fallback.Rejected(req, err) {
			return fallback, true
		}
	}
	return CapabilityFallback{}, false
}
