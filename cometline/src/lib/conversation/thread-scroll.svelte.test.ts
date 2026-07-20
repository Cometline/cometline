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
let clientHeight = 600;
let resizeObserverCallback: ResizeObserverCallback | null = null;

async function settleEffects() {
	await tick();
	await Promise.resolve();
}

describe('createThreadScroll', () => {
	beforeEach(() => {
		clientHeight = 600;
		resizeObserverCallback = null;
		vi.spyOn(HTMLElement.prototype, 'clientHeight', 'get').mockImplementation(
			() => clientHeight
		);
		scrollIntoView = vi.fn();
		Object.defineProperty(HTMLElement.prototype, 'scrollIntoView', {
			configurable: true,
			value: scrollIntoView
		});
		vi.stubGlobal('requestAnimationFrame', (callback: FrameRequestCallback) => {
			callback(0);
			return 1;
		});
		vi.stubGlobal(
			'ResizeObserver',
			class {
				constructor(callback: ResizeObserverCallback) {
					resizeObserverCallback = callback;
				}
				observe() {}
				disconnect() {}
				unobserve() {}
			}
		);
	});

	afterEach(() => {
		vi.unstubAllGlobals();
		vi.restoreAllMocks();
		Reflect.deleteProperty(HTMLElement.prototype, 'scrollIntoView');
	});

	it('keeps follow-up pinning after the response stops streaming', async () => {
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
			expect(screen.getByTestId('thread-scroll').dataset.activeMinHeight).toBe('504')
		);

		scrollIntoView.mockClear();
		await view.rerender({ items: refreshedFollowUpItems, streaming: false, cached: true });
		await settleEffects();
		await waitFor(() =>
			expect(screen.getByTestId('thread-scroll').dataset.activeMinHeight).toBe('504')
		);
		expect(screen.getByTestId('thread-scroll').dataset.activePinnedUserId).toBe('u2-server');
		expect(scrollIntoView).not.toHaveBeenCalled();
	});

	it('retries pin after streaming starts without consuming the user id early', async () => {
		const view = render(ThreadScrollHarness, {
			props: {
				items: initialItems,
				streaming: false,
				cached: true
			}
		});
		await settleEffects();
		await waitFor(() =>
			expect(screen.getByTestId('thread-scroll').dataset.initialPaint).toBe('false')
		);

		scrollIntoView.mockClear();
		await view.rerender({ items: followUpItems, streaming: false, cached: true });
		await settleEffects();

		expect(screen.getByTestId('thread-scroll').dataset.activePinnedUserId).toBe('');
		expect(scrollIntoView).not.toHaveBeenCalled();

		await view.rerender({ items: followUpItems, streaming: true, cached: true });
		await settleEffects();
		await waitFor(() =>
			expect(screen.getByTestId('thread-scroll').dataset.activePinnedUserId).toBe('u2')
		);
		expect(scrollIntoView).toHaveBeenCalled();
		expect(screen.getByTestId('thread-scroll').dataset.activeMinHeight).toBe('504');
	});

	it('retries pin once viewport height becomes available', async () => {
		clientHeight = 0;
		const view = render(ThreadScrollHarness, {
			props: {
				items: initialItems,
				streaming: false,
				cached: true
			}
		});
		await settleEffects();
		await waitFor(() =>
			expect(screen.getByTestId('thread-scroll').dataset.initialPaint).toBe('false')
		);

		scrollIntoView.mockClear();
		await view.rerender({ items: followUpItems, streaming: true, cached: true });
		await settleEffects();

		expect(screen.getByTestId('thread-scroll').dataset.viewportHeight).toBe('0');
		expect(screen.getByTestId('thread-scroll').dataset.activePinnedUserId).toBe('');
		expect(scrollIntoView).not.toHaveBeenCalled();

		clientHeight = 600;
		resizeObserverCallback?.([] as unknown as ResizeObserverEntry[], {} as ResizeObserver);
		await settleEffects();

		await waitFor(() =>
			expect(screen.getByTestId('thread-scroll').dataset.activePinnedUserId).toBe('u2')
		);
		expect(scrollIntoView).toHaveBeenCalled();
		expect(screen.getByTestId('thread-scroll').dataset.activeMinHeight).toBe('504');
	});

	it('finishes pin settle after streaming ends', async () => {
		const view = render(ThreadScrollHarness, {
			props: {
				items: initialItems,
				streaming: false,
				cached: true
			}
		});
		await settleEffects();
		await waitFor(() =>
			expect(screen.getByTestId('thread-scroll').dataset.initialPaint).toBe('false')
		);

		await view.rerender({ items: followUpItems, streaming: true, cached: true });
		await settleEffects();
		await waitFor(() =>
			expect(screen.getByTestId('thread-scroll').dataset.activePinnedUserId).toBe('u2')
		);

		scrollIntoView.mockClear();
		await view.rerender({ items: followUpItems, streaming: false, cached: true });
		await settleEffects();

		// Canvas stays; settle no longer aborts solely because streaming ended.
		expect(screen.getByTestId('thread-scroll').dataset.activeMinHeight).toBe('504');
		expect(screen.getByTestId('thread-scroll').dataset.activePinnedUserId).toBe('u2');
	});

	it('marks a synchronized empty transcript as ready for live turns', async () => {
		const view = render(ThreadScrollHarness, {
			props: {
				items: [],
				loading: true,
				synced: true,
				streaming: false,
				cached: false
			}
		});
		await settleEffects();

		expect(screen.getByTestId('thread-scroll').dataset.initialPaint).toBe('true');

		await view.rerender({ loading: false });
		await settleEffects();

		expect(screen.getByTestId('thread-scroll').dataset.initialPaint).toBe('false');
	});

	it('keeps a fresh session live when its first turn starts during transcript loading', async () => {
		const view = render(ThreadScrollHarness, {
			props: {
				items: [],
				loading: true,
				synced: true,
				streaming: false,
				cached: false
			}
		});
		await settleEffects();

		await view.rerender({
			items: [
				{ id: 'u1', type: 'user', text: 'first' },
				{ id: 'a1', type: 'assistant', text: '', pending: true }
			],
			streaming: true
		});
		await settleEffects();

		expect(screen.getByTestId('thread-scroll').dataset.initialPaint).toBe('false');
	});

	it('pins follow-ups on a fresh session instead of wiping the canvas while streaming', async () => {
		const view = render(ThreadScrollHarness, {
			props: {
				items: [],
				loading: false,
				synced: true,
				streaming: false,
				cached: false
			}
		});
		await settleEffects();
		expect(screen.getByTestId('thread-scroll').dataset.initialPaint).toBe('false');

		await view.rerender({
			items: [
				{ id: 'u1', type: 'user', text: 'first' },
				{ id: 'a1', type: 'assistant', text: '', pending: true }
			],
			streaming: true,
			cached: false
		});
		await settleEffects();

		scrollIntoView.mockClear();
		await view.rerender({
			items: [
				{ id: 'u1', type: 'user', text: 'first' },
				{ id: 'a1', type: 'assistant', text: 'done' },
				{ id: 'u2', type: 'user', text: 'follow up' },
				{ id: 'a2', type: 'assistant', text: '', pending: true }
			],
			streaming: true,
			cached: false
		});
		await settleEffects();

		await waitFor(() =>
			expect(screen.getByTestId('thread-scroll').dataset.activePinnedUserId).toBe('u2')
		);
		expect(screen.getByTestId('thread-scroll').dataset.activeMinHeight).toBe('504');
		expect(scrollIntoView).toHaveBeenCalled();

		// Streaming updates must not clear the active pin on a fresh session.
		await view.rerender({
			items: [
				{ id: 'u1', type: 'user', text: 'first' },
				{ id: 'a1', type: 'assistant', text: 'done' },
				{ id: 'u2', type: 'user', text: 'follow up' },
				{ id: 'a2', type: 'assistant', text: 'partial', pending: true }
			],
			streaming: true,
			cached: false
		});
		await settleEffects();

		expect(screen.getByTestId('thread-scroll').dataset.activePinnedUserId).toBe('u2');
		expect(screen.getByTestId('thread-scroll').dataset.activeMinHeight).toBe('504');
	});

	it('keeps the first live turn out of hydration after clearing the same session', async () => {
		const view = render(ThreadScrollHarness, {
			props: {
				items: initialItems,
				streaming: false,
				cached: true
			}
		});
		await settleEffects();
		await waitFor(() =>
			expect(screen.getByTestId('thread-scroll').dataset.initialPaint).toBe('false')
		);

		await view.rerender({ items: [], streaming: false, cached: false });
		await settleEffects();

		expect(screen.getByTestId('thread-scroll').dataset.initialPaint).toBe('false');

		await view.rerender({
			items: [
				{ id: 'u2', type: 'user', text: 'fresh turn' },
				{ id: 'a2', type: 'assistant', text: '', pending: true }
			],
			streaming: true,
			cached: true
		});
		await settleEffects();

		expect(screen.getByTestId('thread-scroll').dataset.initialPaint).toBe('false');
		expect(screen.getByTestId('thread-scroll').dataset.activeMinHeight).toBe('0');
	});
});
