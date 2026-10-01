package mcp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/oauthex"
	"golang.org/x/oauth2"
)

func TestValidateOAuthFlowOptions(t *testing.T) {
	fetch := AuthCodeFetcher(func(context.Context, string) (string, string, error) { return "", "", nil })
	valid := OAuthFlowOptions{ServerID: "s", ServerURL: "https://mcp.example", RedirectURL: "http://localhost/cb"}
	if err := validateOAuthFlowOptions(valid, fetch); err != nil {
		t.Fatalf("valid options: %v", err)
	}
	cases := map[string]func(*OAuthFlowOptions, *AuthCodeFetcher){
		"server id":    func(o *OAuthFlowOptions, _ *AuthCodeFetcher) { o.ServerID = " " },
		"server url":   func(o *OAuthFlowOptions, _ *AuthCodeFetcher) { o.ServerURL = "" },
		"redirect url": func(o *OAuthFlowOptions, _ *AuthCodeFetcher) { o.RedirectURL = "" },
		"fetcher":      func(_ *OAuthFlowOptions, f *AuthCodeFetcher) { *f = nil },
	}
	for name, mutate := range cases {
		opts, f := valid, fetch
		mutate(&opts, &f)
		if err := validateOAuthFlowOptions(opts, f); err == nil {
			t.Fatalf("missing %s: error = nil", name)
		}
	}
}

func TestResolveAuthServerMetadataFallsBackToPredefinedEndpoints(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	defer srv.Close()
	asm, err := resolveAuthServerMetadata(context.Background(), srv.URL+"/", srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	if asm.AuthorizationEndpoint != srv.URL+"/authorize" || asm.TokenEndpoint != srv.URL+"/token" || asm.RegistrationEndpoint != srv.URL+"/register" {
		t.Fatalf("fallback metadata = %+v", asm)
	}
}

func TestResolveOAuthClientUsesManualClientID(t *testing.T) {
	client, err := resolveOAuthClient(context.Background(), OAuthFlowOptions{ManualClientID: " manual "}, &oauthex.AuthServerMeta{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if client.id != "manual" || client.secret != "" || client.authStyle != oauth2.AuthStyleInParams {
		t.Fatalf("client = %+v", client)
	}
}

func TestResolveOAuthClientRequiresRegistrationEndpoint(t *testing.T) {
	if _, err := resolveOAuthClient(context.Background(), OAuthFlowOptions{}, &oauthex.AuthServerMeta{}, nil); err == nil {
		t.Fatal("error = nil, want missing registration endpoint error")
	}
}
