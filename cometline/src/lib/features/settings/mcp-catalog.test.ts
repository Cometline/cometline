import { describe, expect, it } from 'vitest';
import {
	catalogHostKey,
	findExistingCatalogServer,
	isCatalogProvenance,
	materializeCatalogServer,
	mergeCatalogConnectExisting
} from '#lib/features/settings/mcp-catalog.js';
import { MCP_SERVER_SOURCE } from '#lib/features/settings/mcp-server-source.js';
import type { McpCatalogEntry } from '#lib/client/cometmind.js';
import type { MCPServerConfig } from '#lib/cometmind-settings.js';

const atlassian: McpCatalogEntry = {
	id: 'atlassian',
	version: '1',
	name: 'Atlassian',
	description: 'Jira and Confluence',
	transport: 'http',
	url: 'https://mcp.atlassian.com/v2/mcp',
	authKind: 'oauth',
	oauthScopes: [],
	defaultAllowedTools: []
};

describe('mcp-catalog', () => {
	it('materializes catalog metadata and normalized URL', () => {
		const server = materializeCatalogServer(atlassian);
		expect(server.source).toBe(MCP_SERVER_SOURCE.catalog);
		expect(server.catalogId).toBe('atlassian');
		expect(server.url).toBe('https://mcp.atlassian.com/v2/mcp');
	});

	it('dedupes Atlassian legacy URL via rewrite before host compare', () => {
		const legacy: MCPServerConfig = {
			id: 'my-atlassian',
			name: 'Custom Atlassian',
			enabled: true,
			transport: 'http',
			url: 'https://mcp.atlassian.com/v1/mcp',
			source: 'user'
		};
		expect(findExistingCatalogServer([legacy], atlassian)?.id).toBe('my-atlassian');
		expect(catalogHostKey('https://mcp.atlassian.com/v1/mcp')).toBe('mcp.atlassian.com');
		expect(catalogHostKey('https://mcp.atlassian.com/v2/mcp')).toBe('mcp.atlassian.com');
	});

	it('reuses custom row when server id matches catalog id (avoid duplicate ids)', () => {
		const custom: MCPServerConfig = {
			id: 'atlassian',
			name: 'My custom row',
			enabled: true,
			transport: 'http',
			url: 'https://example.com/mcp',
			source: 'user'
		};
		expect(findExistingCatalogServer([custom], atlassian)?.id).toBe('atlassian');
	});

	it('Connect on unlocked row enables only without re-locking or overwriting url', () => {
		const existing: MCPServerConfig = {
			id: 'atlassian',
			name: 'Renamed by user',
			enabled: false,
			transport: 'http',
			url: 'https://example.com/custom',
			source: 'user',
			catalogId: 'atlassian'
		};
		const merged = mergeCatalogConnectExisting(existing, materializeCatalogServer(atlassian));
		expect(merged.source).toBe('user');
		expect(merged.name).toBe('Renamed by user');
		expect(merged.url).toBe('https://example.com/custom');
		expect(merged.enabled).toBe(true);
	});

	it('reuses same-host custom row when path differs from catalog URL', () => {
		const linear: McpCatalogEntry = {
			...atlassian,
			id: 'linear',
			url: 'https://mcp.linear.app/mcp'
		};
		const custom: MCPServerConfig = {
			id: 'my-linear',
			name: 'Linear SSE',
			enabled: true,
			transport: 'http',
			url: 'https://mcp.linear.app/sse',
			source: MCP_SERVER_SOURCE.user
		};
		expect(findExistingCatalogServer([custom], linear)?.id).toBe('my-linear');
		expect(catalogHostKey('https://mcp.linear.app/mcp')).toBe('mcp.linear.app');
		expect(catalogHostKey('https://mcp.linear.app/sse')).toBe('mcp.linear.app');
	});

	it('reuses catalog-locked row by updating recipe fields', () => {
		const existing: MCPServerConfig = {
			id: 'atlassian',
			name: 'Atlassian',
			enabled: true,
			transport: 'http',
			url: 'https://mcp.atlassian.com/v2/mcp',
			source: MCP_SERVER_SOURCE.catalog,
			catalogId: 'atlassian',
			catalogVersion: '1'
		};
		const merged = mergeCatalogConnectExisting(existing, materializeCatalogServer(atlassian));
		expect(merged.source).toBe(MCP_SERVER_SOURCE.catalog);
		expect(merged.catalogId).toBe('atlassian');
	});
});
