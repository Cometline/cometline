package codex

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
	defaultBaseURL = "https://chatgpt.com/backend-api/codex"
	providerID     = "codex"
	// GPT-5.6 Luna is currently routed through Responses Lite by Codex CLI.
	// Without this header, the Codex backend can report the model as missing
	// even though it is present in the authenticated model catalog.
	responsesLiteHeader = "x-openai-internal-codex-responses-lite"
)

func addCodexResponseHeaders(header http.Header, token borrowedToken, responsesLite bool) {
	if token.AccountID != "" {
		header.Set("ChatGPT-Account-ID", token.AccountID)
	}
	if token.InstallationID != "" {
		header.Set("x-codex-installation-id", token.InstallationID)
	}
	if responsesLite {
		header.Set(responsesLiteHeader, "true")
	}
}

type provider struct {
	cfg cometsdk.ProviderConfig
	log *slog.Logger
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

// NewCodexProvider creates a Provider that reuses the local Codex CLI ChatGPT session.
func NewCodexProvider(opts ...cometsdk.Option) cometsdk.Provider {
	cfg := cometsdk.DefaultProviderConfig()
	cfg.BaseURL = defaultBaseURL
	for _, o := range opts {
		o(&cfg)
	}
	cfg.BaseURL = cometsdk.NormaliseBaseURL(cfg.BaseURL)
	return &provider{cfg: cfg, log: providerbase.Logger(cfg, providerID)}
}

func (p *provider) ID() string { return providerID }

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
			go parseLoop(ctx, providerID, req.Model, !capabilityDisabled(req, cometsdk.CapabilityToolInputStream), httpResp.Body, ch, p.log, p.cfg.StreamIdleTimeout)
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
	token, err := borrowCodexToken(ctx, client)
	if err != nil {
		return nil, err
	}
	body, err := toCodexRequest(req, flags.disableMaxOutputTokens, flags.disableReasoningSummary, flags.disableEncryptedReplay)
	if err != nil {
		return nil, fmt.Errorf("codex: marshal request: %w", err)
	}
	if req.Model == "gpt-5.6-luna" {
		body, err = addResponsesLiteReasoningContext(body)
		if err != nil {
			return nil, fmt.Errorf("codex: add responses-lite reasoning context: %w", err)
		}
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.cfg.BaseURL+"/responses", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("codex: build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")
	httpReq.Header.Set("Authorization", "Bearer "+token.AccessToken)
	addCodexResponseHeaders(httpReq.Header, token, req.Model == "gpt-5.6-luna")

	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("codex: http: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		return nil, providerbase.ClassifyHTTPError(providerID, resp, body)
	}
	return resp, nil
}

func (p *provider) httpClient() *http.Client {
	return cometsdk.StreamingHTTPClient(p.cfg)
}
