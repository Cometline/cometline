// Package openairesponses implements the cometsdk.Provider interface for the
// OpenAI Responses API using a plain API key. It shares the wire protocol
// implementation with the Codex provider but sends no Codex-specific auth,
// headers, or Responses Lite behavior.
package openairesponses

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	cometsdk "github.com/cometline/comet-sdk"
	"github.com/cometline/comet-sdk/internal/providerbase"
	"github.com/cometline/comet-sdk/internal/responsesproto"
	"github.com/cometline/comet-sdk/internal/retry"
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

type streamFlags struct {
	disableMaxOutputTokens  bool
	disableReasoningSummary bool
	disableEncryptedReplay  bool
}

func capabilityDisabled(req *cometsdk.Request, feature cometsdk.Capability) bool {
	return req.Compatibility != nil && req.Compatibility.Disabled(feature)
}

func markCapabilityUnsupported(req *cometsdk.Request, feature cometsdk.Capability) {
	if req.Compatibility != nil {
		req.Compatibility.MarkUnsupported(feature)
	}
}

// NewOpenAIResponsesProvider creates a Provider for the OpenAI Responses API
// authenticated with a plain API key. id is the provider identifier used in
// events and persisted provider state (e.g. "opencode-go").
func NewOpenAIResponsesProvider(apiKey, id string, opts ...cometsdk.Option) cometsdk.Provider {
	cfg := cometsdk.DefaultProviderConfig()
	cfg.BaseURL = defaultBaseURL
	for _, o := range opts {
		o(&cfg)
	}
	cfg.BaseURL = cometsdk.NormaliseBaseURL(cfg.BaseURL)
	return &provider{
		apiKey: apiKey,
		id:     id,
		cfg:    cfg,
		log:    providerbase.Logger(cfg, id),
	}
}

func (p *provider) ID() string { return p.id }

// Stream sends req to the Responses API and returns a channel of events.
func (p *provider) Stream(ctx context.Context, req *cometsdk.Request) (<-chan cometsdk.Event, error) {
	p.log.DebugContext(ctx, "stream.start", "model", req.Model)
	ch := make(chan cometsdk.Event, 32)

	flags := streamFlags{
		disableMaxOutputTokens:  capabilityDisabled(req, cometsdk.CapabilityMaxOutputTokens),
		disableReasoningSummary: capabilityDisabled(req, cometsdk.CapabilityReasoningSummary),
		disableEncryptedReplay:  capabilityDisabled(req, cometsdk.CapabilityEncryptedReasoningReplay),
	}
	for {
		httpResp, err := p.streamWithRetry(ctx, req, flags)
		if err == nil {
			go responsesproto.ParseLoop(ctx, p.id, req.Model, !capabilityDisabled(req, cometsdk.CapabilityToolInputStream), httpResp.Body, ch, p.log, p.cfg.StreamIdleTimeout)
			return ch, nil
		}
		if req.MaxTokens > 0 && !flags.disableMaxOutputTokens && responsesproto.IsMaxOutputTokensUnsupportedError(err) {
			p.log.DebugContext(ctx, "stream.max_output_tokens_fallback", "error", err, "model", req.Model)
			flags.disableMaxOutputTokens = true
			markCapabilityUnsupported(req, cometsdk.CapabilityMaxOutputTokens)
			continue
		}
		if !flags.disableReasoningSummary && responsesproto.IsReasoningSummaryUnsupportedError(err) {
			p.log.DebugContext(ctx, "stream.reasoning_summary_fallback", "error", err, "model", req.Model)
			flags.disableReasoningSummary = true
			markCapabilityUnsupported(req, cometsdk.CapabilityReasoningSummary)
			continue
		}
		if !flags.disableEncryptedReplay && responsesproto.IsEncryptedReasoningReplayError(err) {
			p.log.DebugContext(ctx, "stream.encrypted_reasoning_replay_fallback", "error", err, "model", req.Model)
			flags.disableEncryptedReplay = true
			markCapabilityUnsupported(req, cometsdk.CapabilityEncryptedReasoningReplay)
			continue
		}
		p.log.DebugContext(ctx, "stream.failed", "error", err)
		return nil, err
	}
}

func (p *provider) streamWithRetry(ctx context.Context, req *cometsdk.Request, flags streamFlags) (*http.Response, error) {
	attempt := 0
	var httpResp *http.Response
	err := retry.Do(ctx, p.cfg.MaxRetries, func() error {
		attempt++
		if attempt > 1 {
			p.log.DebugContext(ctx, "stream.retry", "attempt", attempt, "model", req.Model)
		}
		r, err := p.doRequest(ctx, req, flags)
		if err != nil {
			p.log.DebugContext(ctx, "stream.request_error", "attempt", attempt, "error", err)
			return err
		}
		httpResp = r
		return nil
	}, providerbase.IsRetryable)
	return httpResp, err
}

func (p *provider) doRequest(ctx context.Context, req *cometsdk.Request, flags streamFlags) (*http.Response, error) {
	client := p.httpClient()
	body, err := responsesproto.BuildRequest(req, responsesproto.RequestOptions{
		ProviderKey:              "openai",
		DisableMaxOutputTokens:   flags.disableMaxOutputTokens,
		DisableReasoningSummary:  flags.disableReasoningSummary,
		ReplayEncryptedState:     !flags.disableEncryptedReplay,
		IncludeEncryptedReasoning: true,
	})
	if err != nil {
		return nil, fmt.Errorf("openairesponses: marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		providerbase.Endpoint(p.cfg.BaseURL, "/responses"), bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("openairesponses: build request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("openairesponses: http: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		return nil, providerbase.ClassifyHTTPError(p.id, resp, body)
	}
	return resp, nil
}

func (p *provider) httpClient() *http.Client {
	return cometsdk.StreamingHTTPClient(p.cfg)
}
