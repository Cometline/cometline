// Package codex implements the cometsdk.Provider interface for the ChatGPT Codex Responses API.
package codex

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	refreshURL    = "https://auth.openai.com/oauth/token"
	codexClientID = "app_EMoamEEZ73f0CkXaXp7hrann"
	refreshSkew   = 30 * time.Second
)

type authFile struct {
	AuthMode string     `json:"auth_mode"`
	Tokens   authTokens `json:"tokens"`
}

type authTokens struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	IDToken      string `json:"id_token,omitempty"`
	AccountID    string `json:"account_id,omitempty"`
	LastRefresh  string `json:"last_refresh,omitempty"`
}

type borrowedToken struct {
	AccessToken string
	AccountID   string
}

func borrowCodexToken(ctx context.Context, client *http.Client) (borrowedToken, error) {
	return borrowCodexTokenFromPath(ctx, client, codexAuthPath())
}

func codexAuthPath() string {
	if home := strings.TrimSpace(os.Getenv("CODEX_HOME")); home != "" {
		return filepath.Join(home, "auth.json")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".codex", "auth.json")
	}
	return filepath.Join(home, ".codex", "auth.json")
}

func borrowCodexTokenFromPath(ctx context.Context, client *http.Client, path string) (borrowedToken, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return borrowedToken{}, fmt.Errorf("codex: auth file not found at %s; run `codex login`", path)
		}
		return borrowedToken{}, fmt.Errorf("codex: read auth file: %w", err)
	}

	var auth authFile
	if err := json.Unmarshal(data, &auth); err != nil {
		return borrowedToken{}, fmt.Errorf("codex: parse auth file: %w", err)
	}
	if auth.AuthMode != "chatgpt" {
		return borrowedToken{}, fmt.Errorf("codex: auth file is not a ChatGPT login; run `codex login`")
	}
	if strings.TrimSpace(auth.Tokens.AccessToken) == "" {
		return borrowedToken{}, fmt.Errorf("codex: auth file has no access token; run `codex login`")
	}

	if !tokenExpiresSoon(auth.Tokens.AccessToken, time.Now().Add(refreshSkew)) {
		return borrowedToken{AccessToken: auth.Tokens.AccessToken, AccountID: auth.Tokens.AccountID}, nil
	}
	if strings.TrimSpace(auth.Tokens.RefreshToken) == "" {
		return borrowedToken{}, fmt.Errorf("codex: access token expired and no refresh token is available; run `codex login`")
	}

	refreshed, err := refreshCodexToken(ctx, client, auth.Tokens.RefreshToken)
	if err != nil {
		return borrowedToken{}, err
	}
	auth.Tokens.AccessToken = refreshed.AccessToken
	if refreshed.RefreshToken != "" {
		auth.Tokens.RefreshToken = refreshed.RefreshToken
	}
	if refreshed.IDToken != "" {
		auth.Tokens.IDToken = refreshed.IDToken
	}
	auth.Tokens.LastRefresh = time.Now().Format(time.RFC3339)
	if err := writeAuthFile(path, auth); err != nil {
		return borrowedToken{}, err
	}
	return borrowedToken{AccessToken: auth.Tokens.AccessToken, AccountID: auth.Tokens.AccountID}, nil
}

type refreshResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	IDToken      string `json:"id_token"`
	Error        string `json:"error"`
	ErrorDesc    string `json:"error_description"`
}

func refreshCodexToken(ctx context.Context, client *http.Client, refreshToken string) (refreshResponse, error) {
	body, err := json.Marshal(map[string]string{
		"client_id":     codexClientID,
		"grant_type":    "refresh_token",
		"refresh_token": refreshToken,
	})
	if err != nil {
		return refreshResponse{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, refreshURL, bytes.NewReader(body))
	if err != nil {
		return refreshResponse{}, fmt.Errorf("codex: build refresh request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return refreshResponse{}, fmt.Errorf("codex: refresh token: %w", err)
	}
	defer resp.Body.Close()

	var out refreshResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return refreshResponse{}, fmt.Errorf("codex: parse refresh response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 || out.Error != "" {
		message := out.Error
		if out.ErrorDesc != "" {
			message += ": " + out.ErrorDesc
		}
		if message == "" {
			message = resp.Status
		}
		return refreshResponse{}, fmt.Errorf("codex: refresh failed (%s); run `codex login`", message)
	}
	if out.AccessToken == "" {
		return refreshResponse{}, fmt.Errorf("codex: refresh response did not include an access token")
	}
	return out, nil
}

func writeAuthFile(path string, auth authFile) error {
	data, err := json.MarshalIndent(auth, "", "  ")
	if err != nil {
		return fmt.Errorf("codex: marshal auth file: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o600); err != nil {
		return fmt.Errorf("codex: write refreshed auth file: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("codex: replace refreshed auth file: %w", err)
	}
	return nil
}

func tokenExpiresSoon(token string, threshold time.Time) bool {
	exp, ok := jwtExpiry(token)
	if !ok {
		return false
	}
	return !exp.After(threshold)
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
