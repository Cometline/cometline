import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest';

describe('connectionState', () => {
	beforeEach(() => {
		vi.resetModules();
		vi.stubGlobal('fetch', vi.fn());
	});

	afterEach(() => {
		vi.unstubAllGlobals();
	});

	it('sets ready when health check succeeds', async () => {
		vi.mocked(fetch).mockResolvedValue(new Response('ok', { status: 200 }));
		const { connectionState } = await import('./runtime.svelte');
		await connectionState.check();
		expect(connectionState.status).toBe('ready');
		expect(connectionState.message).toBe('');
	});

	it('sets error with message when health check fails', async () => {
		vi.mocked(fetch).mockResolvedValue(new Response('fail', { status: 503 }));
		const { connectionState } = await import('./runtime.svelte');
		await connectionState.check();
		expect(connectionState.status).toBe('error');
		expect(connectionState.message).toContain('503');
	});

	it('sets error when fetch throws', async () => {
		vi.mocked(fetch).mockRejectedValue(new Error('Connection refused'));
		const { connectionState } = await import('./runtime.svelte');
		await connectionState.check();
		expect(connectionState.status).toBe('error');
		expect(connectionState.message).toBe('Connection refused');
	});

	it('reconnect resets to connecting', async () => {
		vi.mocked(fetch).mockRejectedValue(new Error('down'));
		const { connectionState } = await import('./runtime.svelte');
		await connectionState.check();
		expect(connectionState.status).toBe('error');
		connectionState.reconnect();
		expect(connectionState.status).toBe('connecting');
		expect(connectionState.message).toBe('');
	});
});
