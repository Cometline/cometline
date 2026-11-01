import type { MCPServerConfig, MCPTransport } from '#lib/cometmind-settings.js';
import type { McpCatalogEntry, McpServerStatus } from '#lib/client/cometmind.js';
import { MCP_SERVER_SOURCE } from '#lib/features/settings/mcp-server-source.js';
import { rewriteKnownHostUrl } from '#lib/features/settings/mcp-url.js';

export function catalogHostKey(rawUrl: string): string {
	const normalized = rewriteKnownHostUrl(String(rawUrl ?? '').trim());
	if (!normalized) return '';
	try {
		return new URL(normalized).hostname.toLowerCase();
	} catch {
		return normalized.toLowerCase();
	}
}

export function isCatalogLocked(server: MCPServerConfig): boolean {
	return server.source === MCP_SERVER_SOURCE.catalog;
}

/** True when this row is a catalog install for the given recipe id. */
export function isCatalogProvenance(server: MCPServerConfig, entryId: string): boolean {
	const id = String(entryId ?? '').trim();
	if (!id) return false;
	return server.source === MCP_SERVER_SOURCE.catalog && server.catalogId === id;
}

export function findExistingCatalogServer(
	servers: MCPServerConfig[],
	entry: McpCatalogEntry
): MCPServerConfig | undefined {
	const catalogId = entry.id;
	const hostKey = catalogHostKey(entry.url);
	for (const server of servers) {
		if (server.id === catalogId) {
			return server;
		}
		if (server.catalogId === catalogId || isCatalogProvenance(server, catalogId)) {
			return server;
		}
		const sk = catalogHostKey(server.url ?? '');
		if (hostKey && sk === hostKey) {
			return server;
		}
	}
	return undefined;
}

export function materializeCatalogServer(
	entry: McpCatalogEntry,
	inputs: Record<string, string> = {}
): MCPServerConfig {
	const env: Record<string, string> = {};
	const headers: Record<string, string> = {};
	for (const req of entry.requiredInputs ?? []) {
		const value = String(inputs[req.key] ?? '').trim();
		if (!value) continue;
		if (req.kind === 'header') {
			headers[req.key] = value;
		} else {
			env[req.key] = value;
		}
	}
	const url = rewriteKnownHostUrl(entry.url);
	const oauth =
		entry.authKind === 'oauth' && (entry.oauthScopes?.length ?? 0) > 0
			? { scopes: [...(entry.oauthScopes ?? [])] }
			: entry.authKind === 'oauth'
				? {}
				: undefined;
	return {
		id: entry.id,
		name: entry.name,
		enabled: true,
		transport: 'http',
		url,
		env,
		headers,
		oauth,
		allowedTools: [...(entry.defaultAllowedTools ?? [])],
		source: MCP_SERVER_SOURCE.catalog,
		catalogId: entry.id,
		catalogVersion: entry.version
	};
}

/**
 * Apply a catalog Connect action to an existing row without clobbering unlocked/custom/import edits.
 */
export function mergeCatalogConnectExisting(
	existing: MCPServerConfig,
	materialized: MCPServerConfig
): MCPServerConfig {
	if (existing.source === MCP_SERVER_SOURCE.catalog) {
		return {
			...materialized,
			id: existing.id,
			env: { ...(materialized.env ?? {}), ...(existing.env ?? {}) },
			headers: { ...(materialized.headers ?? {}), ...(existing.headers ?? {}) },
			allowedTools:
				(existing.allowedTools?.length ?? 0) > 0
					? existing.allowedTools
					: materialized.allowedTools
		};
	}
	return {
		...existing,
		enabled: true
	};
}

export function unlockCatalogServer(server: MCPServerConfig): MCPServerConfig {
	if (server.source !== MCP_SERVER_SOURCE.catalog) return server;
	return { ...server, source: MCP_SERVER_SOURCE.user };
}

export function catalogAuthLabel(entry: McpCatalogEntry): string {
	switch (entry.authKind) {
		case 'oauth':
			return 'OAuth';
		case 'api_key':
			return 'API key';
		default:
			return 'None';
	}
}

export function catalogScopesLabel(entry: McpCatalogEntry): string {
	const scopes = entry.oauthScopes ?? [];
	if (scopes.length === 0) return 'Default scopes (discovered at connect)';
	return scopes.join(', ');
}

export function isCatalogEntryConnected(
	installed: MCPServerConfig | undefined,
	status: McpServerStatus | undefined,
	mcpEnabled: boolean
): boolean {
	if (!installed?.enabled || !mcpEnabled) return false;
	return status?.status === 'connected';
}

/** One-line subtitle under the connector name in the gallery grid. */
export function catalogMetaLine(
	entry: McpCatalogEntry,
	status: McpServerStatus | undefined,
	connected: boolean
): string {
	const auth = catalogAuthLabel(entry);
	if (connected) {
		const count = status?.tool_count ?? 0;
		if (count > 0) {
			return `${count} tool${count === 1 ? '' : 's'} · ${auth}`;
		}
		return `Connected · ${auth}`;
	}
	return `Remote MCP · ${auth}`;
}
