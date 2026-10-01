// Package openairesponses implements the cometsdk.Provider interface for the
// OpenAI Responses API using a plain API key. It shares the wire protocol
// implementation with the Codex provider but sends no Codex-specific auth,
// headers, or Responses Lite behavior.
package openairesponses

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	cometsdk "github.com/Cometline/cometline/comet-sdk"
	"github.com/Cometline/cometline/comet-sdk/internal/providerbase"
	"github.com/Cometline/cometline/comet-sdk/internal/responsesproto"
)

const (
	defaultBaseURL = "https://api.openai.com/v1"
)

// provider implements cometsdk.Provider for the Responses API.
type provider struct {
	apiKey string
	id     string
	cfg    cometsdk.ProviderConfig
	log    *slog.Logger
}

// New creates a Provider for the OpenAI Responses API authenticated with a
// plain API key. id is the provider identifier used in events and persisted
// provider state (e.g. "opencode-go").
func New(apiKey, id string, opts ...cometsdk.Option) cometsdk.Provider {
	cfg := cometsdk.DefaultProviderConfig()
	cfg.BaseURL = defaultBaseURL
	for _, o := range opts {
		o(&cfg)
	}
	cfg.BaseURL = cometsdk.NormalizeBaseURL(cfg.BaseURL)
	return &provider{
		apiKey: apiKey,
		id:     id,
		cfg:    cfg,
		log:    providerbase.Logger(cfg, id),
	}
}

// NewOpenAIResponsesProvider creates a Provider for the OpenAI Responses API.
//
// Deprecated: use New.
func NewOpenAIResponsesProvider(apiKey, id string, opts ...cometsdk.Option) cometsdk.Provider {
	return New(apiKey, id, opts...)
}

func (p *provider) ID() string { return p.id }

// Stream sends req to the Responses API and returns a channel of events.
func (p *provider) Stream(ctx context.Context, req *cometsdk.Request) (<-chan cometsdk.Event, error) {
	p.log.DebugContext(ctx, "stream.start", "model", req.Model)
	stream := providerbase.FallbackStream{
		MaxRetries: p.cfg.MaxRetries,
		Log:        p.log,
		Fallbacks:  responsesproto.CapabilityFallbacks(),
		Send:       p.doRequest,
	}
	httpResp, err := stream.Open(ctx, req)
	if err != nil {
		return nil, err
	}
	ch := make(chan cometsdk.Event, 32)
	go responsesproto.ParseLoop(ctx, p.id, req.Model, !req.CapabilityDisabled(cometsdk.CapabilityToolInputStream), httpResp.Body, ch, p.log, p.cfg.StreamIdleTimeout)
	return ch, nil
}

func (p *provider) doRequest(ctx context.Context, req *cometsdk.Request, disabled providerbase.DisabledCapabilities) (*http.Response, error) {
	body, err := responsesproto.BuildRequest(req, responsesproto.RequestOptions{
		ProviderKey:               "openai",
		DisableMaxOutputTokens:    disabled[cometsdk.CapabilityMaxOutputTokens],
		DisableReasoningSummary:   disabled[cometsdk.CapabilityReasoningSummary],
		ReplayEncryptedState:      !disabled[cometsdk.CapabilityEncryptedReasoningReplay],
		IncludeEncryptedReasoning: true,
	})
	if err != nil {
		return nil, fmt.Errorf("openairesponses: marshal request: %w", err)
	}

	header := http.Header{}
	header.Set("Authorization", "Bearer "+p.apiKey)
	return providerbase.PostSSE(ctx, p.httpClient(), providerbase.SSERequest{
		ProviderID: p.id,
		ErrPrefix:  "openairesponses",
		URL:        providerbase.Endpoint(p.cfg.BaseURL, "/responses"),
		Body:       body,
		Header:     header,
	})
}

func (p *provider) httpClient() *http.Client {
	return cometsdk.StreamingHTTPClient(p.cfg)
}
