import { describe, expect, it } from 'vitest';
import type { MCPServerConfig } from '#lib/cometmind-settings.js';
import {
	displayError,
	formatEnv,
	mergeKnownTools,
	mergePersistedServers,
	parseArgs,
	parseEnv,
	sameToolNames,
	DENY_ALL_TOOLS,
	isToolAllowed,
	setAllAllowedTools,
	statusClass,
	statusLabel,
	toggledAllowedTools,
	toolMatchesQuery
} from './mcp-panel-format';

const server: MCPServerConfig = {
	id: 'server-1',
	name: 'Server',
	enabled: true,
	transport: 'stdio',
	command: '',
	args: [],
	env: {},
	url: '',
	headers: {}
};

describe('mcp-panel-format', () => {
	it('round-trips env text and skips incomplete lines', () => {
		expect(parseEnv('A=1\nB\n =x\n C = two ')).toEqual({ A: '1', C: 'two' });
		expect(formatEnv({ A: '1', C: 'two' })).toBe('A=1\nC=two');
		expect(parseArgs('  -y   pkg ')).toEqual(['-y', 'pkg']);
	});

	it('labels auth errors as needing sign-in', () => {
		const status = {
			id: 'server-1',
			name: 'Server',
			enabled: true,
			transport: 'http',
			status: 'error',
			error_code: 'needs_auth',
			tool_count: 0
		} as const;
		expect(statusLabel(status, server, true)).toBe('Needs sign-in');
		expect(statusClass(status, server, true)).toBe('pending');
		expect(statusLabel(status, server, false)).toBe('Off');
	});

	it('joins hint and raw error only when expanded', () => {
		const status = {
			id: 'server-1',
			name: 'Server',
			enabled: true,
			transport: 'http',
			status: 'error',
			tool_count: 0,
			error_hint: 'hint',
			last_error: 'raw'
		} as const;
		expect(displayError(status)).toBe('hint');
		expect(displayError(status, true)).toBe('hint\nraw');
	});

	it('merges known tools with discovered descriptions and extra names', () => {
		const merged = mergeKnownTools(
			[{ name: 'b', description: 'old' }],
			[
				{
					server_id: 'server-1',
					server_name: 'Server',
					tool_name: 'b',
					registry_name: 'mcp_server-1_b',
					description: 'new'
				}
			],
			['a', 'b']
		);
		expect(merged).toEqual([
			{ name: 'a', description: '' },
			{ name: 'b', description: 'new' }
		]);
		expect(sameToolNames(merged, [...merged])).toBe(true);
	});

	it('overlays persisted servers except the one being edited', () => {
		const editing = { ...server, id: 'editing', name: 'Draft' };
		const merged = mergePersistedServers(
			[server, editing],
			[
				{ ...server, enabled: false },
				{ ...editing, name: 'Persisted' },
				{ ...server, id: 'new' }
			],
			'editing'
		);
		expect(merged.map((item) => [item.id, item.name, item.enabled])).toEqual([
			['server-1', 'Server', false],
			['editing', 'Draft', true],
			['new', 'Server', true]
		]);
	});

	it('materializes and collapses the allow-list when toggling tools', () => {
		expect(toggledAllowedTools(['a', 'b'], [], 'a')).toEqual(['b']);
		expect(toggledAllowedTools(['a', 'b'], ['b'], 'a')).toEqual([]);
		expect(toggledAllowedTools(['a', 'b'], ['b'], 'b')).toEqual([DENY_ALL_TOOLS]);
		expect(toggledAllowedTools(['a'], [DENY_ALL_TOOLS], 'a')).toEqual([]);
	});

	it('opens every tool or stores the deny-all marker', () => {
		expect(setAllAllowedTools(true)).toEqual([]);
		expect(setAllAllowedTools(false)).toEqual([DENY_ALL_TOOLS]);
		expect(isToolAllowed({ ...server, allowedTools: [] }, 'deploy')).toBe(true);
		expect(isToolAllowed({ ...server, allowedTools: [DENY_ALL_TOOLS] }, 'deploy')).toBe(false);
		expect(isToolAllowed({ ...server, allowedTools: ['deploy'] }, 'deploy')).toBe(true);
	});

	it('filters tools by name or description', () => {
		expect(
			toolMatchesQuery({ name: 'vercel_deploy', description: 'Ship a project' }, 'ship')
		).toBe(true);
		expect(
			toolMatchesQuery({ name: 'vercel_deploy', description: 'Ship a project' }, 'LOG')
		).toBe(false);
		expect(toolMatchesQuery({ name: 'vercel_deploy' }, '  ')).toBe(true);
	});
});
