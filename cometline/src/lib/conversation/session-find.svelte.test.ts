// @vitest-environment jsdom

import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { ChatItem } from '$lib/stores/chat.svelte';
import { createSessionFindController } from './session-find.svelte';
import { groupThreadItemsIntoTurns } from './thread-turns';

class TestHighlight {
	constructor(public readonly ranges: Range[]) {}
}

function turnsFrom(items: ChatItem[]) {
	return groupThreadItemsIntoTurns(items);
}

describe('createSessionFindController', () => {
	let root: HTMLDivElement;
	let highlights: Map<string, TestHighlight>;
	let items: ChatItem[];

	beforeEach(() => {
		highlights = new Map();
		vi.stubGlobal('Highlight', TestHighlight);
		vi.stubGlobal('CSS', {
			highlights: {
				set: (name: string, highlight: TestHighlight) => highlights.set(name, highlight),
				delete: (name: string) => highlights.delete(name)
			}
		});
		Object.defineProperty(HTMLElement.prototype, 'scrollIntoView', {
			configurable: true,
			value: vi.fn()
		});
		items = [
			{ id: 'u1', type: 'user', text: 'one match and another match' },
			{ id: 'a1', type: 'assistant', text: 'idle' }
		];
		root = document.createElement('div');
		root.innerHTML =
			'<div data-session-find-text data-session-find-item="u1">one match and another match</div>';
		document.body.replaceChildren(root);
	});

	afterEach(() => {
		vi.useRealTimers();
		vi.unstubAllGlobals();
	});

	it('opens, counts store matches, navigates with wrapping, and clears', () => {
		let controller!: ReturnType<typeof createSessionFindController>;
		const cleanup = $effect.root(() => {
			controller = createSessionFindController({
				getRoot: () => root,
				getTurns: () => turnsFrom(items)
			});
		});
		controller.openFind();
		controller.setQuery('match');

		expect(controller.open).toBe(true);
		expect(controller.matchCount).toBe(2);
		expect(controller.activeIndex).toBe(0);
		expect(controller.activeTurnIndex).toBe(0);
		expect(controller.activeItemId).toBe('u1');
		expect(highlights.has('session-find-match')).toBe(true);
		expect(highlights.has('session-find-active')).toBe(true);

		controller.previous();
		expect(controller.activeIndex).toBe(1);
		controller.next();
		expect(controller.activeIndex).toBe(0);

		controller.closeFind({ restoreFocus: false });
		expect(controller.open).toBe(false);
		expect(controller.query).toBe('');
		expect(controller.matchCount).toBe(0);
		expect(controller.activeTurnIndex).toBe(-1);
		expect(highlights.size).toBe(0);
		cleanup();
	});

	it('counts matches from the store even when the hit turn is not mounted', () => {
		items = [
			{ id: 'u1', type: 'user', text: 'alpha' },
			{ id: 'a1', type: 'assistant', text: 'first needle' },
			{ id: 'u2', type: 'user', text: 'beta' },
			{ id: 'a2', type: 'assistant', text: 'second needle' }
		];
		// Only the first turn is in the DOM (virtualized offscreen later turns).
		root.innerHTML =
			'<div data-session-find-text data-session-find-item="a1">first needle</div>';

		let controller!: ReturnType<typeof createSessionFindController>;
		const cleanup = $effect.root(() => {
			controller = createSessionFindController({
				getRoot: () => root,
				getTurns: () => turnsFrom(items)
			});
		});
		controller.openFind();
		controller.setQuery('needle');

		expect(controller.matchCount).toBe(2);
		expect(controller.activeTurnIndex).toBe(0);
		controller.next();
		expect(controller.activeTurnIndex).toBe(1);
		expect(controller.activeItemId).toBe('a2');
		cleanup();
	});

	it('reindexes store text on the throttled refresh interval', async () => {
		vi.useFakeTimers();
		let controller!: ReturnType<typeof createSessionFindController>;
		const cleanup = $effect.root(() => {
			controller = createSessionFindController({
				getRoot: () => root,
				getTurns: () => turnsFrom(items)
			});
		});
		controller.openFind();
		controller.setQuery('match');
		const disconnect = controller.observe();

		items = [{ id: 'u1', type: 'user', text: 'one match and another match match' }];
		root.querySelector('[data-session-find-text]')?.append(' match');
		await Promise.resolve();
		await vi.advanceTimersByTimeAsync(120);

		expect(controller.matchCount).toBe(3);
		disconnect();
		cleanup();
	});

	it('expands a collapsed user message before scrolling to its active match', async () => {
		items = [{ id: 'u1', type: 'user', text: 'hidden match' }];
		root.innerHTML = `
			<div data-session-find-text data-session-find-item="u1">
				<div data-user-message-viewport>hidden match</div>
				<button data-user-message-expand aria-expanded="false">Expand</button>
			</div>
		`;
		const expand = root.querySelector<HTMLButtonElement>('[data-user-message-expand]')!;
		const message = root.querySelector<HTMLElement>('[data-session-find-text]')!;
		const order: string[] = [];
		vi.spyOn(expand, 'click').mockImplementation(() => order.push('expand'));
		vi.mocked(HTMLElement.prototype.scrollIntoView).mockImplementation(function (
			this: HTMLElement
		) {
			if (this === message) order.push('scroll');
		});
		let controller!: ReturnType<typeof createSessionFindController>;
		const cleanup = $effect.root(() => {
			controller = createSessionFindController({
				getRoot: () => root,
				getTurns: () => turnsFrom(items)
			});
		});

		controller.openFind();
		controller.setQuery('hidden');
		await vi.waitFor(() => expect(order).toEqual(['expand', 'scroll']));

		cleanup();
	});
});
