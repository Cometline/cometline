package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/oauthex"
	"golang.org/x/oauth2"
)

// AuthCodeFetcher is invoked during interactive OAuth to direct the user to the
// authorization URL (e.g. open a browser) and return the authorization code and
// state captured from the redirect.
//
// Implementations own the redirect listener (loopback HTTP server). The returned
// state MUST be the state echoed back by the authorization server so the caller
// can verify it against the value embedded in authURL.
type AuthCodeFetcher func(ctx context.Context, authURL string) (code string, state string, err error)

// OAuthFlowOptions configures an interactive OAuth connection attempt.
type OAuthFlowOptions struct {
	// ServerID is the MCP server identifier; used for persistence paths.
	ServerID string
	// ServerURL is the MCP endpoint (canonical resource URI) to authorize against.
	ServerURL string
	// RedirectURL is the loopback redirect URI registered with the auth server.
	RedirectURL string
	// ClientName is the human-readable client name presented during DCR.
	ClientName string
	// ManualClientID, when set, skips Dynamic Client Registration and uses this
	// preregistered public client id instead (power-user / fallback path).
	ManualClientID string
	// Scopes optionally overrides the scopes requested. When empty, scopes are
	// taken from discovery (PRM scopes_supported).
	Scopes []string
	// HTTPClient is used for all discovery/registration/exchange requests.
	HTTPClient *http.Client
}

// PerformInteractiveOAuth runs the full Authorization Code + PKCE flow against an
// MCP server that advertises OAuth via Protected Resource Metadata (RFC 9728),
// Authorization Server Metadata (RFC 8414), and Dynamic Client Registration
// (RFC 7591). On success it persists both the access/refresh token and the
// client info needed for headless refresh, then returns.
//
// This is the interactive ("Connect with OAuth") path. It must only be called
// from a context where the fetcher can complete a browser round-trip.
func PerformInteractiveOAuth(ctx context.Context, opts OAuthFlowOptions, fetch AuthCodeFetcher) error {
	if strings.TrimSpace(opts.ServerID) == "" {
		return fmt.Errorf("server id is required")
	}
	if strings.TrimSpace(opts.ServerURL) == "" {
		return fmt.Errorf("server url is required")
	}
	if strings.TrimSpace(opts.RedirectURL) == "" {
		return fmt.Errorf("redirect url is required")
	}
	if fetch == nil {
		return fmt.Errorf("auth code fetcher is required")
	}
	httpClient := opts.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}

	// 1. Discover protected resource metadata for the MCP endpoint.
	prm, err := discoverProtectedResource(ctx, opts.ServerURL, httpClient)
	if err != nil {
		return fmt.Errorf("protected resource discovery failed: %w", err)
	}
	if len(prm.AuthorizationServers) == 0 {
		return fmt.Errorf("protected resource metadata advertises no authorization servers")
	}

	// 2. Discover authorization server metadata.
	//
	// We intentionally do NOT use auth.GetAuthServerMetadata here: it enforces a
	// strict RFC 8414 issuer match (metadata.issuer == requested URL). Some real
	// deployments (e.g. Atlassian's Cloudflare-fronted authorization server, which
	// returns issuer "https://cf.mcp.atlassian.com" for "https://mcp.atlassian.com")
	// fail that check even though the metadata is otherwise valid. discoverAuthServerMetadata
	// tolerates the mismatch while still requiring HTTPS endpoints and PKCE.
	asm, err := discoverAuthServerMetadata(ctx, prm.AuthorizationServers[0], httpClient)
	if err != nil {
		return fmt.Errorf("authorization server discovery failed: %w", err)
	}
	if asm == nil {
		// Fallback to predefined endpoints (2025-03-26 spec).
		base := strings.TrimRight(prm.AuthorizationServers[0], "/")
		asm = &oauthex.AuthServerMeta{
			Issuer:                prm.AuthorizationServers[0],
			AuthorizationEndpoint: base + "/authorize",
			TokenEndpoint:         base + "/token",
			RegistrationEndpoint:  base + "/register",
		}
	}
	if strings.TrimSpace(asm.AuthorizationEndpoint) == "" || strings.TrimSpace(asm.TokenEndpoint) == "" {
		return fmt.Errorf("authorization server metadata missing authorization or token endpoint")
	}

	// 3. Resolve the client identity (DCR or manual/preregistered).
	clientID := strings.TrimSpace(opts.ManualClientID)
	clientSecret := ""
	authStyle := oauth2.AuthStyleInParams
	if clientID == "" {
		if strings.TrimSpace(asm.RegistrationEndpoint) == "" {
			return fmt.Errorf("authorization server does not support dynamic client registration and no client id was provided")
		}
		clientName := strings.TrimSpace(opts.ClientName)
		if clientName == "" {
			clientName = "Cometline"
		}
		regMeta := &oauthex.ClientRegistrationMetadata{
			RedirectURIs:            []string{opts.RedirectURL},
			ClientName:              clientName,
			GrantTypes:              []string{"authorization_code", "refresh_token"},
			ResponseTypes:           []string{"code"},
			TokenEndpointAuthMethod: "none",
		}
		reg, regErr := oauthex.RegisterClient(ctx, asm.RegistrationEndpoint, regMeta, httpClient)
		if regErr != nil {
			return fmt.Errorf("dynamic client registration failed: %w", regErr)
		}
		clientID = reg.ClientID
		clientSecret = reg.ClientSecret
		authStyle = authStyleForMethod(reg.TokenEndpointAuthMethod)
	}
	if clientID == "" {
		return fmt.Errorf("no client id resolved after registration")
	}

	// 4. Resolve scopes: explicit override > discovery.
	scopes := opts.Scopes
	if len(scopes) == 0 {
		scopes = prm.ScopesSupported
	}

	cfg := &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Endpoint: oauth2.Endpoint{
			AuthURL:   asm.AuthorizationEndpoint,
			TokenURL:  asm.TokenEndpoint,
			AuthStyle: authStyle,
		},
		RedirectURL: opts.RedirectURL,
		Scopes:      scopes,
	}

	// 5. Build the authorization URL (PKCE S256 + RFC 8707 resource indicator).
	verifier := oauth2.GenerateVerifier()
	state, err := randomState()
	if err != nil {
		return fmt.Errorf("generate state: %w", err)
	}
	authURL := cfg.AuthCodeURL(state,
		oauth2.S256ChallengeOption(verifier),
		oauth2.SetAuthURLParam("resource", prm.Resource),
	)

	// 6. Run the interactive fetch (browser + loopback capture).
	code, gotState, err := fetch(ctx, authURL)
	if err != nil {
		return fmt.Errorf("authorization fetch failed: %w", err)
	}
	if gotState != state {
		return fmt.Errorf("oauth state mismatch")
	}
	if strings.TrimSpace(code) == "" {
		return fmt.Errorf("authorization server returned an empty code")
	}

	// 7. Exchange the code for tokens.
	clientCtx := context.WithValue(ctx, oauth2.HTTPClient, httpClient)
	tok, err := cfg.Exchange(clientCtx, code,
		oauth2.VerifierOption(verifier),
		oauth2.SetAuthURLParam("resource", prm.Resource),
	)
	if err != nil {
		return fmt.Errorf("token exchange failed: %w", err)
	}

	// 8. Persist client info (for headless refresh) then the token.
	info := &oauthClientInfo{
		AuthorizationEndpoint: asm.AuthorizationEndpoint,
		TokenEndpoint:         asm.TokenEndpoint,
		ClientID:              clientID,
		ClientSecret:          clientSecret,
		Scopes:                scopes,
		Resource:              prm.Resource,
		AuthStyle:             authStyle,
	}
	if err := saveOAuthClientInfo(opts.ServerID, info); err != nil {
		return fmt.Errorf("persist oauth client info: %w", err)
	}
	if err := SaveOAuthToken(opts.ServerID, tok); err != nil {
		return fmt.Errorf("persist oauth token: %w", err)
	}
	return nil
}

// discoverAuthServerMetadata fetches authorization server metadata for an issuer
// URL, trying the OAuth and OIDC well-known locations (with and without path
// insertion). Unlike the strict SDK helper it tolerates an issuer-field mismatch
// (some providers front their AS behind a different host than the advertised
// issuer), but it still requires the response to carry usable HTTPS endpoints.
// Returns (nil, nil) when no metadata document is found so the caller can fall
// back to predefined endpoints.
func discoverAuthServerMetadata(ctx context.Context, issuerURL string, httpClient *http.Client) (*oauthex.AuthServerMeta, error) {
	for _, metaURL := range authServerMetadataURLs(issuerURL) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, metaURL, nil)
		if err != nil {
			continue
		}
		req.Header.Set("Accept", "application/json")
		resp, err := httpClient.Do(req)
		if err != nil {
			continue
		}
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 || readErr != nil {
			continue
		}
		var asm oauthex.AuthServerMeta
		if err := json.Unmarshal(body, &asm); err != nil {
			continue
		}
		if strings.TrimSpace(asm.AuthorizationEndpoint) == "" || strings.TrimSpace(asm.TokenEndpoint) == "" {
			continue
		}
		return &asm, nil
	}
	return nil, nil
}

// authServerMetadataURLs returns the candidate well-known metadata URLs for an
// issuer, mirroring the MCP spec discovery order.
func authServerMetadataURLs(issuerURL string) []string {
	u, err := url.Parse(issuerURL)
	if err != nil {
		return nil
	}
	var urls []string
	if u.Path == "" || u.Path == "/" {
		base := *u
		base.Path = "/.well-known/oauth-authorization-server"
		urls = append(urls, base.String())
		base.Path = "/.well-known/openid-configuration"
		urls = append(urls, base.String())
		return urls
	}
	original := strings.TrimLeft(u.Path, "/")
	pathInsert := *u
	pathInsert.Path = "/.well-known/oauth-authorization-server/" + original
	urls = append(urls, pathInsert.String())
	oidcInsert := *u
	oidcInsert.Path = "/.well-known/openid-configuration/" + original
	urls = append(urls, oidcInsert.String())
	appended := *u
	appended.Path = strings.TrimRight(u.Path, "/") + "/.well-known/openid-configuration"
	urls = append(urls, appended.String())
	return urls
}

// discoverProtectedResource fetches protected resource metadata for an MCP
// endpoint, trying the well-known locations mandated by the MCP spec and falling
// back to treating the server root as the authorization server.
func discoverProtectedResource(ctx context.Context, serverURL string, httpClient *http.Client) (*oauthex.ProtectedResourceMetadata, error) {
	for _, candidate := range protectedResourceMetadataCandidates(serverURL) {
		prm, err := oauthex.GetProtectedResourceMetadata(ctx, candidate.metadataURL, candidate.resource, httpClient)
		if err != nil {
			continue
		}
		if prm == nil || len(prm.AuthorizationServers) == 0 {
			continue
		}
		return prm, nil
	}
	// Fallback: server root acts as the authorization server (2025-03-26 spec).
	u, err := url.Parse(serverURL)
	if err != nil {
		return nil, fmt.Errorf("parse server url: %w", err)
	}
	root := *u
	root.Path = ""
	root.RawQuery = ""
	return &oauthex.ProtectedResourceMetadata{
		AuthorizationServers: []string{root.String()},
		Resource:             serverURL,
	}, nil
}

type prmCandidate struct {
	metadataURL string
	resource    string
}

// protectedResourceMetadataCandidates mirrors the MCP spec discovery order:
// path-scoped well-known first, then root well-known.
func protectedResourceMetadataCandidates(serverURL string) []prmCandidate {
	u, err := url.Parse(serverURL)
	if err != nil {
		return nil
	}
	var out []prmCandidate
	pathScoped := *u
	pathScoped.Path = "/.well-known/oauth-protected-resource/" + strings.TrimLeft(u.Path, "/")
	pathScoped.RawQuery = ""
	out = append(out, prmCandidate{metadataURL: pathScoped.String(), resource: serverURL})

	rootMeta := *u
	rootMeta.Path = "/.well-known/oauth-protected-resource"
	rootMeta.RawQuery = ""
	rootResource := *u
	rootResource.Path = ""
	rootResource.RawQuery = ""
	out = append(out, prmCandidate{metadataURL: rootMeta.String(), resource: rootResource.String()})
	return out
}

func authStyleForMethod(method string) oauth2.AuthStyle {
	switch strings.TrimSpace(method) {
	case "client_secret_post", "none", "":
		return oauth2.AuthStyleInParams
	case "client_secret_basic":
		return oauth2.AuthStyleInHeader
	default:
		return oauth2.AuthStyleInHeader
	}
}
