package server

import (
	"errors"
	"net/http"

	mcppkg "github.com/cometline/cometmind/internal/mcp"
	"github.com/gin-gonic/gin"
)

func (a *App) handleListMCPServers(c *gin.Context) {
	if a.mcpMgr == nil {
		c.JSON(http.StatusOK, gin.H{"servers": []mcppkg.ServerRuntimeStatus{}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"servers": a.mcpMgr.ListServers()})
}

func (a *App) handleListMCPTools(c *gin.Context) {
	if a.mcpMgr == nil {
		c.JSON(http.StatusOK, gin.H{"tools": []mcppkg.ToolInfo{}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"tools": a.mcpMgr.ListToolInfos()})
}

func (a *App) handleTestMCPServer(c *gin.Context) {
	if a.mcpMgr == nil {
		writeError(c, http.StatusServiceUnavailable, "mcp_unavailable", "MCP manager is not initialized")
		return
	}
	id := c.Param("id")
	result := a.mcpMgr.TestServer(c.Request.Context(), id)
	status := http.StatusOK
	if !result.OK {
		status = http.StatusBadGateway
	}
	c.JSON(status, result)
}

func (a *App) handleReconnectMCPServer(c *gin.Context) {
	if a.mcpMgr == nil {
		writeError(c, http.StatusServiceUnavailable, "mcp_unavailable", "MCP manager is not initialized")
		return
	}
	id := c.Param("id")
	if err := a.mcpMgr.Reconnect(c.Request.Context(), id); err != nil {
		body := gin.H{
			"ok":        false,
			"connected": false,
			"error":     err.Error(),
		}
		var connectErr *mcppkg.ConnectError
		if errors.As(err, &connectErr) {
			body["error_code"] = connectErr.Code
			body["error_hint"] = connectErr.Hint
		}
		c.JSON(http.StatusBadGateway, body)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// handleStartMCPOAuth runs the interactive OAuth flow (discovery, dynamic client
// registration, browser authorization, token exchange) for one MCP server and
// reconnects it on success. This is a blocking call: it returns once the user
// completes the browser round-trip or the flow fails/times out.
func (a *App) handleStartMCPOAuth(c *gin.Context) {
	if a.mcpMgr == nil {
		writeError(c, http.StatusServiceUnavailable, "mcp_unavailable", "MCP manager is not initialized")
		return
	}
	id := c.Param("id")
	if err := a.mcpMgr.StartOAuth(c.Request.Context(), id); err != nil {
		if ok, body := mcpOAuthFlowBody(err); ok {
			c.JSON(http.StatusOK, body)
			return
		}
		writeError(c, http.StatusBadGateway, "mcp_oauth_failed", err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "connected": true})
}

func mcpOAuthFlowBody(err error) (bool, gin.H) {
	var reconnectErr *mcppkg.OAuthReconnectError
	if !errors.As(err, &reconnectErr) {
		return false, nil
	}
	body := gin.H{
		"ok":        true,
		"connected": false,
		"error":     err.Error(),
	}
	if code := reconnectErr.Code(); code != "" {
		body["error_code"] = code
	}
	if hint := reconnectErr.Hint(); hint != "" {
		body["error_hint"] = hint
	}
	return true, body
}
