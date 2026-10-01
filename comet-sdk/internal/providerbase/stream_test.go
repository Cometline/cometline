package providerbase

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"

	cometsdk "github.com/Cometline/cometline/comet-sdk"
)

const testCapability cometsdk.Capability = "test_feature"

type recordingPolicy struct {
	disabled map[cometsdk.Capability]bool
	marked   []cometsdk.Capability
}

func (p *recordingPolicy) Disabled(c cometsdk.Capability) bool { return p.disabled[c] }
func (p *recordingPolicy) MarkUnsupported(c cometsdk.Capability) {
	p.marked = append(p.marked, c)
}

// featureServer rejects any body that mentions test_feature with an HTTP 400
// (or always, when rejectAll is set) and otherwise answers with an empty SSE
// stream. It records every request body it receives.
type featureServer struct {
	*httptest.Server
	mu        sync.Mutex
	bodies    []string
	rejectAll bool
}

func newFeatureServer(t *testing.T, rejectAll bool) *featureServer {
	t.Helper()
	s := &featureServer{rejectAll: rejectAll}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		s.mu.Lock()
		s.bodies = append(s.bodies, string(raw))
		s.mu.Unlock()
		if s.rejectAll || strings.Contains(string(raw), "test_feature") {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":{"message":"Unsupported parameter: test_feature"}}`))
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {}\n\n"))
	}))
	t.Cleanup(s.Close)
	return s
}

func (s *featureServer) requestBodies() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.bodies...)
}

func (s *featureServer) stream() FallbackStream {
	return FallbackStream{
		MaxRetries: 1,
		Log:        slog.New(slog.DiscardHandler),
		Fallbacks: []CapabilityFallback{{
			Capability: testCapability,
			LogEvent:   "stream.test_feature_fallback",
			Rejected: func(_ *cometsdk.Request, err error) bool {
				se, ok := ClientServerError(err)
				return ok && strings.Contains(se.Message, "test_feature")
			},
		}},
		Send: func(ctx context.Context, _ *cometsdk.Request, disabled DisabledCapabilities) (*http.Response, error) {
			body := `{"test_feature":true}`
			if disabled[testCapability] {
				body = `{}`
			}
			return PostSSE(ctx, s.Client(), SSERequest{ProviderID: "test", ErrPrefix: "test", URL: s.URL, Body: []byte(body)})
		},
	}
}

func TestFallbackStream_RetriesWithoutRejectedCapability(t *testing.T) {
	t.Parallel()

	srv := newFeatureServer(t, false)
	policy := &recordingPolicy{}
	resp, err := srv.stream().Open(context.Background(), &cometsdk.Request{Compatibility: policy})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	_ = resp.Body.Close()

	if got, want := srv.requestBodies(), []string{`{"test_feature":true}`, `{}`}; !slices.Equal(got, want) {
		t.Fatalf("request bodies = %q, want %q", got, want)
	}
	if len(policy.marked) != 1 || policy.marked[0] != testCapability {
		t.Fatalf("marked = %v, want [%s]", policy.marked, testCapability)
	}
}

func TestFallbackStream_SkipsCapabilityAlreadyDisabled(t *testing.T) {
	t.Parallel()

	srv := newFeatureServer(t, false)
	policy := &recordingPolicy{disabled: map[cometsdk.Capability]bool{testCapability: true}}
	resp, err := srv.stream().Open(context.Background(), &cometsdk.Request{Compatibility: policy})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	_ = resp.Body.Close()

	if got, want := srv.requestBodies(), []string{`{}`}; !slices.Equal(got, want) {
		t.Fatalf("request bodies = %q, want %q", got, want)
	}
	if len(policy.marked) != 0 {
		t.Fatalf("marked = %v, want none", policy.marked)
	}
}

func TestFallbackStream_FallsBackOnceThenReturnsError(t *testing.T) {
	t.Parallel()

	srv := newFeatureServer(t, true)
	policy := &recordingPolicy{}
	_, err := srv.stream().Open(context.Background(), &cometsdk.Request{Compatibility: policy})

	var se *cometsdk.ServerError
	if !errors.As(err, &se) || se.StatusCode != http.StatusBadRequest {
		t.Fatalf("err = %v, want HTTP 400 ServerError", err)
	}
	if got := len(srv.requestBodies()); got != 2 {
		t.Fatalf("requests = %d, want 2", got)
	}
	if len(policy.marked) != 1 {
		t.Fatalf("marked = %v, want one capability", policy.marked)
	}
}

func TestFallbackStream_UnrelatedErrorIsNotAFallback(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	t.Cleanup(srv.Close)
	stream := (&featureServer{Server: srv}).stream()
	policy := &recordingPolicy{}

	_, err := stream.Open(context.Background(), &cometsdk.Request{Compatibility: policy})
	var authErr *cometsdk.AuthError
	if !errors.As(err, &authErr) {
		t.Fatalf("err = %v, want AuthError", err)
	}
	if len(policy.marked) != 0 {
		t.Fatalf("marked = %v, want none", policy.marked)
	}
}

func TestPostSSE_SetsHeadersAndClassifiesErrors(t *testing.T) {
	t.Parallel()

	var gotHeader http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeader = r.Header.Clone()
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	t.Cleanup(srv.Close)

	header := http.Header{}
	header.Set("Authorization", "Bearer k")
	_, err := PostSSE(context.Background(), srv.Client(), SSERequest{ProviderID: "p", ErrPrefix: "p", URL: srv.URL, Body: []byte(`{}`), Header: header})

	var rle *cometsdk.RateLimitError
	if !errors.As(err, &rle) || rle.ProviderID != "p" {
		t.Fatalf("err = %v, want RateLimitError for provider p", err)
	}
	for key, want := range map[string]string{
		"Content-Type":  "application/json",
		"Accept":        "text/event-stream",
		"Authorization": "Bearer k",
	} {
		if got := gotHeader.Get(key); got != want {
			t.Errorf("%s = %q, want %q", key, got, want)
		}
	}
}
