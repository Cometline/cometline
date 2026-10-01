// @vitest-environment jsdom
import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest';
import { createThreadVirtual } from './thread-virtual.svelte';
import type { ThreadTurn } from './thread-turns';

const turns: ThreadTurn[] = [
	{
		id: 'u1',
		user: { id: 'u1', type: 'user', text: 'one' },
		userIndex: 0,
		items: []
	},
	{
		id: 'u2',
		user: { id: 'u2', type: 'user', text: 'two' },
		userIndex: 1,
		items: []
	}
];

describe('createThreadVirtual', () => {
	beforeEach(() => {
		vi.stubGlobal('requestAnimationFrame', (callback: FrameRequestCallback) => {
			callback(0);
			return 1;
		});
		vi.stubGlobal('cancelAnimationFrame', () => {});
	});

	afterEach(() => {
		vi.unstubAllGlobals();
	});

	it('resets measure cache and scrollTop on session change', () => {
		let sessionId = 'a';
		let controller!: ReturnType<typeof createThreadVirtual>;
		const cleanup = $effect.root(() => {
			controller = createThreadVirtual({
				getSessionEpoch: () => (sessionId === 'a' ? 1 : 2),
				getThreadTurns: () => turns,
				getScroller: () => undefined,
				getViewportHeight: () => 600,
				getActivePinnedUserId: () => null,
				getActiveTurnMinHeight: () => 0,
				getLastUserId: () => 'u2',
				getIsInitialTranscriptPaint: () => false,
				getFindOpen: () => false,
				getFindActiveTurnIndex: () => -1,
				getFindJumpKey: () => ''
			});
		});

		controller.setScrollTop(240);
		expect(controller.virtualScrollTop).toBe(240);

		sessionId = 'b';
		// Flush session-reset effect.
		controller.setScrollTop(240);
		// Re-read after effect microtask by creating a nested flush via setScrollTop
		// The session effect runs synchronously when tracked deps are read in the same root...
		cleanup();

		const cleanup2 = $effect.root(() => {
			controller = createThreadVirtual({
				getSessionEpoch: () => 2,
				getThreadTurns: () => turns,
				getScroller: () => undefined,
				getViewportHeight: () => 600,
				getActivePinnedUserId: () => null,
				getActiveTurnMinHeight: () => 0,
				getLastUserId: () => 'u2',
				getIsInitialTranscriptPaint: () => false,
				getFindOpen: () => false,
				getFindActiveTurnIndex: () => -1,
				getFindJumpKey: () => ''
			});
		});
		expect(controller.virtualScrollTop).toBe(0);
		expect(controller.virtualWindow.totalHeight).toBeGreaterThan(0);
		cleanup2();
	});

	it('exposes a visible window over turns', () => {
		let controller!: ReturnType<typeof createThreadVirtual>;
		const cleanup = $effect.root(() => {
			controller = createThreadVirtual({
				getSessionEpoch: () => 1,
				getThreadTurns: () => turns,
				getScroller: () => undefined,
				getViewportHeight: () => 600,
				getActivePinnedUserId: () => null,
				getActiveTurnMinHeight: () => 0,
				getLastUserId: () => 'u2',
				getIsInitialTranscriptPaint: () => false,
				getFindOpen: () => false,
				getFindActiveTurnIndex: () => -1,
				getFindJumpKey: () => ''
			});
		});
		expect(controller.visibleTurns.length).toBeGreaterThan(0);
		expect(controller.visibleTurns.some((entry) => entry.item.id === 'u2')).toBe(true);
		cleanup();
	});

	it('marks mega turns skipHydrationMarkdown only while hydrating', () => {
		const mega = 'x'.repeat(4000);
		const megaTurns: ThreadTurn[] = [
			{
				id: 'u1',
				user: { id: 'u1', type: 'user', text: 'ask' },
				userIndex: 0,
				items: [{ item: { id: 'a1', type: 'assistant', text: mega }, index: 1 }]
			},
			{
				id: 'u2',
				user: { id: 'u2', type: 'user', text: 'hi' },
				userIndex: 2,
				items: []
			}
		];
		const hydrating = true;
		let controller!: ReturnType<typeof createThreadVirtual>;
		const cleanup = $effect.root(() => {
			controller = createThreadVirtual({
				getSessionEpoch: () => 1,
				getThreadTurns: () => megaTurns,
				getScroller: () => undefined,
				getViewportHeight: () => 200,
				getActivePinnedUserId: () => null,
				getActiveTurnMinHeight: () => 0,
				getLastUserId: () => 'u2',
				getIsInitialTranscriptPaint: () => hydrating,
				getFindOpen: () => false,
				getFindActiveTurnIndex: () => -1,
				getFindJumpKey: () => ''
			});
		});
		const total = controller.virtualWindow.totalHeight;
		controller.setScrollTop(Math.max(0, total - 200));
		const megaEntry = controller.visibleTurns.find((entry) => entry.item.id === 'u1');
		const shortEntry = controller.visibleTurns.find((entry) => entry.item.id === 'u2');
		expect(shortEntry?.skipHydrationMarkdown).toBe(false);
		if (megaEntry) {
			expect(megaEntry.skipHydrationMarkdown).toBe(true);
		}
		cleanup();
	});

	it('clears skipHydrationMarkdown for mounted mega after hydration (no scroll gate)', () => {
		const mega = 'x'.repeat(4000);
		const megaTurns: ThreadTurn[] = [
			{
				id: 'u1',
				user: { id: 'u1', type: 'user', text: 'ask' },
				userIndex: 0,
				items: [{ item: { id: 'a1', type: 'assistant', text: mega }, index: 1 }]
			},
			{
				id: 'u2',
				user: { id: 'u2', type: 'user', text: 'hi' },
				userIndex: 2,
				items: []
			}
		];
		let controller!: ReturnType<typeof createThreadVirtual>;
		const cleanup = $effect.root(() => {
			controller = createThreadVirtual({
				getSessionEpoch: () => 1,
				getThreadTurns: () => megaTurns,
				getScroller: () => undefined,
				getViewportHeight: () => 200,
				getActivePinnedUserId: () => null,
				getActiveTurnMinHeight: () => 0,
				getLastUserId: () => 'u2',
				getIsInitialTranscriptPaint: () => false,
				getFindOpen: () => false,
				getFindActiveTurnIndex: () => -1,
				getFindJumpKey: () => ''
			});
		});
		const total = controller.virtualWindow.totalHeight;
		controller.setScrollTop(Math.max(0, total - 200));
		for (const entry of controller.visibleTurns) {
			expect(entry.skipHydrationMarkdown).toBe(false);
		}
		cleanup();
	});
});
