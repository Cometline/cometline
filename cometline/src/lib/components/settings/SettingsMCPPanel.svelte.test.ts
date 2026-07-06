// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from 'vitest';
import { fireEvent, render, waitFor } from '@testing-library/svelte';
import { flushSync } from 'svelte';
import Harness from './SettingsMCPPanel.harness.svelte';
import { startMcpOAuth } from '$lib/client/cometmind';

vi.mock('$lib/client/cometmind', () => ({
	listMcpServers: () => Promise.resolve([]),
	listMcpTools: () => Promise.resolve([]),
	reconnectMcpServer: vi.fn(),
	startMcpOAuth: vi.fn()
}));

afterEach(() => {
	vi.clearAllMocks();
});

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

describe('SettingsMCPPanel add server', () => {
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
		const persist = vi.fn(async () => {
			calls.push('save');
		});
		vi.mocked(startMcpOAuth).mockImplementation(async () => {
			calls.push('oauth');
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
		expect(calls).toEqual(['save', 'oauth']);
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
