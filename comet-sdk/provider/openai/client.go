// Package openai implements the cometsdk.Provider interface for OpenAI's Chat Completions API.
package openai

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
	defaultBaseURL = "https://api.openai.com"
	providerID     = "openai"
)

// provider implements cometsdk.Provider for OpenAI.
type provider struct {
	apiKey string
	cfg    cometsdk.ProviderConfig
	log    *slog.Logger
}

// NewOpenAIProvider creates a Provider for OpenAI's Chat Completions API.
// apiKey is required. Use cometsdk.With* options to override defaults.
func NewOpenAIProvider(apiKey string, opts ...cometsdk.Option) cometsdk.Provider {
	cfg := cometsdk.DefaultProviderConfig()
	cfg.BaseURL = defaultBaseURL
	for _, o := range opts {
		o(&cfg)
	}
	cfg.BaseURL = cometsdk.NormaliseBaseURL(cfg.BaseURL)
	return &provider{
		apiKey: apiKey,
		cfg:    cfg,
		log:    providerbase.Logger(cfg, providerID),
	}
}

func (p *provider) ID() string { return providerID }

type streamFlags struct {
	disableImageContent    bool
	enableReasoningSplit   bool
	useMaxCompletionTokens bool
}

// Stream sends req to the OpenAI Chat Completions API and returns a channel of events.
func (p *provider) Stream(ctx context.Context, req *cometsdk.Request) (<-chan cometsdk.Event, error) {
	ch := make(chan cometsdk.Event, 32)

	p.log.DebugContext(ctx, "stream.start", "model", req.Model)

	flags := streamFlags{enableReasoningSplit: shouldEnableReasoningSplit(req.Model)}
	httpResp, err := p.streamWithRetry(ctx, req, flags)

	// Some OpenAI-compatible gateways (e.g. LiteLLM → Anthropic) reject the
	// non-standard reasoning_split field. Retry once without it; embedded
	// thinking tags in content are still parsed when providers use them.
	if err != nil && isReasoningSplitUnsupportedError(err) {
		p.log.DebugContext(ctx, "stream.reasoning_split_fallback", "error", err, "model", req.Model)
		flags.enableReasoningSplit = false
		httpResp, err = p.streamWithRetry(ctx, req, flags)
	}

	// Newer OpenAI models reject max_tokens and require
	// max_completion_tokens. Retry immediately with the newer field name when
	// the API explicitly asks for it, without changing OpenAI-compatible defaults.
	if err != nil && req.MaxTokens > 0 && isMaxTokensUnsupportedError(err) {
		p.log.DebugContext(ctx, "stream.max_completion_tokens_fallback", "error", err, "model", req.Model)
		flags.useMaxCompletionTokens = true
		httpResp, err = p.streamWithRetry(ctx, req, flags)
	}

	// Reactive image fallback: if the request carried image content and the
	// endpoint rejected it (some OpenAI-compatible models such as DeepSeek
	// don't accept "image_url" parts and return an HTTP 400), retry once with
	// image blocks downgraded to a text placeholder. This avoids maintaining a
	// per-model vision-capability list — we only react when the provider itself
	// tells us it can't read the image.
	if err != nil && requestHasImage(req) && isImageUnsupportedError(err) {
		p.log.DebugContext(ctx, "stream.image_fallback", "error", err, "model", req.Model)
		flags.disableImageContent = true
		httpResp, err = p.streamWithRetry(ctx, req, flags)
	}

	if err != nil {
		p.log.DebugContext(ctx, "stream.failed", "error", err)
		return nil, err
	}

	go parseLoop(ctx, providerID, httpResp.Body, ch, p.log)
	return ch, nil
}

// streamWithRetry runs the request through the standard retry policy. When
// disableImageContent is true, image content blocks are downgraded to a text
// placeholder before sending.
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
	body, err := toOpenAIRequest(req, flags.disableImageContent, flags.enableReasoningSplit, flags.useMaxCompletionTokens)
	if err != nil {
		return nil, fmt.Errorf("openai: marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		providerbase.Endpoint(p.cfg.BaseURL, "/chat/completions"), bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("openai: build request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)

	client := p.cfg.HTTPClient
	if p.cfg.Timeout > 0 {
		client = &http.Client{
			Transport: p.cfg.HTTPClient.Transport,
			Timeout:   p.cfg.Timeout,
		}
	}

	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("openai: http: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		return nil, providerbase.ClassifyHTTPError(providerID, resp, body)
	}

	return resp, nil
}

// requestHasImage reports whether any user message in req carries an image
// content block. The reactive fallback only triggers when there is actually an
// image to downgrade.
func requestHasImage(req *cometsdk.Request) bool {
	for _, m := range req.Messages {
		for _, b := range m.Content {
			if _, ok := b.(cometsdk.ImageBlock); ok {
				return true
			}
		}
	}
	return false
}

// isImageUnsupportedError reports whether err is a 4xx ServerError whose message
// indicates the endpoint rejected image content. OpenAI-compatible gateways
// phrase this differently (e.g. DeepSeek returns "unknown variant `image_url`,
// expected `text`"), so we match on the salient substrings rather than an exact
// message.
func isImageUnsupportedError(err error) bool {
	se, ok := err.(*cometsdk.ServerError)
	if !ok {
		return false
	}
	// Only client-side (4xx) rejections are downgrade candidates; 5xx is a
	// transient server fault that the normal retry policy already handles.
	if se.StatusCode < 400 || se.StatusCode >= 500 {
		return false
	}
	msg := strings.ToLower(se.Message)
	if strings.Contains(msg, "image_url") {
		return true
	}
	// Generic phrasings used by various gateways when a non-vision model is
	// handed image content.
	return strings.Contains(msg, "image") &&
		(strings.Contains(msg, "unsupported") ||
			strings.Contains(msg, "not support") ||
			strings.Contains(msg, "invalid") ||
			strings.Contains(msg, "unknown variant"))
}

// isReasoningSplitUnsupportedError reports whether err is a 4xx ServerError
// whose message indicates the endpoint rejected the reasoning_split field.
func isReasoningSplitUnsupportedError(err error) bool {
	se, ok := err.(*cometsdk.ServerError)
	if !ok {
		return false
	}
	if se.StatusCode < 400 || se.StatusCode >= 500 {
		return false
	}
	msg := strings.ToLower(se.Message)
	return strings.Contains(msg, "reasoning_split")
}

// isMaxTokensUnsupportedError reports whether err is a 4xx ServerError whose
// message says max_tokens is rejected in favour of max_completion_tokens.
func isMaxTokensUnsupportedError(err error) bool {
	se, ok := err.(*cometsdk.ServerError)
	if !ok {
		return false
	}
	if se.StatusCode < 400 || se.StatusCode >= 500 {
		return false
	}
	msg := strings.ToLower(se.Message)
	return strings.Contains(msg, "max_tokens") && strings.Contains(msg, "max_completion_tokens")
}

// shouldEnableReasoningSplit reports whether to send the non-standard
// reasoning_split request field. Most OpenAI-compatible gateways (LiteLLM,
// Azure, Anthropic proxies) reject it; only enable for providers known to use it.
func shouldEnableReasoningSplit(model string) bool {
	m := strings.ToLower(strings.TrimSpace(model))
	return strings.Contains(m, "minimax") || strings.HasPrefix(m, "mimo-")
}
