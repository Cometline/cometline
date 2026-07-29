// @vitest-environment jsdom
import { describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen } from '@testing-library/svelte';
import RuntimeOverlay from './RuntimeOverlay.svelte';

/** Exhaust the connecting grace budget so failed health checks surface as error. */
async function exhaustConnectingGrace(
	connectionState: { check: () => Promise<void> },
	attempts = 30
) {
	for (let i = 0; i < attempts; i++) {
		await connectionState.check();
	}
}

describe('RuntimeOverlay', () => {
	it('shows connecting state copy', async () => {
		const { connectionState } = await import('$lib/stores/runtime.svelte');
		connectionState.reconnect();
		render(RuntimeOverlay);
		expect(screen.getByText('Starting CometMind…')).toBeTruthy();
	});

	it('stays on connecting UI after a single failed health check', async () => {
		const { connectionState } = await import('$lib/stores/runtime.svelte');
		vi.spyOn(globalThis, 'fetch').mockRejectedValue(new Error('Failed to fetch'));
		await connectionState.check();
		render(RuntimeOverlay);
		expect(screen.getByText('Starting CometMind…')).toBeTruthy();
		expect(screen.queryByRole('alert')).toBeNull();
	});

	it('shows error state with retry button after grace budget', async () => {
		const { connectionState } = await import('$lib/stores/runtime.svelte');
		vi.spyOn(globalThis, 'fetch').mockRejectedValue(new Error('Connection refused'));
		await exhaustConnectingGrace(connectionState);
		render(RuntimeOverlay);
		expect(screen.getByRole('alert')).toBeTruthy();
		expect(screen.getByText('Cannot reach CometMind')).toBeTruthy();
		expect(screen.getByRole('button', { name: /Retry connection/i })).toBeTruthy();
	});

	it('retries connection when retry button is clicked', async () => {
		const { connectionState } = await import('$lib/stores/runtime.svelte');
		vi.spyOn(globalThis, 'fetch').mockRejectedValue(new Error('Connection refused'));
		await exhaustConnectingGrace(connectionState);
		const reconnectSpy = vi.spyOn(connectionState, 'reconnect');
		render(RuntimeOverlay);
		await fireEvent.click(screen.getByRole('button', { name: /Retry connection/i }));
		expect(reconnectSpy).toHaveBeenCalled();
	});
});
