/** MCP server row provenance (`cometmind.mcp.servers[].source`). Keep in sync with config.MCPServerSource*. */
export const MCP_SERVER_SOURCE = {
	catalog: 'catalog',
	user: 'user',
	import: 'import'
} as const;

export type MCPServerSource = (typeof MCP_SERVER_SOURCE)[keyof typeof MCP_SERVER_SOURCE];
