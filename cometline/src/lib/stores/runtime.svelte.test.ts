import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest';

describe('connectionState', () => {
	beforeEach(() => {
		vi.resetModules();
		vi.stubGlobal('fetch', vi.fn());
		vi.useFakeTimers();
	});

	afterEach(() => {
		vi.useRealTimers();
		vi.unstubAllGlobals();
	});

	it('sets ready when health check succeeds', async () => {
		vi.mocked(fetch).mockResolvedValue(new Response('ok', { status: 200 }));
		const { connectionState } = await import('./runtime.svelte');
		await connectionState.check();
		expect(connectionState.status).toBe('ready');
		expect(connectionState.message).toBe('');
	});

	it('stays connecting on the first failed health check (cold start)', async () => {
		vi.mocked(fetch).mockRejectedValue(new Error('Failed to fetch'));
		const { connectionState } = await import('./runtime.svelte');
		await connectionState.check();
		expect(connectionState.status).toBe('connecting');
		expect(connectionState.message).toBe('Failed to fetch');
	});

	it('stays connecting on non-ok health responses during startup', async () => {
		vi.mocked(fetch).mockResolvedValue(new Response('fail', { status: 503 }));
		const { connectionState } = await import('./runtime.svelte');
		await connectionState.check();
		expect(connectionState.status).toBe('connecting');
		expect(connectionState.message).toContain('503');
	});

	it('escalates to error after the connecting grace budget is exhausted', async () => {
		vi.mocked(fetch).mockRejectedValue(new Error('Failed to fetch'));
		const { connectionState } = await import('./runtime.svelte');
		// CONNECTING_GRACE_ATTEMPTS is 30
		for (let i = 0; i < 29; i++) {
			await connectionState.check();
			expect(connectionState.status).toBe('connecting');
		}
		await connectionState.check();
		expect(connectionState.status).toBe('error');
		expect(connectionState.message).toBe('Failed to fetch');
	});

	it('sets error immediately when a previously-ready connection drops', async () => {
		vi.mocked(fetch)
			.mockResolvedValueOnce(new Response('ok', { status: 200 }))
			.mockRejectedValueOnce(new Error('Failed to fetch'));
		const { connectionState } = await import('./runtime.svelte');
		await connectionState.check();
		expect(connectionState.status).toBe('ready');
		await connectionState.check();
		expect(connectionState.status).toBe('error');
		expect(connectionState.message).toBe('Failed to fetch');
	});

	it('reconnect resets to connecting and clears the failure budget', async () => {
		vi.mocked(fetch).mockRejectedValue(new Error('down'));
		const { connectionState } = await import('./runtime.svelte');
		for (let i = 0; i < 30; i++) {
			await connectionState.check();
		}
		expect(connectionState.status).toBe('error');
		connectionState.reconnect();
		expect(connectionState.status).toBe('connecting');
		expect(connectionState.message).toBe('');
		// First failure after reconnect should not immediately re-error
		await vi.advanceTimersByTimeAsync(0);
		await Promise.resolve();
	});

	it('recovers to ready after initial fetch failures during startup', async () => {
		vi.mocked(fetch)
			.mockRejectedValueOnce(new Error('Failed to fetch'))
			.mockRejectedValueOnce(new Error('Failed to fetch'))
			.mockResolvedValueOnce(new Response('ok', { status: 200 }));
		const { connectionState } = await import('./runtime.svelte');
		await connectionState.check();
		expect(connectionState.status).toBe('connecting');
		expect(connectionState.message).toBe('Failed to fetch');
		await connectionState.check();
		expect(connectionState.status).toBe('connecting');
		await connectionState.check();
		expect(connectionState.status).toBe('ready');
		expect(connectionState.message).toBe('');
	});
});
