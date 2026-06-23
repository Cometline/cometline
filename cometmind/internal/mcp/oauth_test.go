package mcp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"golang.org/x/oauth2"
)

func TestOAuthClientInfoRoundTrip(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	const serverID = "atlassian"
	info := &oauthClientInfo{
		AuthorizationEndpoint: "https://auth.example.com/authorize",
		TokenEndpoint:         "https://auth.example.com/token",
		ClientID:              "client-123",
		ClientSecret:          "secret-456",
		Scopes:                []string{"read", "write"},
		Resource:              "https://mcp.example.com/v1/mcp",
		AuthStyle:             oauth2.AuthStyleInParams,
	}
	if err := saveOAuthClientInfo(serverID, info); err != nil {
		t.Fatalf("saveOAuthClientInfo: %v", err)
	}
	got, err := loadOAuthClientInfo(serverID)
	if err != nil {
		t.Fatalf("loadOAuthClientInfo: %v", err)
	}
	if got.TokenEndpoint != info.TokenEndpoint {
		t.Errorf("TokenEndpoint = %q, want %q", got.TokenEndpoint, info.TokenEndpoint)
	}
	if got.ClientID != info.ClientID {
		t.Errorf("ClientID = %q, want %q", got.ClientID, info.ClientID)
	}
	if got.ClientSecret != info.ClientSecret {
		t.Errorf("ClientSecret = %q, want %q", got.ClientSecret, info.ClientSecret)
	}
	if got.Resource != info.Resource {
		t.Errorf("Resource = %q, want %q", got.Resource, info.Resource)
	}
	if len(got.Scopes) != 2 || got.Scopes[0] != "read" || got.Scopes[1] != "write" {
		t.Errorf("Scopes = %v, want [read write]", got.Scopes)
	}
	if got.AuthStyle != oauth2.AuthStyleInParams {
		t.Errorf("AuthStyle = %v, want %v", got.AuthStyle, oauth2.AuthStyleInParams)
	}
}

func TestSaveOAuthClientInfoRejectsIncomplete(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	cases := map[string]*oauthClientInfo{
		"nil":          nil,
		"no token":     {ClientID: "c"},
		"no client id": {TokenEndpoint: "https://t"},
		"both empty":   {},
	}
	for name, info := range cases {
		t.Run(name, func(t *testing.T) {
			if err := saveOAuthClientInfo("srv", info); err == nil {
				t.Errorf("expected error for %s", name)
			}
		})
	}
}

func TestLoadOAuthClientInfoRejectsIncomplete(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	// Write a structurally valid JSON file missing required fields by going
	// around the save guard: marshal a partial struct directly.
	path, err := oauthClientInfoPath("partial")
	if err != nil {
		t.Fatalf("oauthClientInfoPath: %v", err)
	}
	if _, err := OAuthTokenDir(); err != nil {
		t.Fatalf("OAuthTokenDir: %v", err)
	}
	if err := os.WriteFile(path, []byte(`{"clientId":"c"}`), 0o600); err != nil {
		t.Fatalf("write partial: %v", err)
	}
	if _, err := loadOAuthClientInfo("partial"); err == nil {
		t.Error("expected error loading client info without token endpoint")
	}
}

func TestPersistingTokenSourcePersistsRotatedToken(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	const serverID = "rotating"
	// Seed an initial token on disk.
	initial := &oauth2.Token{AccessToken: "old", RefreshToken: "r", Expiry: time.Now().Add(time.Hour)}
	if err := SaveOAuthToken(serverID, initial); err != nil {
		t.Fatalf("SaveOAuthToken: %v", err)
	}

	rotated := &oauth2.Token{AccessToken: "new", RefreshToken: "r2", Expiry: time.Now().Add(2 * time.Hour)}
	ps := &persistingTokenSource{
		serverID: serverID,
		base:     oauth2.StaticTokenSource(rotated),
		last:     initial,
	}
	tok, err := ps.Token()
	if err != nil {
		t.Fatalf("Token: %v", err)
	}
	if tok.AccessToken != "new" {
		t.Fatalf("Token().AccessToken = %q, want new", tok.AccessToken)
	}
	// The rotated token should have been persisted to disk.
	loaded, err := LoadOAuthToken(serverID)
	if err != nil {
		t.Fatalf("LoadOAuthToken: %v", err)
	}
	if loaded.AccessToken != "new" {
		t.Errorf("persisted AccessToken = %q, want new", loaded.AccessToken)
	}
}

func TestProtectedResourceMetadataCandidates(t *testing.T) {
	got := protectedResourceMetadataCandidates("https://mcp.atlassian.com/v1/mcp")
	if len(got) != 2 {
		t.Fatalf("got %d candidates, want 2", len(got))
	}
	wantPathScoped := "https://mcp.atlassian.com/.well-known/oauth-protected-resource/v1/mcp"
	if got[0].metadataURL != wantPathScoped {
		t.Errorf("path-scoped metadataURL = %q, want %q", got[0].metadataURL, wantPathScoped)
	}
	if got[0].resource != "https://mcp.atlassian.com/v1/mcp" {
		t.Errorf("path-scoped resource = %q", got[0].resource)
	}
	wantRoot := "https://mcp.atlassian.com/.well-known/oauth-protected-resource"
	if got[1].metadataURL != wantRoot {
		t.Errorf("root metadataURL = %q, want %q", got[1].metadataURL, wantRoot)
	}
	if got[1].resource != "https://mcp.atlassian.com" {
		t.Errorf("root resource = %q, want https://mcp.atlassian.com", got[1].resource)
	}
}

func TestAuthStyleForMethod(t *testing.T) {
	cases := map[string]oauth2.AuthStyle{
		"client_secret_post":  oauth2.AuthStyleInParams,
		"none":                oauth2.AuthStyleInParams,
		"":                    oauth2.AuthStyleInParams,
		"client_secret_basic": oauth2.AuthStyleInHeader,
		"private_key_jwt":     oauth2.AuthStyleInHeader,
	}
	for method, want := range cases {
		if got := authStyleForMethod(method); got != want {
			t.Errorf("authStyleForMethod(%q) = %v, want %v", method, got, want)
		}
	}
}

func TestRandomStateUnique(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		s, err := randomState()
		if err != nil {
			t.Fatalf("randomState: %v", err)
		}
		if s == "" {
			t.Fatal("randomState returned empty string")
		}
		if seen[s] {
			t.Fatalf("randomState produced duplicate: %q", s)
		}
		seen[s] = true
	}
}

func TestRedirectURL(t *testing.T) {
	if got := RedirectURL(); got != "http://localhost:1456/mcp/oauth/callback" {
		t.Errorf("RedirectURL() = %q", got)
	}
}

func TestAuthServerMetadataURLsPathScoped(t *testing.T) {
	got := authServerMetadataURLs("https://mcp.atlassian.com/v1/mcp")
	want := []string{
		"https://mcp.atlassian.com/.well-known/oauth-authorization-server/v1/mcp",
		"https://mcp.atlassian.com/.well-known/openid-configuration/v1/mcp",
		"https://mcp.atlassian.com/v1/mcp/.well-known/openid-configuration",
	}
	if len(got) != len(want) {
		t.Fatalf("authServerMetadataURLs len = %d, want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("authServerMetadataURLs[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestAuthServerMetadataURLsRoot(t *testing.T) {
	got := authServerMetadataURLs("https://example.com")
	want := []string{
		"https://example.com/.well-known/oauth-authorization-server",
		"https://example.com/.well-known/openid-configuration",
	}
	if len(got) != len(want) {
		t.Fatalf("authServerMetadataURLs(root) = %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("authServerMetadataURLs(root)[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

// TestDiscoverAuthServerMetadataToleratesIssuerMismatch reproduces the Atlassian
// case where the AS metadata document advertises a different issuer host than the
// URL it was fetched from. The strict SDK helper rejects this; ours must accept it.
func TestDiscoverAuthServerMetadataToleratesIssuerMismatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/.well-known/oauth-authorization-server" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"issuer": "https://cf.example.test",
			"authorization_endpoint": "https://example.test/v1/authorize",
			"token_endpoint": "https://cf.example.test/v1/token",
			"registration_endpoint": "https://cf.example.test/v1/register",
			"code_challenge_methods_supported": ["S256"]
		}`))
	}))
	defer srv.Close()

	asm, err := discoverAuthServerMetadata(context.Background(), srv.URL, srv.Client())
	if err != nil {
		t.Fatalf("discoverAuthServerMetadata error = %v", err)
	}
	if asm == nil {
		t.Fatal("discoverAuthServerMetadata returned nil despite valid metadata")
	}
	if asm.Issuer != "https://cf.example.test" {
		t.Errorf("issuer = %q", asm.Issuer)
	}
	if asm.AuthorizationEndpoint == "" || asm.TokenEndpoint == "" {
		t.Errorf("missing endpoints: %+v", asm)
	}
	if asm.RegistrationEndpoint != "https://cf.example.test/v1/register" {
		t.Errorf("registration endpoint = %q", asm.RegistrationEndpoint)
	}
}

func TestDiscoverAuthServerMetadataNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	asm, err := discoverAuthServerMetadata(context.Background(), srv.URL, srv.Client())
	if err != nil {
		t.Fatalf("discoverAuthServerMetadata error = %v", err)
	}
	if asm != nil {
		t.Fatalf("expected nil metadata on all-404, got %+v", asm)
	}
}

// TestOAuthHandlerForWiresWithoutConfigBlock verifies the runtime handler is wired
// whenever a token exists on disk, even when the server has no `oauth` config block
// (the discovery/DCR-only path used by Atlassian).
func TestOAuthHandlerForWiresWithoutConfigBlock(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	const serverID = "server-3"
	// No token yet: handler must be nil.
	if h := oauthHandlerFor(serverID, nil); h != nil {
		t.Fatalf("expected nil handler before token saved, got %#v", h)
	}
	// Save a token (no oauth config block at all).
	if err := SaveOAuthToken(serverID, &oauth2.Token{AccessToken: "tok", TokenType: "Bearer"}); err != nil {
		t.Fatalf("SaveOAuthToken: %v", err)
	}
	if h := oauthHandlerFor(serverID, nil); h == nil {
		t.Fatal("expected non-nil handler once token exists, even without oauth config block")
	}
}
