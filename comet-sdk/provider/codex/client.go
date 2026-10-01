package codex

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

// New creates a Provider that reuses the local Codex CLI ChatGPT session.
func New(opts ...cometsdk.Option) cometsdk.Provider {
	cfg := cometsdk.DefaultProviderConfig()
	cfg.BaseURL = defaultBaseURL
	for _, o := range opts {
		o(&cfg)
	}
	cfg.BaseURL = cometsdk.NormalizeBaseURL(cfg.BaseURL)
	return &provider{cfg: cfg, log: providerbase.Logger(cfg, providerID)}
}

// NewCodexProvider creates a Provider that reuses the local Codex CLI ChatGPT session.
//
// Deprecated: use New.
func NewCodexProvider(opts ...cometsdk.Option) cometsdk.Provider {
	return New(opts...)
}

func (p *provider) ID() string { return providerID }

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
	go responsesproto.ParseLoop(ctx, providerID, req.Model, !req.CapabilityDisabled(cometsdk.CapabilityToolInputStream), httpResp.Body, ch, p.log, p.cfg.StreamIdleTimeout)
	return ch, nil
}

func (p *provider) doRequest(ctx context.Context, req *cometsdk.Request, disabled providerbase.DisabledCapabilities) (*http.Response, error) {
	client := p.httpClient()
	token, err := borrowCodexToken(ctx, client)
	if err != nil {
		return nil, err
	}
	body, err := toCodexRequest(req,
		disabled[cometsdk.CapabilityMaxOutputTokens],
		disabled[cometsdk.CapabilityReasoningSummary],
		disabled[cometsdk.CapabilityEncryptedReasoningReplay],
	)
	if err != nil {
		return nil, fmt.Errorf("codex: marshal request: %w", err)
	}
	if req.Model == "gpt-5.6-luna" {
		body, err = addResponsesLiteReasoningContext(body)
		if err != nil {
			return nil, fmt.Errorf("codex: add responses-lite reasoning context: %w", err)
		}
	}

	header := http.Header{}
	header.Set("Authorization", "Bearer "+token.AccessToken)
	addCodexResponseHeaders(header, token, req.Model == "gpt-5.6-luna")
	return providerbase.PostSSE(ctx, client, providerbase.SSERequest{
		ProviderID: providerID,
		ErrPrefix:  "codex",
		URL:        p.cfg.BaseURL + "/responses",
		Body:       body,
		Header:     header,
	})
}

func (p *provider) httpClient() *http.Client {
	return cometsdk.StreamingHTTPClient(p.cfg)
}
