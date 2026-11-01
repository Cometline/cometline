import { describe, expect, it } from 'vitest';
import {
	catalogHostKey,
	catalogMetaLine,
	findExistingCatalogServer,
	isCatalogEntryConnected,
	materializeCatalogServer
} from '#lib/features/settings/mcp-catalog.js';
import type { McpCatalogEntry } from '#lib/client/cometmind.js';

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
		expect(server.source).toBe('catalog');
		expect(server.catalogId).toBe('atlassian');
		expect(server.url).toBe('https://mcp.atlassian.com/v2/mcp');
	});

	it('reuses row by catalogId or host key', () => {
		const existing = materializeCatalogServer(atlassian);
		existing.id = 'custom-atlassian';
		const byHost = findExistingCatalogServer(
			[
				{
					id: 'other',
					name: 'Other',
					enabled: true,
					transport: 'http',
					url: 'https://api.heptabase.com/mcp'
				},
				existing
			],
			atlassian
		);
		expect(byHost?.id).toBe('custom-atlassian');
		expect(catalogHostKey('https://api.heptabase.com/mcp/')).toBe(
			catalogHostKey('https://api.heptabase.com/mcp')
		);
	});

	it('formats meta line and connected state', () => {
		expect(catalogMetaLine(atlassian, undefined, false)).toBe('Remote MCP · OAuth');
		expect(
			catalogMetaLine(atlassian, { status: 'connected', tool_count: 3 } as never, true)
		).toBe('3 tools · OAuth');
		expect(
			isCatalogEntryConnected(
				{ id: 'a', name: 'A', enabled: true, transport: 'http' },
				{ status: 'connected' } as never,
				true
			)
		).toBe(true);
	});
});
