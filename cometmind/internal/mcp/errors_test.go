package mcp

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"testing"

	"golang.org/x/oauth2"
)

func TestClassifyConnectErrorTimeouts(t *testing.T) {
	handshake := classifyConnectError(fmt.Errorf(`connect: sending "notifications/initialized": rejected by transport: Post "https://mcp.atlassian.com/v1/mcp/authv2": %w`, context.DeadlineExceeded))
	if handshake.Code != CodeHandshakeTimeout {
		t.Fatalf("code = %q, want %q", handshake.Code, CodeHandshakeTimeout)
	}
	if handshake.Hint == "" {
		t.Fatal("hint is empty")
	}

	generic := classifyConnectError(fmt.Errorf("post: %w", context.DeadlineExceeded))
	if generic.Code != CodeTransportTimeout {
		t.Fatalf("generic timeout code = %q, want %q", generic.Code, CodeTransportTimeout)
	}
}

func TestClassifyConnectErrorCommandNotFound(t *testing.T) {
	err := &exec.Error{Name: "nope-mcp", Err: exec.ErrNotFound}
	got := classifyConnectError(err)
	if got.Code != CodeCommandNotFound {
		t.Fatalf("code = %q, want %q", got.Code, CodeCommandNotFound)
	}
}

func TestClassifyConnectErrorNeedsAuth(t *testing.T) {
	got := classifyConnectError(errors.New(`connect: sending "initialize": unauthorized: 401`))
	if got.Code != CodeNeedsAuth {
		t.Fatalf("code = %q, want %q", got.Code, CodeNeedsAuth)
	}
}

func TestClassifyConnectErrorAuthExpired(t *testing.T) {
	got := classifyConnectError(errors.New(`MCP OAuth token for server "jira" is invalid or expired; re-run Connect with OAuth`))
	if got.Code != CodeAuthExpired {
		t.Fatalf("code = %q, want %q", got.Code, CodeAuthExpired)
	}
}

func TestClassifyConnectErrorCanceledIsNotTimeout(t *testing.T) {
	got := classifyConnectError(fmt.Errorf("connect: %w", context.Canceled))
	if got.Code == CodeHandshakeTimeout || got.Code == CodeTransportTimeout {
		t.Fatalf("canceled classified as %q, want protocol", got.Code)
	}
}

func TestClassifyConnectErrorUnauthorizedWhenTokenPresent(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	const serverID = "jira"
	if err := SaveOAuthToken(serverID, &oauth2.Token{AccessToken: "tok"}); err != nil {
		t.Fatalf("SaveOAuthToken: %v", err)
	}
	got := classifyConnectErrorFor(serverID, errors.New(`connect: sending "initialize": unauthorized: 401`))
	if got.Code != CodeUnauthorized {
		t.Fatalf("code = %q, want %q", got.Code, CodeUnauthorized)
	}
}

func TestOAuthReconnectErrorIsClassified(t *testing.T) {
	inner := classifyConnectError(fmt.Errorf(`connect: sending "notifications/initialized": %w`, context.DeadlineExceeded))
	err := &OAuthReconnectError{Err: inner}
	if !errors.As(err, new(*OAuthReconnectError)) {
		t.Fatal("expected errors.As to match OAuthReconnectError")
	}
	if got := err.Error(); !strings.Contains(got, "handshake timed out") {
		t.Fatalf("error = %q, want handshake timeout hint", got)
	}
	if err.Code() != CodeHandshakeTimeout {
		t.Fatalf("code = %q, want %q", err.Code(), CodeHandshakeTimeout)
	}
	if err.Hint() == "" {
		t.Fatal("hint is empty")
	}
}
