package mcp

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"golang.org/x/oauth2"
)

// mcpHTTPTransport is a dedicated RoundTripper for remote MCP sessions.
// It is not http.DefaultTransport: streamable HTTP needs several concurrent
// connections to one host (initialize POST, standalone GET SSE,
// notifications/initialized POST). HTTP/2's typical max-concurrent-streams=2
// on Cloudflare fronts starves the initialized POST until the connect
// deadline fires. Client-wide Timeout stays 0 so SSE streams are not killed;
// per-call deadlines live on the request context.
func mcpHTTPTransport() *http.Transport {
	return &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2: false,
		TLSNextProto:      map[string]func(string, *tls.Conn) http.RoundTripper{},
		TLSClientConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
			NextProtos: []string{"http/1.1"},
		},
		MaxIdleConns:          32,
		MaxIdleConnsPerHost:   16,
		MaxConnsPerHost:       16,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		ResponseHeaderTimeout: 0,
	}
}

var mcpSessionCloseTimeout = 5 * time.Second

func httpClientWithHeaders(base *http.Client, headers map[string]string, oauth *OAuthConfig, serverID string, injectOAuth bool) *http.Client {
	var transport http.RoundTripper
	if base != nil && base.Transport != nil {
		transport = base.Transport
	} else {
		transport = mcpHTTPTransport()
	}
	wrapped := &headerTransport{
		base:        transport,
		headers:     headers,
		serverID:    serverID,
		oauth:       oauth,
		injectOAuth: injectOAuth,
	}
	client := &http.Client{
		Transport: wrapped,
		Timeout:   0,
	}
	return client
}

type headerTransport struct {
	base        http.RoundTripper
	headers     map[string]string
	serverID    string
	oauth       *OAuthConfig
	injectOAuth bool
}

func (t *headerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Method == http.MethodDelete {
		ctx, cancel := context.WithTimeout(req.Context(), mcpSessionCloseTimeout)
		defer cancel()
		req = req.Clone(ctx)
	}
	for k, v := range t.headers {
		if strings.TrimSpace(v) == "" {
			continue
		}
		req.Header.Set(k, v)
	}
	// Inject through the persisted refreshing token source for transports that
	// do not have the go-sdk OAuthHandler wired in (i.e. SSE). For streamable
	// HTTP the handler owns Authorization and refresh retries.
	// We gate on a saved token (not on an explicit oauth config block) so that
	// discovery/DCR-only servers without an oauth block still get the bearer.
	var tokenSource oauth2.TokenSource
	if t.injectOAuth && t.serverID != "" {
		var err error
		tokenSource, err = (fileOAuthHandler{serverID: t.serverID}).TokenSource(req.Context())
		if err != nil {
			return nil, newConnectError(CodeAuthExpired, "The saved sign-in could not be refreshed. Click Connect with OAuth and sign in again.", err)
		}
		if tokenSource != nil {
			tok, err := tokenSource.Token()
			if err != nil {
				return nil, newConnectError(CodeAuthExpired, "The saved sign-in could not be refreshed. Click Connect with OAuth and sign in again.", err)
			}
			setBearerToken(req, tok)
		}
	}
	resp, err := t.base.RoundTrip(req)
	if err != nil || resp == nil || resp.StatusCode != http.StatusUnauthorized || tokenSource == nil {
		return resp, err
	}
	refresher, ok := tokenSource.(*persistingTokenSource)
	if !ok {
		return resp, nil
	}
	tok, refreshErr := refresher.ForceRefresh(req.Context())
	if refreshErr != nil {
		resp.Body.Close()
		return nil, newConnectError(CodeAuthExpired, "The saved sign-in was rejected. Click Connect with OAuth and sign in again.", refreshErr)
	}
	retry := req.Clone(req.Context())
	if req.Body != nil && req.GetBody != nil {
		retry.Body, err = req.GetBody()
		if err != nil {
			resp.Body.Close()
			return nil, err
		}
	} else if req.Body != nil && req.Body != http.NoBody {
		return resp, nil
	}
	resp.Body.Close()
	setBearerToken(retry, tok)
	return t.base.RoundTrip(retry)
}

func setBearerToken(req *http.Request, tok *oauth2.Token) {
	if tok == nil {
		return
	}
	tokenType := strings.TrimSpace(tok.TokenType)
	if tokenType == "" {
		tokenType = "Bearer"
	}
	req.Header.Set("Authorization", tokenType+" "+tok.AccessToken)
}

func streamableTransport(cfg ServerConfig) *mcp.StreamableClientTransport {
	// injectOAuth=false: the OAuthHandler's refreshing TokenSource owns the
	// Authorization header for streamable HTTP.
	client := httpClientWithHeaders(nil, cfg.Headers, cfg.OAuth, cfg.ID, false)
	return &mcp.StreamableClientTransport{
		Endpoint:     NormalizeServerURL(cfg.URL),
		HTTPClient:   client,
		OAuthHandler: oauthHandlerFor(cfg),
		// SDK treats 0 as default 5; this bounds post-session SSE reconnects,
		// not the initialize handshake budget.
		MaxRetries: 2,
	}
}
