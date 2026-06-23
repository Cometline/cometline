package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	mcppkg "github.com/cometline/cometmind/internal/mcp"
	"github.com/gin-gonic/gin"
)

func init() { gin.SetMode(gin.TestMode) }

func newOAuthTestContext(t *testing.T, serverID string) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/mcp/servers/"+serverID+"/oauth/start", nil)
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
