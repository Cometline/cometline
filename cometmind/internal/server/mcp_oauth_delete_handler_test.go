package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	mcppkg "github.com/Cometline/cometline/cometmind/internal/mcp"
	"github.com/gin-gonic/gin"
)

func TestHandleDeleteMCPOAuthCredentials(t *testing.T) {
	gin.SetMode(gin.TestMode)
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	home := dir
	_ = os.MkdirAll(filepath.Join(home, ".cometmind", "mcp-oauth"), 0o700)
	tokenPath := filepath.Join(home, ".cometmind", "mcp-oauth", "linear.json")
	if err := os.WriteFile(tokenPath, []byte(`{"access_token":"x"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if !mcppkg.OAuthConnected("linear") {
		t.Fatal("expected token before delete")
	}

	app := &App{}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: "linear"}}
	c.Request = httptest.NewRequest(http.MethodDelete, "/api/v1/mcp/servers/linear/oauth-credentials", nil)
	app.handleDeleteMCPOAuthCredentials(c)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	if mcppkg.OAuthConnected("linear") {
		t.Fatal("expected token removed")
	}
}
