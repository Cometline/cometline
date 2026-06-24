// @vitest-environment jsdom
import { describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen } from '@testing-library/svelte';
import RuntimeOverlay from './RuntimeOverlay.svelte';

describe('RuntimeOverlay', () => {
	it('shows connecting state copy', async () => {
		const { connectionState } = await import('$lib/stores/runtime.svelte');
		connectionState.reconnect();
		render(RuntimeOverlay);
		expect(screen.getByText('Starting CometMind…')).toBeTruthy();
	});

	it('shows error state with retry button', async () => {
		const { connectionState } = await import('$lib/stores/runtime.svelte');
		vi.spyOn(globalThis, 'fetch').mockRejectedValueOnce(new Error('Connection refused'));
		await connectionState.check();
		render(RuntimeOverlay);
		expect(screen.getByRole('alert')).toBeTruthy();
		expect(screen.getByText('Cannot reach CometMind')).toBeTruthy();
		expect(screen.getByRole('button', { name: /Retry connection/i })).toBeTruthy();
	});

	it('retries connection when retry button is clicked', async () => {
		const { connectionState } = await import('$lib/stores/runtime.svelte');
		vi.spyOn(globalThis, 'fetch').mockRejectedValueOnce(new Error('Connection refused'));
		await connectionState.check();
		const reconnectSpy = vi.spyOn(connectionState, 'reconnect');
		render(RuntimeOverlay);
		await fireEvent.click(screen.getByRole('button', { name: /Retry connection/i }));
		expect(reconnectSpy).toHaveBeenCalled();
	});
});
