import { describe, expect, it } from 'vitest';
import { mergeImportedMcpServers, parseCursorMcpJson } from './cursor-mcp-import';

describe('cursor-mcp-import', () => {
	it('parses stdio servers from Cursor mcp.json', () => {
		const servers = parseCursorMcpJson({
			mcpServers: {
				filesystem: {
					command: 'npx',
					args: ['-y', '@modelcontextprotocol/server-filesystem', '/tmp']
				},
				github: {
					command: 'npx',
					args: ['-y', '@modelcontextprotocol/server-github'],
					env: { GITHUB_PERSONAL_ACCESS_TOKEN: 'secret' },
					disabled: true
				}
			}
		});

		expect(servers).toHaveLength(2);
		expect(servers[0]).toMatchObject({
			id: 'filesystem',
			name: 'filesystem',
			transport: 'stdio',
			command: 'npx',
			args: ['-y', '@modelcontextprotocol/server-filesystem', '/tmp'],
			enabled: true
		});
		expect(servers[1]).toMatchObject({
			id: 'github',
			enabled: false,
			env: { GITHUB_PERSONAL_ACCESS_TOKEN: 'secret' }
		});
	});

	it('parses remote HTTP and SSE servers', () => {
		const servers = parseCursorMcpJson({
			mcpServers: {
				remote: {
					url: 'https://example.com/mcp',
					headers: { Authorization: 'Bearer token' }
				},
				legacy: {
					type: 'sse',
					url: 'https://example.com/sse'
				}
			}
		});

		expect(servers[0]).toMatchObject({
			id: 'remote',
			transport: 'http',
			url: 'https://example.com/mcp',
			headers: { Authorization: 'Bearer token' }
		});
		expect(servers[1]).toMatchObject({
			id: 'legacy',
			transport: 'sse',
			url: 'https://example.com/sse'
		});
	});

	it('skips invalid entries and avoids duplicate ids within one import', () => {
		const servers = parseCursorMcpJson({
			mcpServers: {
				'My Server': { command: 'node', args: ['a.js'] },
				'my server': { command: 'node', args: ['b.js'] },
				empty: {}
			}
		});

		expect(servers).toHaveLength(2);
		expect(servers.map((server) => server.id)).toEqual(['my-server', 'my-server-2']);
	});

	it('merges imported servers without clobbering existing ids', () => {
		const merged = mergeImportedMcpServers(
			[
				{
					id: 'filesystem',
					name: 'filesystem',
					enabled: true,
					transport: 'stdio',
					command: 'npx'
				}
			],
			[
				{
					id: 'filesystem',
					name: 'filesystem',
					enabled: true,
					transport: 'stdio',
					command: 'uvx'
				}
			]
		);

		expect(merged).toHaveLength(2);
		expect(merged[1]?.id).toBe('filesystem-2');
	});
});
