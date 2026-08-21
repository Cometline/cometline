package mcp

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os/exec"
	"strings"
)

// ConnectErrorCode classifies a failed MCP connect / handshake / auth step so
// the Settings UI and agent tools can tell the user what to do next instead of
// dumping a raw SDK timeout.
type ConnectErrorCode string

const (
	CodeNeedsAuth        ConnectErrorCode = "needs_auth"
	CodeAuthExpired      ConnectErrorCode = "auth_expired"
	CodeHandshakeTimeout ConnectErrorCode = "handshake_timeout"
	CodeTransportTimeout ConnectErrorCode = "transport_timeout"
	CodeCommandNotFound  ConnectErrorCode = "command_not_found"
	CodeUnauthorized     ConnectErrorCode = "unauthorized"
	CodeBadURL           ConnectErrorCode = "bad_url"
	CodeProtocol         ConnectErrorCode = "protocol"
)

// ConnectError is a classified MCP connection failure.
type ConnectError struct {
	Code    ConnectErrorCode
	Hint    string
	wrapped error
}

func (e *ConnectError) Error() string {
	if e == nil {
		return ""
	}
	if e.wrapped != nil {
		return e.wrapped.Error()
	}
	return e.Hint
}

func (e *ConnectError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.wrapped
}

func newConnectError(code ConnectErrorCode, hint string, err error) *ConnectError {
	return &ConnectError{Code: code, Hint: hint, wrapped: err}
}

func classifyConnectError(err error) *ConnectError {
	return classifyConnectErrorFor("", err)
}

func classifyConnectErrorFor(serverID string, err error) *ConnectError {
	if err == nil {
		return nil
	}
	var already *ConnectError
	if errors.As(err, &already) {
		return already
	}

	msg := err.Error()
	lower := strings.ToLower(msg)

	var execErr *exec.Error
	if errors.As(err, &execErr) && errors.Is(execErr.Err, exec.ErrNotFound) {
		return newConnectError(CodeCommandNotFound,
			fmt.Sprintf("Command %q was not found on PATH. Install it or use the full path.", execErr.Name),
			err)
	}

	if strings.Contains(lower, "command is required") ||
		strings.Contains(lower, "url is required") ||
		strings.Contains(lower, "unsupported transport") {
		return newConnectError(CodeBadURL, "This server is missing a command or URL, or uses an unsupported transport.", err)
	}

	if strings.Contains(lower, "invalid or expired") ||
		strings.Contains(lower, "oauth token") && strings.Contains(lower, "expired") {
		return newConnectError(CodeAuthExpired,
			"The saved sign-in expired. Click Connect with OAuth and sign in again.", err)
	}

	unauthorized := strings.Contains(lower, "401") ||
		strings.Contains(lower, "unauthorized") ||
		strings.Contains(lower, "invalid_token")
	if unauthorized {
		if serverID != "" && OAuthConnected(serverID) {
			if OAuthHintFromError(err) {
				return newConnectError(CodeAuthExpired,
					"The saved sign-in was rejected. Click Connect with OAuth and sign in again.", err)
			}
			return newConnectError(CodeUnauthorized,
				"Signed in, but the server still rejected the request. Click Connect with OAuth and sign in again.", err)
		}
		return newConnectError(CodeNeedsAuth,
			"This server needs sign-in. Click Connect with OAuth, or add an API token in Headers.", err)
	}

	if isTimeoutError(err) {
		if strings.Contains(lower, "notifications/initialized") ||
			strings.Contains(lower, `"initialize"`) ||
			strings.Contains(lower, "sending \"initialize\"") ||
			strings.Contains(lower, "list tools") {
			return newConnectError(CodeHandshakeTimeout,
				"Signed in (if required), but the MCP handshake timed out. Click Reconnect. If this keeps happening, the server may be slow or blocking the connection.", err)
		}
		return newConnectError(CodeTransportTimeout,
			"The MCP server stopped responding. Click Reconnect.", err)
	}

	return newConnectError(CodeProtocol, "The MCP server could not complete the connection.", err)
}

// OAuthHintFromError reports whether err looks like a rejected bearer token
// rather than a missing one.
func OAuthHintFromError(err error) bool {
	if err == nil {
		return false
	}
	lower := strings.ToLower(err.Error())
	return strings.Contains(lower, "oauth") || strings.Contains(lower, "bearer") || strings.Contains(lower, "invalid_token")
}

func isTimeoutError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	lower := strings.ToLower(err.Error())
	return strings.Contains(lower, "deadline exceeded") ||
		strings.Contains(lower, "timed out") ||
		strings.Contains(lower, "timeout")
}

func errorCodeOf(err error) ConnectErrorCode {
	if classified := classifyConnectError(err); classified != nil {
		return classified.Code
	}
	return ""
}

func errorHintOf(err error) string {
	if classified := classifyConnectError(err); classified != nil {
		return classified.Hint
	}
	return ""
}

func skipAutoReconnect(code ConnectErrorCode) bool {
	switch code {
	case CodeCommandNotFound, CodeBadURL, CodeNeedsAuth, CodeUnauthorized, CodeAuthExpired:
		return true
	default:
		return false
	}
}
