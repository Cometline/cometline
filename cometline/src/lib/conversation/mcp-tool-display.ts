/** Mirrors cometmind/internal/mcp/config.go sanitizeToolNamePart. */
export function sanitizeMcpToolNamePart(value: string): string {
	let part = value.trim();
	if (!part) return 'unknown';
	part = part.replace(/[^a-zA-Z0-9_-]+/g, '_');
	part = part.replace(/^_+|_+$/g, '');
	return part || 'unknown';
}

export type McpServerNameLookup = { id: string; name: string };

/**
 * Parse a registry tool name (`mcp_{serverId}_{toolName}`) into the configured
 * server display name and the underlying MCP tool name.
 */
export function parseMcpRegistryName(
	registryName: string,
	servers: McpServerNameLookup[]
): { serverName: string; toolName: string } | null {
	if (!registryName.startsWith('mcp_')) return null;
	const rest = registryName.slice(4);
	const sorted = [...servers].sort((a, b) => b.id.length - a.id.length);
	for (const server of sorted) {
		const prefix = `${sanitizeMcpToolNamePart(server.id)}_`;
		if (!rest.startsWith(prefix)) continue;
		const toolName = rest.slice(prefix.length);
		if (!toolName) return null;
		const displayName = String(server.name ?? '').trim() || server.id;
		return { serverName: displayName, toolName };
	}
	return null;
}

/** User-facing label for a tool row (built-in tools pass through unchanged). */
export function formatToolDisplayName(
	registryName: string,
	servers: McpServerNameLookup[] = []
): string {
	const parsed = parseMcpRegistryName(registryName, servers);
	if (!parsed) return registryName;
	return `${parsed.serverName} · ${parsed.toolName}`;
}