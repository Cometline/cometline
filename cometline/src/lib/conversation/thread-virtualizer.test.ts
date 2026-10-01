import { describe, expect, it } from 'vitest';
import type { ChatItem } from '$lib/stores/chat.svelte';
import { groupThreadItemsIntoTurns } from './thread-turns';
import {
	THREAD_TURN_GAP,
	THREAD_TURN_ESTIMATE_MIN,
	THREAD_TURN_ESTIMATE_MAX,
	THREAD_MEGA_ASSISTANT_CHARS,
	computeVirtualWindow,
	estimateTurnHeight,
	isOversizedAssistantText,
	prefixOffsets,
	resolveTurnSizes,
	scrollDeltaForSizeChange,
	shouldSkipMegaMarkdownDuringHydration,
	totalHeightFromSizes,
	turnHasOversizedAssistant,
	virtualTurnEntries,
	virtualTurnEntriesWithForced,
	scrollTopAfterPrepend,
	visualColumnCount
} from './thread-virtualizer';

function turnsFrom(items: ChatItem[]) {
	return groupThreadItemsIntoTurns(items);
}

describe('estimateTurnHeight', () => {
	it('floors short turns and grows with assistant text', () => {
		const [short] = turnsFrom([
			{ id: 'u1', type: 'user', text: 'hi' },
			{ id: 'a1', type: 'assistant', text: 'ok' }
		]);
		const [long] = turnsFrom([
			{ id: 'u2', type: 'user', text: 'hi' },
			{ id: 'a2', type: 'assistant', text: 'x'.repeat(2000) }
		]);
		expect(estimateTurnHeight(short)).toBeGreaterThanOrEqual(THREAD_TURN_ESTIMATE_MIN);
		expect(estimateTurnHeight(long)).toBeGreaterThan(estimateTurnHeight(short));
	});

	it('accounts for tool and subagent rows', () => {
		const [plain] = turnsFrom([
			{ id: 'u1', type: 'user', text: 'go' },
			{ id: 'a1', type: 'assistant', text: 'done' }
		]);
		const [withTools] = turnsFrom([
			{ id: 'u1', type: 'user', text: 'go' },
			{ id: 'a1', type: 'assistant', text: 'done' },
			{ id: 't1', type: 'tool', toolName: 'read', input: {}, output: 'ok' },
			{
				id: 's1',
				type: 'subagent',
				childSessionId: 'c1',
				purpose: 'explore',
				agentName: 'explore',
				status: 'completed',
				progress: []
			}
		]);
		expect(estimateTurnHeight(withTools)).toBeGreaterThan(estimateTurnHeight(plain));
	});
});

describe('resolveTurnSizes', () => {
	it('prefers measured heights and applies active min-height', () => {
		const items: ChatItem[] = [
			{ id: 'u1', type: 'user', text: 'one' },
			{ id: 'a1', type: 'assistant', text: 'a' },
			{ id: 'u2', type: 'user', text: 'two' },
			{ id: 'a2', type: 'assistant', text: 'b' }
		];
		const list = turnsFrom(items);
		const sizes = resolveTurnSizes(
			list,
			{ u1: 400 },
			{ activePinnedUserId: 'u2', activeTurnMinHeight: 500 }
		);
		expect(sizes[0]).toBe(400);
		expect(sizes[1]).toBeGreaterThanOrEqual(500);
	});
});

describe('computeVirtualWindow', () => {
	it('returns an empty window for no turns', () => {
		expect(computeVirtualWindow([], 0, 600)).toEqual({
			start: 0,
			end: 0,
			offset: 0,
			totalHeight: 0
		});
	});

	it('only includes turns around the viewport plus overscan', () => {
		const sizes = Array.from({ length: 40 }, () => 100);
		const gap = THREAD_TURN_GAP;
		const window = computeVirtualWindow(sizes, 10 * (100 + gap), 250, 2, gap);
		expect(window.start).toBeGreaterThanOrEqual(8);
		expect(window.end).toBeLessThanOrEqual(16);
		expect(window.totalHeight).toBe(totalHeightFromSizes(sizes, gap));
		expect(window.offset).toBe(prefixOffsets(sizes, gap)[window.start]);
	});

	it('clamps to list bounds and covers the full list when short', () => {
		const sizes = [120, 200, 180];
		const window = computeVirtualWindow(sizes, 0, 2000, 8);
		expect(window.start).toBe(0);
		expect(window.end).toBe(3);
	});

	it('prefers the tail when viewport height is unknown', () => {
		const sizes = Array.from({ length: 20 }, () => 100);
		const window = computeVirtualWindow(sizes, 0, 0, 2);
		expect(window.end).toBe(20);
		expect(window.start).toBeGreaterThan(0);
		expect(window.start).toBeLessThan(20);
	});
});

describe('virtualTurnEntries', () => {
	it('pairs absolute offsets with the visible slice', () => {
		const turns = turnsFrom([
			{ id: 'u1', type: 'user', text: 'a' },
			{ id: 'a1', type: 'assistant', text: '1' },
			{ id: 'u2', type: 'user', text: 'b' },
			{ id: 'a2', type: 'assistant', text: '2' },
			{ id: 'u3', type: 'user', text: 'c' },
			{ id: 'a3', type: 'assistant', text: '3' }
		]);
		const sizes = [100, 100, 100];
		const window = {
			start: 1,
			end: 3,
			offset: 100 + THREAD_TURN_GAP,
			totalHeight: 300 + 2 * THREAD_TURN_GAP
		};
		const entries = virtualTurnEntries(turns, sizes, window);
		expect(entries).toHaveLength(2);
		expect(entries[0].item.id).toBe('u2');
		expect(entries[0].offset).toBe(100 + THREAD_TURN_GAP);
		expect(entries[1].item.id).toBe('u3');
	});
});

describe('scrollDeltaForSizeChange', () => {
	it('shifts scroll when a row above the viewport changes height', () => {
		expect(scrollDeltaForSizeChange(0, 500, 100, 180)).toBe(80);
		expect(scrollDeltaForSizeChange(0, 500, 180, 100)).toBe(-80);
	});

	it('does not shift for rows at or below the scroll top', () => {
		expect(scrollDeltaForSizeChange(500, 500, 100, 200)).toBe(0);
		expect(scrollDeltaForSizeChange(600, 500, 100, 200)).toBe(0);
		expect(scrollDeltaForSizeChange(0, 500, 100, 100)).toBe(0);
	});
});

describe('virtualTurnEntriesWithForced', () => {
	it('mounts forced indices without spanning the gap', () => {
		const turns = turnsFrom(
			Array.from({ length: 10 }, (_, i) => [
				{ id: `u${i}`, type: 'user' as const, text: `u${i}` },
				{ id: `a${i}`, type: 'assistant' as const, text: `a${i}` }
			]).flat()
		);
		const sizes = turns.map(() => 100);
		const window = computeVirtualWindow(sizes, 0, 150, 0);
		expect(window.end).toBeLessThan(turns.length);
		const entries = virtualTurnEntriesWithForced(turns, sizes, window, [turns.length - 1]);
		const indexes = entries.map((entry) => entry.index);
		expect(indexes).toContain(0);
		expect(indexes).toContain(turns.length - 1);
		// Must not mount the entire span just to include the tail.
		expect(entries.length).toBeLessThan(turns.length);
	});
});

describe('ChatThread find virtualization guard', () => {
	it('does not remount the full transcript when session find is open', async () => {
		const { readFileSync } = await import('node:fs');
		const { fileURLToPath } = await import('node:url');
		const chatThread = readFileSync(
			fileURLToPath(new URL('../components/chat/ChatThread.svelte', import.meta.url)),
			'utf8'
		);
		const virtualController = readFileSync(
			fileURLToPath(new URL('./thread-virtual.svelte.ts', import.meta.url)),
			'utf8'
		);
		expect(chatThread).not.toMatch(/sessionFind\.open[\s\S]{0,240}end:\s*turnSizes\.length/);
		expect(virtualController).not.toMatch(
			/getFindOpen\(\)[\s\S]{0,240}end:\s*turnSizes\.length/
		);
		expect(virtualController).toContain('getFindActiveTurnIndex');
		expect(virtualController).toContain('virtualTurnEntriesWithForced');
	});
});

describe('scrollTopAfterPrepend', () => {
	it('shifts scrollTop by the height delta so the viewport stays anchored', () => {
		expect(scrollTopAfterPrepend(120, 1000, 1400)).toBe(520);
	});

	it('does not move when height did not grow', () => {
		expect(scrollTopAfterPrepend(120, 1000, 1000)).toBe(120);
		expect(scrollTopAfterPrepend(120, 1000, 900)).toBe(120);
	});
});

describe('mega-turn estimates and hydration-only skip', () => {
	it('counts CJK characters as wider columns', () => {
		expect(visualColumnCount('hi')).toBe(2);
		expect(visualColumnCount('你好')).toBe(4);
		expect(visualColumnCount('a你b')).toBe(4);
	});

	it('grows mega estimates past the old 2400 soft ceiling', () => {
		const mega = '中'.repeat(8_000);
		const [turn] = turnsFrom([
			{ id: 'u1', type: 'user', text: 'hi' },
			{ id: 'a1', type: 'assistant', text: mega }
		]);
		const height = estimateTurnHeight(turn);
		expect(height).toBeGreaterThan(2400);
		expect(height).toBeLessThanOrEqual(THREAD_TURN_ESTIMATE_MAX);
		expect(isOversizedAssistantText(mega)).toBe(true);
		expect(turnHasOversizedAssistant(turn)).toBe(true);
	});

	it('does not treat short assistant text as mega', () => {
		expect(isOversizedAssistantText('hi')).toBe(false);
		expect(isOversizedAssistantText('x'.repeat(THREAD_MEGA_ASSISTANT_CHARS - 1))).toBe(false);
		expect(isOversizedAssistantText('x'.repeat(THREAD_MEGA_ASSISTANT_CHARS))).toBe(true);
	});

	it('skips mega Shiki only while hydrating — never scroll-gated after clear', () => {
		const mega = 'x'.repeat(THREAD_MEGA_ASSISTANT_CHARS);
		const turns = turnsFrom([
			{ id: 'u1', type: 'user', text: 'ask' },
			{ id: 'a1', type: 'assistant', text: mega },
			{ id: 'u2', type: 'user', text: 'hi' }
		]);
		expect(turns).toHaveLength(2);
		// During hydration: skip mega (including mega-above-hi and mega-as-last).
		expect(shouldSkipMegaMarkdownDuringHydration(turns[0], true)).toBe(true);
		expect(shouldSkipMegaMarkdownDuringHydration(turns[1], true)).toBe(false);
		// After hydration: always run full markdown — no scroll gate.
		expect(shouldSkipMegaMarkdownDuringHydration(turns[0], false)).toBe(false);
		expect(shouldSkipMegaMarkdownDuringHydration(turns[1], false)).toBe(false);
	});

	it('skips mega-as-last while hydrating, then allows full markdown after clear', () => {
		const mega = 'x'.repeat(THREAD_MEGA_ASSISTANT_CHARS);
		const turns = turnsFrom([
			{ id: 'u1', type: 'user', text: 'ask' },
			{ id: 'a1', type: 'assistant', text: mega }
		]);
		expect(shouldSkipMegaMarkdownDuringHydration(turns[0], true)).toBe(true);
		expect(shouldSkipMegaMarkdownDuringHydration(turns[0], false)).toBe(false);
	});
});
