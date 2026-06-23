package mcp

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// OAuthCallbackPort is the loopback port cometmind listens on to capture the
// OAuth authorization redirect. It must match the redirect URI registered with
// the authorization server and the value used by the desktop shell.
const OAuthCallbackPort = 1456

// OAuthCallbackPath is the loopback redirect path.
const OAuthCallbackPath = "/mcp/oauth/callback"

// oauthInteractiveTimeout bounds how long we wait for the user to complete the
// browser authorization round-trip.
const oauthInteractiveTimeout = 5 * time.Minute

// RedirectURL returns the full loopback redirect URI.
func RedirectURL() string {
	return fmt.Sprintf("http://localhost:%d%s", OAuthCallbackPort, OAuthCallbackPath)
}

// StartOAuth runs the interactive OAuth flow for one configured MCP server:
// discovery, dynamic client registration, browser authorization (via a loopback
// listener), token exchange, persistence, and finally a reconnect so the new
// token takes effect immediately. It is intended to be invoked from an explicit
// user action ("Connect with OAuth"), not the headless startup path.
func (m *Manager) StartOAuth(ctx context.Context, serverID string) error {
	m.mu.RLock()
	entry, ok := m.servers[serverID]
	cfg := ServerConfig{}
	if ok {
		cfg = entry.cfg
	}
	m.mu.RUnlock()
	if !ok {
		// Fall back to the static config snapshot (server may be disabled and so
		// not present in the live servers map until Start runs).
		for _, s := range m.cfg.Servers {
			if s.ID == serverID {
				cfg = s
				ok = true
				break
			}
		}
	}
	if !ok {
		return fmt.Errorf("unknown MCP server: %s", serverID)
	}
	if strings.TrimSpace(cfg.URL) == "" {
		return fmt.Errorf("MCP server %q has no URL; OAuth requires an http or sse server", serverID)
	}

	manualClientID := ""
	var scopes []string
	if cfg.OAuth != nil {
		manualClientID = strings.TrimSpace(cfg.OAuth.ClientID)
		scopes = cfg.OAuth.Scopes
	}

	opts := OAuthFlowOptions{
		ServerID:       cfg.ID,
		ServerURL:      cfg.URL,
		RedirectURL:    RedirectURL(),
		ClientName:     "Cometline",
		ManualClientID: manualClientID,
		Scopes:         scopes,
		HTTPClient:     &http.Client{Timeout: 30 * time.Second},
	}

	if err := PerformInteractiveOAuth(ctx, opts, loopbackFetcher); err != nil {
		return err
	}

	// Reconnect so the freshly stored token is used right away. Best-effort:
	// surface reconnect failures but the token is already persisted.
	if err := m.Reconnect(ctx, serverID); err != nil {
		return fmt.Errorf("oauth succeeded but reconnect failed: %w", err)
	}
	return nil
}

// loopbackFetcher starts a one-shot HTTP listener on the loopback callback port,
// opens the system browser to authURL, and blocks until the authorization server
// redirects back with a code (or the timeout elapses).
func loopbackFetcher(ctx context.Context, authURL string) (string, string, error) {
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", OAuthCallbackPort))
	if err != nil {
		return "", "", fmt.Errorf("listen on oauth callback port %d: %w", OAuthCallbackPort, err)
	}

	type result struct {
		code  string
		state string
		err   error
	}
	resCh := make(chan result, 1)

	srv := &http.Server{}
	mux := http.NewServeMux()
	mux.HandleFunc(OAuthCallbackPath, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if e := q.Get("error"); e != "" {
			desc := q.Get("error_description")
			writeCallbackPage(w, false, desc)
			resCh <- result{err: fmt.Errorf("authorization error: %s %s", e, desc)}
			return
		}
		code := q.Get("code")
		state := q.Get("state")
		if code == "" {
			writeCallbackPage(w, false, "missing authorization code")
			resCh <- result{err: fmt.Errorf("authorization redirect missing code")}
			return
		}
		writeCallbackPage(w, true, "")
		resCh <- result{code: code, state: state}
	})
	srv.Handler = mux

	go func() { _ = srv.Serve(ln) }()
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	if err := openBrowser(authURL); err != nil {
		// Non-fatal: the user can still paste the URL. Surface it via error only
		// if nothing arrives, but log-friendly behaviour is to continue waiting.
		_ = err
	}

	timer := time.NewTimer(oauthInteractiveTimeout)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return "", "", ctx.Err()
	case <-timer.C:
		return "", "", fmt.Errorf("timed out waiting for OAuth authorization")
	case res := <-resCh:
		if res.err != nil {
			return "", "", res.err
		}
		return res.code, res.state, nil
	}
}

func writeCallbackPage(w http.ResponseWriter, ok bool, detail string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if ok {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("<!doctype html><html><body style=\"font-family:sans-serif;text-align:center;padding:48px\"><h1>Connected</h1><p>You can close this window and return to Cometline.</p></body></html>"))
		return
	}
	w.WriteHeader(http.StatusBadRequest)
	safe := url.QueryEscape(detail)
	_, _ = w.Write([]byte("<!doctype html><html><body style=\"font-family:sans-serif;text-align:center;padding:48px\"><h1>Authorization failed</h1><p>" + safe + "</p></body></html>"))
}

// openBrowser opens the default system browser at the given URL.
func openBrowser(target string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", target).Start()
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", target).Start()
	default:
		return exec.Command("xdg-open", target).Start()
	}
}
