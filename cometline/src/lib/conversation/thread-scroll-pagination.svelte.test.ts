// @vitest-environment jsdom

import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { tick } from 'svelte';
import { createThreadScroll } from './thread-scroll.svelte';
import type { ChatItem } from '$lib/stores/chat.svelte';

const items: ChatItem[] = [
	{ id: 'u1', type: 'user', text: 'hello' },
	{ id: 'a1', type: 'assistant', text: 'world' }
];

async function flush() {
	await tick();
	await Promise.resolve();
	await Promise.resolve();
}

describe('createThreadScroll maybeLoadOlderHistory', () => {
	beforeEach(() => {
		vi.stubGlobal('requestAnimationFrame', (callback: FrameRequestCallback) => {
			callback(0);
			return 1;
		});
		vi.stubGlobal(
			'ResizeObserver',
			class {
				observe() {}
				disconnect() {}
				unobserve() {}
			}
		);
	});

	afterEach(() => {
		vi.unstubAllGlobals();
	});

	it('prepends older history near the top and restores scrollTop after tick', async () => {
		let height = 1000;
		const loadOlderTranscript = vi.fn(async () => {
			height = 1400;
			return 3;
		});
		const onScrollTopChange = vi.fn();
		let currentItems: ChatItem[] = [];
		const loading = false;

		let scroll!: ReturnType<typeof createThreadScroll>;
		const cleanup = $effect.root(() => {
			scroll = createThreadScroll({
				getSessionId: () => 'sess-1',
				getIsSessionSynced: () => true,
				getThreadItems: () => currentItems,
				getSessionStreaming: () => false,
				getLastUserId: () => currentItems.findLast((item) => item.type === 'user')?.id ?? null,
				getUserMessageCount: () =>
					currentItems.reduce((count, item) => (item.type === 'user' ? count + 1 : count), 0),
				getIsLoading: () => loading,
				sessionHasCachedTranscript: () => false,
				getHasMoreHistory: () => true,
				getIsLoadingOlder: () => false,
				loadOlderTranscript,
				onScrollTopChange
			});
		});

		// Empty synced transcript marks paint complete (live-ready).
		await flush();
		expect(scroll.isInitialTranscriptPaint).toBe(false);

		currentItems = items;
		await flush();

		const scroller = document.createElement('div');
		Object.defineProperty(scroller, 'scrollTop', {
			configurable: true,
			writable: true,
			value: 120
		});
		Object.defineProperty(scroller, 'scrollHeight', {
			configurable: true,
			get: () => height
		});
		Object.defineProperty(scroller, 'clientHeight', {
			configurable: true,
			get: () => 600
		});
		scroll.setScroller(scroller);
		await flush();

		scroller.scrollTop = 120;
		await scroll.maybeLoadOlderHistory();

		expect(loadOlderTranscript).toHaveBeenCalledWith('sess-1');
		expect(scroller.scrollTop).toBe(520);
		expect(onScrollTopChange).toHaveBeenCalledWith(520);
		cleanup();
	});

	it('skips when scrollTop is below the near-top threshold', async () => {
		const loadOlderTranscript = vi.fn(async () => 1);
		let currentItems: ChatItem[] = [];

		let scroll!: ReturnType<typeof createThreadScroll>;
		const cleanup = $effect.root(() => {
			scroll = createThreadScroll({
				getSessionId: () => 'sess-1',
				getIsSessionSynced: () => true,
				getThreadItems: () => currentItems,
				getSessionStreaming: () => false,
				getLastUserId: () => null,
				getUserMessageCount: () => 0,
				getIsLoading: () => false,
				sessionHasCachedTranscript: () => false,
				getHasMoreHistory: () => true,
				getIsLoadingOlder: () => false,
				loadOlderTranscript
			});
		});

		await flush();
		expect(scroll.isInitialTranscriptPaint).toBe(false);
		currentItems = items;
		await flush();

		const scroller = document.createElement('div');
		Object.defineProperty(scroller, 'scrollTop', {
			configurable: true,
			writable: true,
			value: 400
		});
		Object.defineProperty(scroller, 'scrollHeight', {
			configurable: true,
			get: () => 1000
		});
		Object.defineProperty(scroller, 'clientHeight', {
			configurable: true,
			get: () => 600
		});
		scroll.setScroller(scroller);
		await flush();

		scroller.scrollTop = 400;
		await scroll.maybeLoadOlderHistory();
		expect(loadOlderTranscript).not.toHaveBeenCalled();
		cleanup();
	});

	it('skips while store reports an in-flight older load', async () => {
		const loadOlderTranscript = vi.fn(async () => 1);
		let currentItems: ChatItem[] = [];

		let scroll!: ReturnType<typeof createThreadScroll>;
		const cleanup = $effect.root(() => {
			scroll = createThreadScroll({
				getSessionId: () => 'sess-1',
				getIsSessionSynced: () => true,
				getThreadItems: () => currentItems,
				getSessionStreaming: () => false,
				getLastUserId: () => null,
				getUserMessageCount: () => 0,
				getIsLoading: () => false,
				sessionHasCachedTranscript: () => false,
				getHasMoreHistory: () => true,
				getIsLoadingOlder: () => true,
				loadOlderTranscript
			});
		});

		await flush();
		currentItems = items;
		await flush();

		const scroller = document.createElement('div');
		Object.defineProperty(scroller, 'scrollTop', {
			configurable: true,
			writable: true,
			value: 40
		});
		Object.defineProperty(scroller, 'scrollHeight', {
			configurable: true,
			get: () => 1000
		});
		Object.defineProperty(scroller, 'clientHeight', {
			configurable: true,
			get: () => 600
		});
		scroll.setScroller(scroller);
		await scroll.maybeLoadOlderHistory();
		expect(loadOlderTranscript).not.toHaveBeenCalled();
		cleanup();
	});
});
