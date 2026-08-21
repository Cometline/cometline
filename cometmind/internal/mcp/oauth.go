package mcp

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gofrs/flock"
	"github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"golang.org/x/oauth2"
)

const oauthDirName = "mcp-oauth"

const (
	oauthOperationTimeout = 45 * time.Second
	oauthLockRetryDelay   = 50 * time.Millisecond
)

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
	ctx, cancel := context.WithTimeout(context.Background(), oauthOperationTimeout)
	defer cancel()
	if err := withOAuthLock(ctx, serverID, func() error {
		return saveOAuthToken(serverID, tok)
	}); err != nil {
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
	return writePrivateFileAtomic(path, data)
}

func writePrivateFileAtomic(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer tmp.Close()
	defer os.Remove(tmpPath)
	if err := tmp.Chmod(0o600); err != nil {
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		return err
	}
	if err := tmp.Sync(); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}

func withOAuthLock(ctx context.Context, serverID string, fn func() error) (retErr error) {
	path, err := oauthTokenPath(serverID)
	if err != nil {
		return err
	}
	fileLock := flock.New(path+".lock", flock.SetPermissions(0o600))
	locked, err := fileLock.TryLockContext(ctx, oauthLockRetryDelay)
	if err != nil {
		return fmt.Errorf("lock oauth credentials: %w", err)
	}
	if !locked {
		return fmt.Errorf("lock oauth credentials: %w", ctx.Err())
	}
	defer func() {
		if unlockErr := fileLock.Unlock(); retErr == nil && unlockErr != nil {
			retErr = fmt.Errorf("unlock oauth credentials: %w", unlockErr)
		}
	}()
	return fn()
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
		source := newPersistingTokenSource(serverID, tok)
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
			if _, refreshErr := refresher.ForceRefresh(ctx); refreshErr == nil {
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
	if err := ctx.Err(); err != nil {
		return err
	}
	return fmt.Errorf("MCP OAuth token for server %q is invalid or expired; re-run Connect with OAuth in Cometline Settings", h.serverID)
}

// persistingTokenSource wraps a refreshing oauth2.TokenSource and writes rotated
// tokens back to disk so subsequent runtime connects reuse the refreshed token.
type persistingTokenSource struct {
	serverID    string
	refreshLock chan struct{}
	last        *oauth2.Token
}

func newPersistingTokenSource(serverID string, tok *oauth2.Token) *persistingTokenSource {
	source := &persistingTokenSource{
		serverID:    serverID,
		refreshLock: make(chan struct{}, 1),
		last:        tok,
	}
	source.refreshLock <- struct{}{}
	return source
}

func oauthConfigFromClientInfo(info *oauthClientInfo) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     info.ClientID,
		ClientSecret: info.ClientSecret,
		Endpoint: oauth2.Endpoint{
			TokenURL:  info.TokenEndpoint,
			AuthStyle: info.AuthStyle,
		},
		Scopes: info.Scopes,
	}
}

func (p *persistingTokenSource) Token() (*oauth2.Token, error) {
	if err := p.acquire(context.Background()); err != nil {
		return nil, err
	}
	defer p.release()
	if p.last != nil && p.last.Valid() {
		return p.last, nil
	}
	return p.refreshLocked(context.Background(), false)
}

func (p *persistingTokenSource) ForceRefresh(ctx context.Context) (*oauth2.Token, error) {
	if err := p.acquire(ctx); err != nil {
		return nil, err
	}
	defer p.release()
	return p.refreshLocked(ctx, true)
}

func (p *persistingTokenSource) acquire(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-p.refreshLock:
		return nil
	}
}

func (p *persistingTokenSource) release() {
	p.refreshLock <- struct{}{}
}

func (p *persistingTokenSource) refreshLocked(parent context.Context, force bool) (*oauth2.Token, error) {
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithTimeout(parent, oauthOperationTimeout)
	defer cancel()
	var result *oauth2.Token
	err := withOAuthLock(ctx, p.serverID, func() error {
		seed, err := LoadOAuthToken(p.serverID)
		if err != nil {
			return err
		}
		info, err := loadOAuthClientInfo(p.serverID)
		if err != nil {
			return err
		}
		cfg := oauthConfigFromClientInfo(info)

		// Another process or a new interactive grant may already have replaced
		// both credentials while this source was waiting for the lock.
		if seed.Valid() && (tokenChanged(seed, p.last) || !force) {
			p.last = seed
			result = seed
			return nil
		}
		if strings.TrimSpace(seed.RefreshToken) == "" {
			return fmt.Errorf("oauth token missing refresh_token")
		}
		expired := *seed
		expired.Expiry = time.Now().Add(-time.Hour)
		tok, err := refreshOAuthToken(ctx, cfg, &expired, isAtlassianTokenEndpoint(info.TokenEndpoint))
		if err != nil {
			return err
		}
		if err := saveOAuthToken(p.serverID, tok); err != nil {
			return fmt.Errorf("persist refreshed oauth token: %w", err)
		}
		p.last = tok
		result = tok
		return nil
	})
	return result, err
}

func refreshOAuthToken(ctx context.Context, cfg *oauth2.Config, seed *oauth2.Token, retryAmbiguous bool) (*oauth2.Token, error) {
	var lastErr error
	attempts := 1
	if retryAmbiguous {
		attempts = 2
	}
	for attempt := 0; attempt < attempts; attempt++ {
		clientCtx := context.WithValue(ctx, oauth2.HTTPClient, &http.Client{Timeout: 20 * time.Second})
		tok, err := cfg.TokenSource(clientCtx, seed).Token()
		if err == nil {
			return tok, nil
		}
		lastErr = err
		var retrieveErr *oauth2.RetrieveError
		if errors.As(err, &retrieveErr) || attempt == attempts-1 {
			break
		}
		timer := time.NewTimer(250 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
	return nil, lastErr
}

func isAtlassianTokenEndpoint(rawURL string) bool {
	u, err := url.Parse(rawURL)
	return err == nil && strings.EqualFold(u.Scheme, "https") && strings.EqualFold(u.Hostname(), "auth.atlassian.com")
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

func oauthHandlerFor(cfg ServerConfig) auth.OAuthHandler {
	// Wire the OAuth handler whenever a token has been saved for this server,
	// regardless of whether an explicit `oauth` config block is present. With
	// discovery + dynamic client registration the user never has to author an
	// oauth block (e.g. Atlassian), so gating on it would leave the refreshing
	// token source unwired and every request would 401.
	if !OAuthConnected(cfg.ID) || oauthTokenStaleForURL(cfg.ID, cfg.URL) {
		return nil
	}
	return fileOAuthHandler{serverID: cfg.ID}
}

// randomState returns a URL-safe random state value for the OAuth flow.
func randomState() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

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

func sseTransport(cfg ServerConfig) *mcp.SSEClientTransport {
	// SSE has no OAuthHandler field in the SDK; inject the bearer token directly.
	return &mcp.SSEClientTransport{
		Endpoint:   NormalizeServerURL(cfg.URL),
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
