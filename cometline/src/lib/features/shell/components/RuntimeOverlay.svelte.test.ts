// @vitest-environment jsdom
import { describe, expect, it, vi } from 'vitest';
import { render, screen } from '@testing-library/svelte';
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
		const { connectionState } = await import('#lib/stores/runtime.svelte.js');
		const { shellStore } = await import('#lib/stores/shell.svelte.js');
		shellStore.closeIntro();
		connectionState.reconnect();
		render(RuntimeOverlay);
		expect(screen.getByText('Starting CometMind…')).toBeTruthy();
	});

	it('stays on connecting UI after a single failed health check', async () => {
		const { connectionState } = await import('#lib/stores/runtime.svelte.js');
		const { shellStore } = await import('#lib/stores/shell.svelte.js');
		shellStore.closeIntro();
		vi.spyOn(globalThis, 'fetch').mockRejectedValue(new Error('Failed to fetch'));
		await connectionState.check();
		render(RuntimeOverlay);
		expect(screen.getByText('Starting CometMind…')).toBeTruthy();
		expect(screen.queryByRole('alert')).toBeNull();
	});

	it('hides while the first-run intro is open', async () => {
		const { connectionState } = await import('#lib/stores/runtime.svelte.js');
		const { shellStore } = await import('#lib/stores/shell.svelte.js');
		shellStore.openIntro();
		connectionState.reconnect();
		render(RuntimeOverlay);
		expect(screen.queryByText('Starting CometMind…')).toBeNull();
	});

	it('does not block the UI with an error card after grace budget', async () => {
		const { connectionState } = await import('#lib/stores/runtime.svelte.js');
		const { shellStore } = await import('#lib/stores/shell.svelte.js');
		shellStore.closeIntro();
		vi.spyOn(globalThis, 'fetch').mockRejectedValue(new Error('Connection refused'));
		await exhaustConnectingGrace(connectionState);
		render(RuntimeOverlay);
		expect(screen.queryByRole('alert')).toBeNull();
		expect(screen.queryByText('Cannot reach CometMind')).toBeNull();
		expect(screen.queryByText('Starting CometMind…')).toBeNull();
	});
});
