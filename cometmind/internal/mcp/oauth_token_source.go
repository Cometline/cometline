package mcp

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/oauth2"
)

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
