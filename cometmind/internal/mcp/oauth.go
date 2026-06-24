package mcp

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/cometline/cometmind/internal/paths"
	"github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"golang.org/x/oauth2"
)

// OAuthTokenDir returns the MCP OAuth token directory under the CometMind data dir.
func OAuthTokenDir() (string, error) {
	return paths.MCPOAuthDir()
}

func oauthTokenPath(serverID string) (string, error) {
	dir, err := OAuthTokenDir()
	if err != nil {
		return "", err
	}
	id := strings.TrimSpace(serverID)
	if id == "" {
		return "", fmt.Errorf("empty MCP server id")
	}
	return filepath.Join(dir, id+".json"), nil
}

// LoadOAuthToken reads a stored OAuth token for one MCP server.
func LoadOAuthToken(serverID string) (*oauth2.Token, error) {
	path, err := oauthTokenPath(serverID)
	if err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var tok oauth2.Token
	if err := json.Unmarshal(raw, &tok); err != nil {
		return nil, fmt.Errorf("parse oauth token: %w", err)
	}
	if strings.TrimSpace(tok.AccessToken) == "" {
		return nil, fmt.Errorf("oauth token missing access_token")
	}
	return &tok, nil
}

// SaveOAuthToken writes an OAuth token for one MCP server (mode 0600).
func SaveOAuthToken(serverID string, tok *oauth2.Token) error {
	if tok == nil || strings.TrimSpace(tok.AccessToken) == "" {
		return fmt.Errorf("empty oauth token")
	}
	path, err := oauthTokenPath(serverID)
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(tok, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

// OAuthConnected reports whether a non-empty token file exists for the server.
func OAuthConnected(serverID string) bool {
	tok, err := LoadOAuthToken(serverID)
	return err == nil && tok != nil && strings.TrimSpace(tok.AccessToken) != ""
}

// fileOAuthHandler implements the go-sdk auth.OAuthHandler for the headless
// runtime connect path. It serves the persisted access token and, when client
// info is available, transparently refreshes it (persisting the rotated token
// back to disk). It never starts an interactive/browser flow: that is owned by
// the explicit "Connect with OAuth" path (PerformInteractiveOAuth).
type fileOAuthHandler struct {
	serverID string
}

func (h fileOAuthHandler) TokenSource(ctx context.Context) (oauth2.TokenSource, error) {
	tok, err := LoadOAuthToken(h.serverID)
	if err != nil {
		return nil, nil
	}
	// If we have persisted client info, build a refreshing source so expired
	// access tokens are renewed via the stored refresh token + token endpoint.
	if info, infoErr := loadOAuthClientInfo(h.serverID); infoErr == nil && info != nil {
		cfg := &oauth2.Config{
			ClientID:     info.ClientID,
			ClientSecret: info.ClientSecret,
			Endpoint: oauth2.Endpoint{
				TokenURL:  info.TokenEndpoint,
				AuthStyle: info.AuthStyle,
			},
			Scopes: info.Scopes,
		}
		clientCtx := context.WithValue(ctx, oauth2.HTTPClient, &http.Client{Timeout: 30 * time.Second})
		base := cfg.TokenSource(clientCtx, tok)
		return &persistingTokenSource{serverID: h.serverID, base: base, last: tok}, nil
	}
	// No client info (e.g. token injected externally): serve it statically.
	return oauth2.StaticTokenSource(tok), nil
}

func (h fileOAuthHandler) Authorize(ctx context.Context, _ *http.Request, resp *http.Response) error {
	if resp != nil && resp.Body != nil {
		resp.Body.Close()
	}
	// A 401 reached Authorize, meaning the bearer token was rejected. Attempt a
	// silent refresh; only if that fails do we surface an actionable error.
	if ts, err := h.TokenSource(ctx); err == nil && ts != nil {
		if _, refreshErr := ts.Token(); refreshErr == nil {
			// Refresh succeeded; returning nil triggers an immediate retry with
			// the freshly persisted token.
			return nil
		}
	}
	return fmt.Errorf("MCP OAuth token for server %q is invalid or expired; re-run Connect with OAuth in Cometline Settings", h.serverID)
}

// persistingTokenSource wraps a refreshing oauth2.TokenSource and writes rotated
// tokens back to disk so subsequent runtime connects reuse the refreshed token.
type persistingTokenSource struct {
	serverID string
	base     oauth2.TokenSource
	mu       sync.Mutex
	last     *oauth2.Token
}

func (p *persistingTokenSource) Token() (*oauth2.Token, error) {
	tok, err := p.base.Token()
	if err != nil {
		return nil, err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.last == nil || tok.AccessToken != p.last.AccessToken {
		// Token rotated; persist it (best-effort, do not fail the request).
		_ = SaveOAuthToken(p.serverID, tok)
		p.last = tok
	}
	return tok, nil
}

func oauthHandlerFor(serverID string, _ *OAuthConfig) auth.OAuthHandler {
	// Wire the OAuth handler whenever a token has been saved for this server,
	// regardless of whether an explicit `oauth` config block is present. With
	// discovery + dynamic client registration the user never has to author an
	// oauth block (e.g. Atlassian), so gating on it would leave the refreshing
	// token source unwired and every request would 401.
	if !OAuthConnected(serverID) {
		return nil
	}
	return fileOAuthHandler{serverID: serverID}
}

// randomState returns a URL-safe random state value for the OAuth flow.
func randomState() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func httpClientWithHeaders(base *http.Client, headers map[string]string, oauth *OAuthConfig, serverID string, injectOAuth bool) *http.Client {
	if base == nil {
		base = http.DefaultClient
	}
	transport := base.Transport
	if transport == nil {
		transport = http.DefaultTransport
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
		Timeout:   base.Timeout,
	}
	if base.Timeout == 0 {
		client.Timeout = 0
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
	for k, v := range t.headers {
		if strings.TrimSpace(v) == "" {
			continue
		}
		req.Header.Set(k, v)
	}
	// Only inject a static bearer token for transports that do not have the
	// go-sdk OAuthHandler wired in (i.e. SSE). For streamable HTTP the handler's
	// refreshing TokenSource sets Authorization with an always-fresh token.
	// We gate on a saved token (not on an explicit oauth config block) so that
	// discovery/DCR-only servers without an oauth block still get the bearer.
	if t.injectOAuth && t.serverID != "" {
		if tok, err := LoadOAuthToken(t.serverID); err == nil && tok != nil {
			tokenType := strings.TrimSpace(tok.TokenType)
			if tokenType == "" {
				tokenType = "Bearer"
			}
			req.Header.Set("Authorization", tokenType+" "+tok.AccessToken)
		}
	}
	return t.base.RoundTrip(req)
}

func streamableTransport(cfg ServerConfig) *mcp.StreamableClientTransport {
	// injectOAuth=false: the OAuthHandler's refreshing TokenSource owns the
	// Authorization header for streamable HTTP.
	client := httpClientWithHeaders(nil, cfg.Headers, cfg.OAuth, cfg.ID, false)
	return &mcp.StreamableClientTransport{
		Endpoint:     cfg.URL,
		HTTPClient:   client,
		OAuthHandler: oauthHandlerFor(cfg.ID, cfg.OAuth),
	}
}

func sseTransport(cfg ServerConfig) *mcp.SSEClientTransport {
	// SSE has no OAuthHandler field in the SDK; inject the bearer token directly.
	return &mcp.SSEClientTransport{
		Endpoint:   cfg.URL,
		HTTPClient: httpClientWithHeaders(nil, cfg.Headers, cfg.OAuth, cfg.ID, true),
	}
}

// TokenExpiry returns the token expiry time when a token file exists.
func TokenExpiry(serverID string) *time.Time {
	tok, err := LoadOAuthToken(serverID)
	if err != nil || tok == nil {
		return nil
	}
	if tok.Expiry.IsZero() {
		return nil
	}
	exp := tok.Expiry
	return &exp
}
