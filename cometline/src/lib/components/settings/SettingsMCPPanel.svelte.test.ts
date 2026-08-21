// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from 'vitest';
import { fireEvent, render, waitFor } from '@testing-library/svelte';
import { flushSync } from 'svelte';
import Harness from './SettingsMCPPanel.harness.svelte';
import { listMcpServers, startMcpOAuth, testMcpServer } from '$lib/client/cometmind';
import type { CometMindMCPSettings } from '$lib/cometmind-settings';
import { defaultSettings } from '$lib/settings/schema';
import { settingsStore } from '$lib/stores/settings.svelte';
import type { ProviderSettings } from '$lib/types';

vi.mock('$lib/client/cometmind', () => ({
	apiErrorMessage: (error: unknown, fallback: string) => {
		if (error instanceof Error) return error.message;
		if (error && typeof error === 'object' && 'error_hint' in error) {
			const hint = error.error_hint;
			if (typeof hint === 'string') return hint;
		}
		return fallback;
	},
	listMcpServers: vi.fn(() => Promise.resolve([])),
	listMcpTools: () => Promise.resolve([]),
	reconnectMcpServer: vi.fn(),
	startMcpOAuth: vi.fn(),
	testMcpServer: vi.fn()
}));

afterEach(() => {
	vi.useRealTimers();
	vi.clearAllMocks();
	settingsStore.apply(defaultSettings());
});

/** Seeds settingsStore.settings.cometmind.mcp so resyncDraftFromPersistedSettings has something to pull. */
function seedPersistedMcp(mcp: CometMindMCPSettings) {
	const next: ProviderSettings = {
		...defaultSettings(),
		cometmind: { ...defaultSettings().cometmind, mcp }
	};
	settingsStore.apply(next);
}

function clickAddServer(container: HTMLElement) {
	const addButton = [...container.querySelectorAll('button')].find((button) =>
		button.textContent?.includes('Add server')
	);
	expect(addButton).toBeTruthy();
	flushSync(() => addButton!.click());
}

async function configureHttpServer(container: HTMLElement) {
	clickAddServer(container);
	await waitFor(() => {
		expect(container.querySelector('.mcp-server-editor')).toBeTruthy();
	});

	const transport = container.querySelector('select') as HTMLSelectElement | null;
	expect(transport).toBeTruthy();
	await fireEvent.change(transport!, { target: { value: 'http' } });

	const urlInput = [...container.querySelectorAll('input')].find((input) =>
		input.getAttribute('placeholder')?.startsWith('https://example.com')
	) as HTMLInputElement | undefined;
	expect(urlInput).toBeTruthy();
	await fireEvent.input(urlInput!, { target: { value: 'https://mcp.example.com/mcp' } });
}

function oauthButton(container: HTMLElement): HTMLButtonElement {
	const button = [...container.querySelectorAll('button')].find((candidate) =>
		candidate.textContent?.includes('Connect with OAuth')
	) as HTMLButtonElement | undefined;
	expect(button).toBeTruthy();
	return button!;
}

/** Finds the toolbar's refresh button, waiting out onMount's initial refresh
 *  (which flips the label to "Refreshing…" until it settles). */
async function waitForRefreshButton(container: HTMLElement): Promise<HTMLButtonElement> {
	let button: HTMLButtonElement | undefined;
	await waitFor(() => {
		button = [...container.querySelectorAll('button')].find((candidate) =>
			candidate.textContent?.includes('Refresh status')
		) as HTMLButtonElement | undefined;
		expect(button).toBeTruthy();
	});
	return button!;
}

describe('SettingsMCPPanel add server', () => {
	it('updates the draft when toggling MCP tools on and off', async () => {
		const { container } = render(Harness);

		await waitFor(() => {
			expect(container.querySelector('[data-testid="mcp-enabled"]')?.textContent).toBe('false');
		});

		const toggle = container.querySelector('button[role="switch"]') as HTMLButtonElement | null;
		expect(toggle).toBeTruthy();
		await fireEvent.click(toggle!);

		await waitFor(() => {
			expect(container.querySelector('[data-testid="mcp-enabled"]')?.textContent).toBe('true');
			expect(toggle!.getAttribute('aria-checked')).toBe('true');
		});

		await fireEvent.click(toggle!);

		await waitFor(() => {
			expect(container.querySelector('[data-testid="mcp-enabled"]')?.textContent).toBe('false');
			expect(toggle!.getAttribute('aria-checked')).toBe('false');
		});
	});

	it('adds a server and shows the expanded editor', async () => {
		const { container } = render(Harness);

		await waitFor(() => {
			expect(container.textContent).toContain('No servers configured yet');
		});

		clickAddServer(container);

		await waitFor(() => {
			expect(container.querySelector('[data-testid="server-count"]')?.textContent).toBe('1');
			expect(container.textContent).toContain('MCP Server 1');
		});

		expect(container.querySelector('.mcp-server-editor')).toBeTruthy();
	});

	it('keeps environment variable text while typing incomplete lines', async () => {
		const { container } = render(Harness);

		clickAddServer(container);

		await waitFor(() => {
			expect(container.querySelector('.mcp-server-editor')).toBeTruthy();
		});

		const envField = container.querySelector(
			'textarea'
		) as HTMLTextAreaElement | null;
		expect(envField).toBeTruthy();

		await fireEvent.input(envField!, { target: { value: 'MY_KEY' } });
		expect(envField!.value).toBe('MY_KEY');

		await fireEvent.input(envField!, { target: { value: 'MY_KEY=secret' } });
		expect(envField!.value).toBe('MY_KEY=secret');
	});

	it('assigns unique server ids when adding multiple servers', async () => {
		const { container } = render(Harness);

		clickAddServer(container);
		clickAddServer(container);
		clickAddServer(container);

		await waitFor(() => {
			expect(container.querySelector('[data-testid="server-count"]')?.textContent).toBe('3');
		});

		const ids = container.querySelector('[data-testid="server-ids"]')?.textContent?.split(',') ?? [];
		expect(ids).toHaveLength(3);
		expect(new Set(ids).size).toBe(3);
	});

	it('saves settings before starting OAuth for a draft server', async () => {
		const calls: string[] = [];
		const persist = vi.fn(async (_overrides?: { mcp: CometMindMCPSettings }) => {
			calls.push('save');
		});
		vi.mocked(startMcpOAuth).mockImplementation(async () => {
			calls.push('oauth');
			return { ok: true, connected: true };
		});
		const { container } = render(Harness, {
			props: { onPersistBeforeRuntimeAction: persist }
		});

		await configureHttpServer(container);
		await fireEvent.click(oauthButton(container));

		await waitFor(() => {
			expect(persist).toHaveBeenCalledTimes(1);
			expect(startMcpOAuth).toHaveBeenCalledTimes(1);
		});
		expect(persist.mock.calls[0]?.[0]?.mcp.enabled).toBe(true);
		expect(persist.mock.calls[0]?.[0]?.mcp.servers[0]?.url).toBe('https://mcp.example.com/mcp');
		expect(calls).toEqual(['save', 'oauth']);
	});

	it('shows a Test button that does not disable Refresh status', async () => {
		const { container } = render(Harness);
		await configureHttpServer(container);
		const testButton = [...container.querySelectorAll('button')].find(
			(button) => button.textContent?.trim() === 'Test'
		);
		expect(testButton).toBeTruthy();
		expect(testButton!.disabled).toBe(false);
		const refresh = await waitForRefreshButton(container);
		expect(refresh.disabled).toBe(false);
	});

	it('shows tools discovered by a successful connection test', async () => {
		vi.mocked(testMcpServer).mockResolvedValue({
			ok: true,
			tool_count: 2,
			tools: ['search', 'create']
		});
		const { container } = render(Harness);
		await configureHttpServer(container);
		const testButton = [...container.querySelectorAll('button')].find(
			(button) => button.textContent?.trim() === 'Test'
		);
		expect(testButton).toBeTruthy();
		await fireEvent.click(testButton!);

		await waitFor(() => {
			expect(container.textContent).toContain('Test ok · 2 tools');
			expect(container.textContent).toContain('search');
			expect(container.textContent).toContain('create');
		});
	});

	it('shows an actionable hint from a non-Error test rejection', async () => {
		vi.mocked(testMcpServer).mockRejectedValue({
			error: 'connect: context deadline exceeded',
			error_hint: 'The handshake timed out. Click Reconnect.'
		});
		const { container } = render(Harness);
		await configureHttpServer(container);
		const testButton = [...container.querySelectorAll('button')].find(
			(button) => button.textContent?.trim() === 'Test'
		);
		await fireEvent.click(testButton!);

		await waitFor(() => {
			expect(container.textContent).toContain('The handshake timed out. Click Reconnect.');
		});
	});

	it('does not start OAuth when pre-save fails', async () => {
		const persist = vi.fn(async () => {
			throw new Error('save failed');
		});
		const { container } = render(Harness, {
			props: { onPersistBeforeRuntimeAction: persist }
		});

		await configureHttpServer(container);
		await fireEvent.click(oauthButton(container));

		await waitFor(() => {
			expect(persist).toHaveBeenCalledTimes(1);
			expect(container.textContent).toContain('save failed');
		});
		expect(startMcpOAuth).not.toHaveBeenCalled();
	});
});

describe('SettingsMCPPanel refresh status resync (fix for stuck toggle)', () => {
	it('Refresh status re-syncs mcp.enabled from persisted settings when the draft reverted', async () => {
		// Simulate the reported bug: persisted settings have MCP enabled, but the
		// in-memory draft (e.g. after a discard or panel remount) reverted to
		// disabled. Before the fix, clicking Refresh status never looked at
		// persisted settings and so could never recover from this.
		seedPersistedMcp({ enabled: true, servers: [] });
		const { container } = render(Harness);

		await waitFor(() => {
			expect(container.querySelector('[data-testid="mcp-enabled"]')?.textContent).toBe('true');
		});
	});

	it('Refresh status pulls in a server enabled flag that changed on disk without touching an expanded (in-progress) server', async () => {
		const { container } = render(Harness);
		clickAddServer(container);
		await waitFor(() => {
			expect(container.querySelector('.mcp-server-editor')).toBeTruthy();
		});
		const serverId = container
			.querySelector('[data-testid="server-ids"]')
			?.textContent?.split(',')[0];
		expect(serverId).toBeTruthy();

		// Persisted settings now show this server as disabled (e.g. saved from
		// another window), but the user has it expanded/mid-edit here.
		seedPersistedMcp({
			enabled: true,
			servers: [
				{
					id: serverId!,
					name: 'Persisted Name',
					enabled: false,
					transport: 'stdio',
					command: '',
					args: [],
					env: {},
					url: '',
					headers: {}
				}
			]
		});

		const refreshButton = await waitForRefreshButton(container);
		await fireEvent.click(refreshButton);

		// Expanded server is being edited right now — resync must not clobber it.
		await waitFor(() => {
			expect(container.querySelector('[data-testid="mcp-enabled"]')?.textContent).toBe('true');
		});
		const displayNameInput = container.querySelector(
			'input[type="text"]'
		) as HTMLInputElement | null;
		expect(displayNameInput?.value).not.toBe('Persisted Name');
	});

	it('shows a reloading hint and badge while CometMind reports a server as reloading', async () => {
		// MCP is already enabled and saved (as it would be for this scenario to
		// be reachable in practice), then the user adds a new server locally.
		seedPersistedMcp({ enabled: true, servers: [] });
		const { container } = render(Harness);
		clickAddServer(container);
		await waitFor(() => {
			expect(container.querySelector('.mcp-server-editor')).toBeTruthy();
		});
		const serverId = container
			.querySelector('[data-testid="server-ids"]')
			?.textContent?.split(',')[0];
		expect(serverId).toBeTruthy();

		vi.mocked(listMcpServers).mockResolvedValue([
			{
				id: serverId!,
				name: 'MCP Server 1',
				enabled: true,
				transport: 'stdio',
				status: 'reloading',
				tool_count: 0
			}
		]);

		const refreshButton = await waitForRefreshButton(container);
		await fireEvent.click(refreshButton);

		await waitFor(() => {
			expect(container.textContent).toContain('CometMind is reloading MCP servers');
			expect(container.querySelector('.status-badge.pending')).toBeTruthy();
		});
	});

	it('auto-refreshes while reloading until the server becomes connected', async () => {
		seedPersistedMcp({
			enabled: true,
			servers: [
				{
					id: 'server-1',
					name: 'MCP Server 1',
					enabled: true,
					transport: 'stdio',
					command: 'false',
					args: [],
					env: {},
					url: '',
					headers: {}
				}
			]
		});
		vi.mocked(listMcpServers)
			.mockResolvedValueOnce([
				{
					id: 'server-1',
					name: 'MCP Server 1',
					enabled: true,
					transport: 'stdio',
					status: 'reloading',
					tool_count: 0
				}
			])
			.mockResolvedValue([
				{
					id: 'server-1',
					name: 'MCP Server 1',
					enabled: true,
					transport: 'stdio',
					status: 'connected',
					tool_count: 2
				}
			]);

		const { container } = render(Harness);

		await waitFor(() => {
			expect(container.textContent).toContain('CometMind is reloading MCP servers');
			expect(container.querySelector('.status-badge.pending')).toBeTruthy();
		});

		await waitFor(() => {
			expect(container.textContent).not.toContain('CometMind is reloading MCP servers');
			expect(container.querySelector('.status-badge.connected')).toBeTruthy();
			expect(container.textContent).toContain('connected');
		}, { timeout: 3_000 });
		expect(listMcpServers).toHaveBeenCalledTimes(2);
	});

	it('auto-refreshes while connecting until the server reaches a terminal state', async () => {
		seedPersistedMcp({
			enabled: true,
			servers: [
				{
					id: 'server-1',
					name: 'MCP Server 1',
					enabled: true,
					transport: 'http',
					command: '',
					args: [],
					env: {},
					url: 'https://mcp.example.com/mcp',
					headers: {}
				}
			]
		});
		vi.mocked(listMcpServers)
			.mockResolvedValueOnce([
				{
					id: 'server-1',
					name: 'MCP Server 1',
					enabled: true,
					transport: 'http',
					status: 'connecting',
					tool_count: 0
				}
			])
			.mockResolvedValue([
				{
					id: 'server-1',
					name: 'MCP Server 1',
					enabled: true,
					transport: 'http',
					status: 'connected',
					tool_count: 1
				}
			]);

		const { container } = render(Harness);
		await waitFor(() => expect(container.textContent).toContain('Connecting'));
		await waitFor(
			() => {
				expect(container.querySelector('.status-badge.connected')).toBeTruthy();
				expect(container.textContent).toContain('connected');
			},
			{ timeout: 3_000 }
		);
		expect(listMcpServers).toHaveBeenCalledTimes(2);
	});
});
