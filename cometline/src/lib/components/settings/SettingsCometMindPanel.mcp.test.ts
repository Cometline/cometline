// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from 'vitest';
import { fireEvent, render, waitFor } from '@testing-library/svelte';
import { flushSync } from 'svelte';
import Harness from './SettingsCometMindPanel.mcp.harness.svelte';

vi.mock('$lib/client/cometmind', () => ({
	listSkills: () => Promise.resolve({ skills: [], errors: [] }),
	syncSkills: vi.fn(),
	deleteSkill: vi.fn(),
	exportSkill: vi.fn(),
	listMcpServers: () => Promise.resolve([]),
	listMcpTools: () => Promise.resolve([]),
	reconnectMcpServer: vi.fn(),
	startMcpOAuth: vi.fn(),
	testMcpServer: vi.fn()
}));

vi.mock('$lib/stores/shell.svelte', () => ({
	shellStore: { workspacePath: '/tmp/workspace' }
}));

afterEach(() => {
	vi.clearAllMocks();
});

describe('SettingsCometMindPanel MCP add server', () => {
	it('keeps the parent draft in sync when toggling MCP tools', async () => {
		const { container } = render(Harness);

		await waitFor(() => {
			expect(container.querySelector('[data-testid="mcp-enabled"]')?.textContent).toBe('false');
		});

		const toggle = [...container.querySelectorAll('button[role="switch"]')].find((button) =>
			button.getAttribute('aria-label')?.includes('Use MCP tools')
		) as HTMLButtonElement | undefined;
		expect(toggle).toBeTruthy();
		await fireEvent.click(toggle!);

		await waitFor(() => {
			expect(container.querySelector('[data-testid="mcp-enabled"]')?.textContent).toBe('true');
		});
	});

	it('shows the new server in the MCP list after Add server', async () => {
		const { container } = render(Harness);

		await waitFor(() => {
			expect(container.textContent).toContain('MCP servers');
		});

		const addButton = [...container.querySelectorAll('button')].find((button) =>
			button.textContent?.includes('Add server')
		);
		expect(addButton).toBeTruthy();

		flushSync(() => {
			addButton!.click();
		});

		await waitFor(() => {
			expect(container.querySelector('[data-testid="server-count"]')?.textContent).toBe('1');
			expect(container.textContent).toContain('MCP Server 1');
		});
	});
});
