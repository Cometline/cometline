package codex

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	cometsdk "github.com/cometline/comet-sdk"
	"github.com/cometline/comet-sdk/internal/providerbase"
	"github.com/cometline/comet-sdk/internal/retry"
)

const (
	defaultBaseURL = "https://chatgpt.com/backend-api/codex"
	providerID     = "codex"
)

type provider struct {
	cfg cometsdk.ProviderConfig
	log *slog.Logger
}

type streamFlags struct {
	disableMaxOutputTokens bool
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
	ch := make(chan cometsdk.Event, 32)
	p.log.DebugContext(ctx, "stream.start", "model", req.Model)

	flags := streamFlags{}
	httpResp, err := p.streamWithRetry(ctx, req, flags)
	if err != nil && req.MaxTokens > 0 && isMaxOutputTokensUnsupportedError(err) {
		p.log.DebugContext(ctx, "stream.max_output_tokens_fallback", "error", err, "model", req.Model)
		flags.disableMaxOutputTokens = true
		httpResp, err = p.streamWithRetry(ctx, req, flags)
	}
	if err != nil {
		p.log.DebugContext(ctx, "stream.failed", "error", err)
		return nil, err
	}

	go parseLoop(ctx, providerID, httpResp.Body, ch, p.log)
	return ch, nil
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
	body, err := toCodexRequest(req, flags.disableMaxOutputTokens)
	if err != nil {
		return nil, fmt.Errorf("codex: marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.cfg.BaseURL+"/responses", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("codex: build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")
	httpReq.Header.Set("Authorization", "Bearer "+token.AccessToken)
	if token.AccountID != "" {
		httpReq.Header.Set("ChatGPT-Account-ID", token.AccountID)
	}

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

func isMaxOutputTokensUnsupportedError(err error) bool {
	se, ok := err.(*cometsdk.ServerError)
	if !ok {
		return false
	}
	if se.StatusCode < 400 || se.StatusCode >= 500 {
		return false
	}
	msg := strings.ToLower(se.Message)
	return strings.Contains(msg, "max_output_tokens") &&
		(strings.Contains(msg, "unsupported") || strings.Contains(msg, "unknown") || strings.Contains(msg, "invalid"))
}

func (p *provider) httpClient() *http.Client {
	client := p.cfg.HTTPClient
	if p.cfg.Timeout > 0 {
		client = &http.Client{Transport: p.cfg.HTTPClient.Transport, Timeout: p.cfg.Timeout}
	}
	return client
}
