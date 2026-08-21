package server

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	mcppkg "github.com/cometline/cometmind/internal/mcp"
	"github.com/gin-gonic/gin"
)

func init() { gin.SetMode(gin.TestMode) }

func newOAuthTestContext(t *testing.T, serverID string) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/mcp/servers/"+serverID+"/oauth-flows", nil)
	c.Params = gin.Params{{Key: "id", Value: serverID}}
	return c, w
}

func TestHandleStartMCPOAuthNilManager(t *testing.T) {
	app := &App{}
	c, w := newOAuthTestContext(t, "atlassian")
	app.handleStartMCPOAuth(c)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusServiceUnavailable)
	}
}

func TestHandleStartMCPOAuthUnknownServer(t *testing.T) {
	mgr := mcppkg.NewManager(mcppkg.Config{Enabled: true})
	app := &App{mcpMgr: mgr}
	c, w := newOAuthTestContext(t, "does-not-exist")
	app.handleStartMCPOAuth(c)
	if w.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadGateway)
	}
}

func TestMCPOAuthFlowBodyHandshakeFailure(t *testing.T) {
	inner := fmt.Errorf(`connect: sending "notifications/initialized": %w`, context.DeadlineExceeded)
	ok, body := mcpOAuthFlowBody(&mcppkg.OAuthReconnectError{Err: inner})
	if !ok {
		t.Fatal("expected oauth reconnect failure to be a 200 body")
	}
	if body["ok"] != true || body["connected"] != false {
		t.Fatalf("body = %#v", body)
	}
	if body["error_code"] != mcppkg.CodeHandshakeTimeout {
		t.Fatalf("error_code = %#v, want %q", body["error_code"], mcppkg.CodeHandshakeTimeout)
	}
	hint, _ := body["error_hint"].(string)
	if hint == "" {
		t.Fatal("error_hint is empty")
	}
}

func TestMCPOAuthFlowBodyOtherError(t *testing.T) {
	ok, _ := mcpOAuthFlowBody(fmt.Errorf("unknown MCP server"))
	if ok {
		t.Fatal("non-reconnect errors must not map to 200")
	}
}

func TestHandleReconnectMCPServerReturnsClassifiedError(t *testing.T) {
	mgr := mcppkg.NewManager(mcppkg.Config{
		Enabled: true,
		Servers: []mcppkg.ServerConfig{{
			ID:        "bad-url",
			Name:      "Bad URL",
			Enabled:   true,
			Transport: mcppkg.TransportHTTP,
			URL:       "ftp://example.com/mcp",
		}},
	})
	mgr.Start(context.Background())
	app := &App{mcpMgr: mgr}
	c, w := newOAuthTestContext(t, "bad-url")
	app.handleReconnectMCPServer(c)
	if w.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadGateway)
	}
	if body := w.Body.String(); !strings.Contains(body, `"error_code":"bad_url"`) || !strings.Contains(body, `"error_hint"`) {
		t.Fatalf("body = %s, want classified error", body)
	}
}
