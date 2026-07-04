// @vitest-environment jsdom
import { render, screen, waitFor } from '@testing-library/svelte';
import { tick } from 'svelte';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { ChatItem } from '$lib/stores/chat.svelte';
import ThreadScrollHarness from './ThreadScrollHarness.svelte';

const initialItems: ChatItem[] = [
	{ id: 'u1', type: 'user', text: 'first' },
	{ id: 'a1', type: 'assistant', text: 'first response' }
];

const followUpItems: ChatItem[] = [
	...initialItems,
	{ id: 'u2', type: 'user', text: 'follow up' },
	{ id: 'a2', type: 'assistant', text: '', pending: true }
];

const refreshedFollowUpItems: ChatItem[] = [
	{ id: 'u1-server', type: 'user', text: 'first' },
	{ id: 'a1-server', type: 'assistant', text: 'first response' },
	{ id: 'u2-server', type: 'user', text: 'follow up' },
	{ id: 'a2-server', type: 'assistant', text: 'done', pending: false }
];

let scrollIntoView: ReturnType<typeof vi.fn>;

async function settleEffects() {
	await tick();
	await Promise.resolve();
}

describe('createThreadScroll', () => {
	beforeEach(() => {
		vi.spyOn(HTMLElement.prototype, 'clientHeight', 'get').mockReturnValue(600);
		scrollIntoView = vi.fn();
		Object.defineProperty(HTMLElement.prototype, 'scrollIntoView', {
			configurable: true,
			value: scrollIntoView
		});
		vi.stubGlobal('requestAnimationFrame', (callback: FrameRequestCallback) => {
			callback(0);
			return 1;
		});
	});

	afterEach(() => {
		vi.unstubAllGlobals();
		vi.restoreAllMocks();
		Reflect.deleteProperty(HTMLElement.prototype, 'scrollIntoView');
	});

	it('releases follow-up pinning when the response stops streaming', async () => {
		const view = render(ThreadScrollHarness, {
			props: {
				items: initialItems,
				streaming: false,
				cached: true
			}
		});
		await settleEffects();

		await view.rerender({ items: followUpItems, streaming: true, cached: true });
		await settleEffects();
		await waitFor(() =>
			expect(screen.getByTestId('thread-scroll').dataset.activeMinHeight).toBe('504')
		);

		await view.rerender({ items: followUpItems, streaming: false, cached: true });
		await settleEffects();
		await waitFor(() =>
			expect(screen.getByTestId('thread-scroll').dataset.activeMinHeight).toBe('0')
		);

		scrollIntoView.mockClear();
		await view.rerender({ items: refreshedFollowUpItems, streaming: false, cached: true });
		await settleEffects();
		await waitFor(() =>
			expect(screen.getByTestId('thread-scroll').dataset.activeMinHeight).toBe('0')
		);
		expect(scrollIntoView).not.toHaveBeenCalled();
	});
});
