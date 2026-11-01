import type { MCPServerConfig } from '#lib/cometmind-settings.js';
import type { McpCatalogEntry } from '#lib/client/cometmind.js';
import { rewriteKnownHostUrl } from '#lib/features/settings/mcp-url.js';

export function catalogHostKey(rawUrl: string): string {
	const trimmed = (rawUrl ?? '').trim();
	if (!trimmed) return '';
	try {
		const parsed = new URL(trimmed);
		const path = parsed.pathname.replace(/\/$/, '') || '/';
		return `${parsed.hostname.toLowerCase()}${path}`;
	} catch {
		return trimmed.toLowerCase();
	}
}

export function isCatalogLocked(server: MCPServerConfig): boolean {
	return server.source === 'catalog';
}

export function findExistingCatalogServer(
	servers: MCPServerConfig[],
	entry: McpCatalogEntry
): MCPServerConfig | undefined {
	const catalogId = entry.id;
	const hostKey = catalogHostKey(entry.url);
	for (const server of servers) {
		if (server.catalogId === catalogId || server.id === catalogId) {
			return server;
		}
		if (hostKey && catalogHostKey(server.url ?? '') === hostKey) {
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
		source: 'catalog',
		catalogId: entry.id,
		catalogVersion: entry.version
	};
}

export function unlockCatalogServer(server: MCPServerConfig): MCPServerConfig {
	if (server.source !== 'catalog') return server;
	return { ...server, source: 'user' };
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
