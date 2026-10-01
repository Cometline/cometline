package mcp

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/auth"
	"golang.org/x/oauth2"
)

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
