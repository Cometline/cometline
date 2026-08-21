package mcp

// ToolBinding connects one discovered MCP tool to the manager. Execute looks up
// the live session via Manager.CallTool so a reconnect is visible on the next call.
type ToolBinding struct {
	ServerID string
	Tool     DiscoveredTool
}
