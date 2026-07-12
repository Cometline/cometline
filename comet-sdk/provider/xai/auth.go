package xai

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	refreshURL  = "https://auth.x.ai/oauth2/token"
	clientID    = "b1a00492-073a-47ea-816f-4c329264a828"
	refreshSkew = 2 * time.Minute
)

type authFile struct {
	AuthMode string     `json:"auth_mode"`
	Tokens   authTokens `json:"tokens"`
}

type authTokens struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresAt    int64  `json:"expires_at,omitempty"`
	LastRefresh  string `json:"last_refresh,omitempty"`
}

type borrowedToken struct {
	AccessToken string
}

var refreshMu sync.Mutex

// BorrowToken reads the local xAI subscription session and refreshes it when
// needed. It is exported as the token source consumed by the shared OpenAI
// compatible client.
func BorrowToken(ctx context.Context, client *http.Client) (string, error) {
	refreshMu.Lock()
	defer refreshMu.Unlock()

	path := authPath()
	unlock, err := lockAuthFile(ctx, path)
	if err != nil {
		return "", err
	}
	defer unlock()

	token, err := borrowTokenFromPath(ctx, client, path)
	if err != nil {
		return "", err
	}
	return token.AccessToken, nil
}

func lockAuthFile(ctx context.Context, path string) (func(), error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("xai: create auth directory: %w", err)
	}
	lockPath := path + ".lock"
	for {
		err := os.Mkdir(lockPath, 0o700)
		if err == nil {
			return func() { _ = os.Remove(lockPath) }, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return nil, fmt.Errorf("xai: acquire auth lock: %w", err)
		}
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("xai: acquire auth lock: %w", ctx.Err())
		case <-time.After(50 * time.Millisecond):
		}
	}
}

func authPath() string {
	if home := strings.TrimSpace(os.Getenv("XAI_HOME")); home != "" {
		return filepath.Join(home, "auth.json")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".cometmind", "xai", "auth.json")
	}
	return filepath.Join(home, ".cometmind", "xai", "auth.json")
}

func borrowTokenFromPath(ctx context.Context, client *http.Client, path string) (borrowedToken, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return borrowedToken{}, fmt.Errorf("xai: subscription session not found at %s; sign in with Grok first", path)
		}
		return borrowedToken{}, fmt.Errorf("xai: read auth file: %w", err)
	}

	var auth authFile
	if err := json.Unmarshal(data, &auth); err != nil {
		return borrowedToken{}, fmt.Errorf("xai: parse auth file: %w", err)
	}
	if auth.AuthMode != "subscription" {
		return borrowedToken{}, fmt.Errorf("xai: auth file is not a Grok subscription session; sign in with Grok first")
	}
	if strings.TrimSpace(auth.Tokens.AccessToken) == "" {
		return borrowedToken{}, fmt.Errorf("xai: auth file has no access token; sign in with Grok first")
	}
	if !tokenExpiresSoon(auth.Tokens, time.Now().Add(refreshSkew)) {
		return borrowedToken{AccessToken: auth.Tokens.AccessToken}, nil
	}
	if strings.TrimSpace(auth.Tokens.RefreshToken) == "" {
		return borrowedToken{}, fmt.Errorf("xai: access token expired and no refresh token is available; sign in with Grok again")
	}

	refreshed, err := refreshToken(ctx, client, auth.Tokens.RefreshToken)
	if err != nil {
		return borrowedToken{}, err
	}
	auth.Tokens.AccessToken = refreshed.AccessToken
	if refreshed.RefreshToken != "" {
		auth.Tokens.RefreshToken = refreshed.RefreshToken
	}
	if refreshed.ExpiresIn > 0 {
		auth.Tokens.ExpiresAt = time.Now().Add(time.Duration(refreshed.ExpiresIn) * time.Second).UnixMilli()
	} else {
		auth.Tokens.ExpiresAt = 0
	}
	auth.Tokens.LastRefresh = time.Now().Format(time.RFC3339)
	if err := writeAuthFile(path, auth); err != nil {
		return borrowedToken{}, err
	}
	return borrowedToken{AccessToken: auth.Tokens.AccessToken}, nil
}

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
	Error        string `json:"error"`
	ErrorDesc    string `json:"error_description"`
}

func refreshToken(ctx context.Context, client *http.Client, refresh string) (tokenResponse, error) {
	form := url.Values{
		"client_id":     []string{clientID},
		"grant_type":    []string{"refresh_token"},
		"refresh_token": []string{refresh},
	}
	body := []byte(form.Encode())
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, refreshURL, bytes.NewReader(body))
	if err != nil {
		return tokenResponse{}, fmt.Errorf("xai: build refresh request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "cometline")

	resp, err := client.Do(req)
	if err != nil {
		return tokenResponse{}, fmt.Errorf("xai: refresh token: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return tokenResponse{}, fmt.Errorf("xai: read refresh response: %w", err)
	}
	var out tokenResponse
	if err := json.Unmarshal(data, &out); err != nil {
		return tokenResponse{}, fmt.Errorf("xai: parse refresh response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 || out.Error != "" {
		message := out.Error
		if out.ErrorDesc != "" {
			message += ": " + out.ErrorDesc
		}
		if message == "" {
			message = resp.Status
		}
		return tokenResponse{}, fmt.Errorf("xai: refresh failed (%s); sign in with Grok again", message)
	}
	if out.AccessToken == "" {
		return tokenResponse{}, fmt.Errorf("xai: refresh response did not include an access token")
	}
	return out, nil
}

func writeAuthFile(path string, auth authFile) error {
	data, err := json.MarshalIndent(auth, "", "  ")
	if err != nil {
		return fmt.Errorf("xai: marshal auth file: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("xai: create auth directory: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o600); err != nil {
		return fmt.Errorf("xai: write refreshed auth file: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("xai: replace refreshed auth file: %w", err)
	}
	return nil
}

func tokenExpiresSoon(tokens authTokens, threshold time.Time) bool {
	if tokens.ExpiresAt > 0 {
		return time.UnixMilli(tokens.ExpiresAt).Before(threshold)
	}
	if exp, ok := jwtExpiry(tokens.AccessToken); ok {
		return exp.Before(threshold)
	}
	return false
}

func jwtExpiry(token string) (time.Time, bool) {
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return time.Time{}, false
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return time.Time{}, false
	}
	var claims struct {
		Exp int64 `json:"exp"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil || claims.Exp == 0 {
		return time.Time{}, false
	}
	return time.Unix(claims.Exp, 0), true
}
