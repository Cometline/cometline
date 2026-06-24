// @vitest-environment jsdom
import { describe, expect, it } from 'vitest';
import { render, screen } from '@testing-library/svelte';
import RuntimeOverlay from './RuntimeOverlay.svelte';

describe('RuntimeOverlay', () => {
	it('shows connecting state copy', async () => {
		const { connectionState } = await import('$lib/stores/runtime.svelte');
		connectionState.reconnect();
		render(RuntimeOverlay);
		expect(screen.getByText('Starting CometMind…')).toBeTruthy();
	});
});
