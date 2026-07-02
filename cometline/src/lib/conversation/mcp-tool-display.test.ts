import { describe, expect, it } from 'vitest';
import {
	formatToolDisplayName,
	parseMcpRegistryName,
	sanitizeMcpToolNamePart
} from './mcp-tool-display';

describe('sanitizeMcpToolNamePart', () => {
	it('matches Go sanitizer behavior', () => {
		expect(sanitizeMcpToolNamePart('server-1782968109811')).toBe('server-1782968109811');
		expect(sanitizeMcpToolNamePart('plugin/browser')).toBe('plugin_browser');
	});
});

describe('parseMcpRegistryName', () => {
	const servers = [
		{ id: 'server-1782968109811', name: 'SearXNG' },
		{ id: 'demo', name: 'Demo MCP' }
	];

	it('resolves server display name and tool suffix', () => {
		expect(
			parseMcpRegistryName('mcp_server-1782968109811_searxng_web_search', servers)
		).toEqual({
			serverName: 'SearXNG',
			toolName: 'searxng_web_search'
		});
	});

	it('prefers the longest matching server id', () => {
		expect(parseMcpRegistryName('mcp_demo_echo', servers)).toEqual({
			serverName: 'Demo MCP',
			toolName: 'echo'
		});
	});

	it('returns null for non-mcp tools', () => {
		expect(parseMcpRegistryName('read_file', servers)).toBeNull();
	});
});

describe('formatToolDisplayName', () => {
	it('formats MCP tools with the configured server name', () => {
		expect(
			formatToolDisplayName('mcp_server-1782968109811_searxng_web_search', [
				{ id: 'server-1782968109811', name: 'SearXNG' }
			])
		).toBe('SearXNG · searxng_web_search');
	});

	it('leaves built-in tool names unchanged', () => {
		expect(formatToolDisplayName('read_file', [])).toBe('read_file');
	});
});