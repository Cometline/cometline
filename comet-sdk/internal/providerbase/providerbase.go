// Package providerbase holds the HTTP, retry, error-classification, logging and
// request-marshalling helpers shared by every concrete cometsdk.Provider
// implementation. The provider packages (anthropic, openai, …) differ only in
// their endpoint, auth header, and wire structs; everything else lives here so
// a change to retry policy or error taxonomy lands in exactly one place.
package providerbase

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	cometsdk "github.com/cometline/comet-sdk"
)

// ClassifyHTTPError maps a non-200 HTTP response to a typed SDK error.
// 401/403 → AuthError, 429 → RateLimitError (honouring Retry-After),
// everything else → ServerError with the provider's error message if present.
func ClassifyHTTPError(providerID string, resp *http.Response, body []byte) error {
	switch resp.StatusCode {
	case http.StatusUnauthorized, http.StatusForbidden:
		return &cometsdk.AuthError{ProviderID: providerID, StatusCode: resp.StatusCode}

	case http.StatusTooManyRequests:
		var d time.Duration
		if ra := resp.Header.Get("Retry-After"); ra != "" {
			if secs, err := strconv.Atoi(ra); err == nil {
				d = time.Duration(secs) * time.Second
			}
		}
		return &cometsdk.RateLimitError{ProviderID: providerID, RetryAfterDelay: d}

	default:
		msg := string(body)
		var apiErr struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if json.Unmarshal(body, &apiErr) == nil && apiErr.Error.Message != "" {
			msg = apiErr.Error.Message
		}
		return &cometsdk.ServerError{
			ProviderID: providerID,
			StatusCode: resp.StatusCode,
			Message:    msg,
		}
	}
}

// IsRetryable reports whether err should be retried: rate limits always, and
// the transient 5xx/529 server errors.
func IsRetryable(err error) bool {
	switch e := err.(type) {
	case *cometsdk.RateLimitError:
		return true
	case *cometsdk.ServerError:
		return e.StatusCode == 500 || e.StatusCode == 502 ||
			e.StatusCode == 503 || e.StatusCode == 529
	}
	return false
}

// MarshalWithOptions marshals base into JSON, then merges any keys from
// overrides[providerKey] underneath it so SDK-managed fields always win over
// caller-supplied provider-specific overrides.
func MarshalWithOptions(base any, overrides map[string]any, providerKey string) ([]byte, error) {
	merged := make(map[string]any)
	if overrides != nil {
		if providerOpts, ok := overrides[providerKey]; ok {
			switch v := providerOpts.(type) {
			case map[string]any:
				for k, val := range v {
					merged[k] = val
				}
			default:
				return nil, fmt.Errorf("%s: Options[%q] must be map[string]any, got %T", providerKey, providerKey, providerOpts)
			}
		}
	}

	structBytes, err := json.Marshal(base)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(structBytes, &merged); err != nil {
		return nil, err
	}

	return json.Marshal(merged)
}

// Endpoint joins baseURL with the provider's request path, tolerating a baseURL
// that already ends in "/v1" (as unified gateways often do). path must start
// with "/" and must not include the "/v1" prefix (e.g. "/messages").
func Endpoint(baseURL, path string) string {
	baseURL = cometsdk.NormaliseBaseURL(baseURL)
	if strings.HasSuffix(baseURL, "/v1") {
		return baseURL + path
	}
	return baseURL + "/v1" + path
}

// Logger returns cfg.Logger tagged with the provider id, or a discard logger
// when the caller passed WithLogger(nil).
func Logger(cfg cometsdk.ProviderConfig, providerID string) *slog.Logger {
	log := cfg.Logger
	if log == nil {
		log = slog.New(noopHandler{})
	}
	return log.With("provider", providerID)
}

// noopHandler is a slog.Handler that discards all log records.
type noopHandler struct{}

func (noopHandler) Enabled(_ context.Context, _ slog.Level) bool  { return false }
func (noopHandler) Handle(_ context.Context, _ slog.Record) error { return nil }
func (noopHandler) WithAttrs(_ []slog.Attr) slog.Handler          { return noopHandler{} }
func (noopHandler) WithGroup(_ string) slog.Handler               { return noopHandler{} }
