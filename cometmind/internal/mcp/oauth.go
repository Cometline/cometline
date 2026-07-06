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

	"github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"golang.org/x/oauth2"
)

const oauthDirName = "mcp-oauth"

// OAuthTokenDir returns ~/.cometmind/mcp-oauth (created if missing).
func OAuthTokenDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".cometmind", oauthDirName)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return dir, nil
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
	if err := saveOAuthToken(serverID, tok); err != nil {
		return err
	}
	invalidateOAuthTokenSource(serverID)
	return nil
}

func saveOAuthToken(serverID string, tok *oauth2.Token) error {
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

func invalidateOAuthTokenSource(serverID string) {
	oauthTokenSources.Delete(strings.TrimSpace(serverID))
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

var oauthTokenSources sync.Map // map[string]*persistingTokenSource

func (h fileOAuthHandler) TokenSource(_ context.Context) (oauth2.TokenSource, error) {
	serverID := strings.TrimSpace(h.serverID)
	if cached, ok := oauthTokenSources.Load(serverID); ok {
		return cached.(*persistingTokenSource), nil
	}
	tok, err := LoadOAuthToken(serverID)
	if err != nil {
		return nil, nil
	}
	// If we have persisted client info, build a refreshing source so expired
	// access tokens are renewed via the stored refresh token + token endpoint.
	if info, infoErr := loadOAuthClientInfo(serverID); infoErr == nil && info != nil {
		cfg := &oauth2.Config{
			ClientID:     info.ClientID,
			ClientSecret: info.ClientSecret,
			Endpoint: oauth2.Endpoint{
				TokenURL:  info.TokenEndpoint,
				AuthStyle: info.AuthStyle,
			},
			Scopes: info.Scopes,
		}
		source := newPersistingTokenSource(serverID, cfg, tok)
		actual, _ := oauthTokenSources.LoadOrStore(serverID, source)
		return actual.(*persistingTokenSource), nil
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
		if refresher, ok := ts.(*persistingTokenSource); ok {
			if _, refreshErr := refresher.ForceRefresh(); refreshErr == nil {
				// Refresh succeeded; returning nil triggers an immediate retry with
				// the freshly persisted token.
				return nil
			}
		} else if _, refreshErr := ts.Token(); refreshErr == nil {
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
	cfg      *oauth2.Config
	base     oauth2.TokenSource
	mu       sync.Mutex
	last     *oauth2.Token
}

func newPersistingTokenSource(serverID string, cfg *oauth2.Config, tok *oauth2.Token) *persistingTokenSource {
	return &persistingTokenSource{
		serverID: serverID,
		cfg:      cfg,
		base:     cfg.TokenSource(oauthClientContext(), tok),
		last:     tok,
	}
}

func oauthClientContext() context.Context {
	return context.WithValue(context.Background(), oauth2.HTTPClient, &http.Client{Timeout: 30 * time.Second})
}

func (p *persistingTokenSource) Token() (*oauth2.Token, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	tok, err := p.base.Token()
	if err != nil {
		return nil, err
	}
	if p.last == nil || tokenChanged(tok, p.last) {
		// Token rotated; persist it (best-effort, do not fail the request).
		_ = saveOAuthToken(p.serverID, tok)
		p.last = tok
	}
	return tok, nil
}

func (p *persistingTokenSource) ForceRefresh() (*oauth2.Token, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	seed, err := LoadOAuthToken(p.serverID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(seed.RefreshToken) == "" {
		return nil, fmt.Errorf("oauth token missing refresh_token")
	}
	expired := *seed
	expired.Expiry = time.Now().Add(-time.Hour)
	p.base = p.cfg.TokenSource(oauthClientContext(), &expired)
	tok, err := p.base.Token()
	if err != nil {
		return nil, err
	}
	_ = saveOAuthToken(p.serverID, tok)
	p.last = tok
	return tok, nil
}

func tokenChanged(a, b *oauth2.Token) bool {
	if a == nil || b == nil {
		return a != b
	}
	return a.AccessToken != b.AccessToken ||
		a.RefreshToken != b.RefreshToken ||
		a.TokenType != b.TokenType ||
		!a.Expiry.Equal(b.Expiry)
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
