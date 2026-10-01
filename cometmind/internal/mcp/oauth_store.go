package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gofrs/flock"
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
