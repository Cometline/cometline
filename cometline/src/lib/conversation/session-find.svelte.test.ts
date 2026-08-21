// @vitest-environment jsdom

import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { createSessionFindController } from './session-find.svelte';

class TestHighlight {
	constructor(public readonly ranges: Range[]) {}
}

describe('createSessionFindController', () => {
	let root: HTMLDivElement;
	let highlights: Map<string, TestHighlight>;

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
		root = document.createElement('div');
		root.innerHTML = '<div data-session-find-text>one match and another match</div>';
		document.body.replaceChildren(root);
	});

	afterEach(() => {
		vi.useRealTimers();
		vi.unstubAllGlobals();
	});

	it('opens, counts matches, navigates with wrapping, and clears', () => {
		let controller!: ReturnType<typeof createSessionFindController>;
		const cleanup = $effect.root(() => {
			controller = createSessionFindController(() => root);
		});
		controller.openFind();
		controller.setQuery('match');

		expect(controller.open).toBe(true);
		expect(controller.matchCount).toBe(2);
		expect(controller.activeIndex).toBe(0);
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
		expect(highlights.size).toBe(0);
		cleanup();
	});

	it('reindexes mutations on the throttled refresh interval', async () => {
		vi.useFakeTimers();
		let controller!: ReturnType<typeof createSessionFindController>;
		const cleanup = $effect.root(() => {
			controller = createSessionFindController(() => root);
		});
		controller.openFind();
		controller.setQuery('match');
		const disconnect = controller.observe();

		root.querySelector('[data-session-find-text]')?.append(' match');
		await Promise.resolve();
		await vi.advanceTimersByTimeAsync(120);

		expect(controller.matchCount).toBe(3);
		disconnect();
		cleanup();
	});

	it('expands a collapsed user message before scrolling to its active match', async () => {
		root.innerHTML = `
			<div data-session-find-text>
				<div data-user-message-viewport>hidden match</div>
				<button data-user-message-expand aria-expanded="false">Expand</button>
			</div>
		`;
		const expand = root.querySelector<HTMLButtonElement>('[data-user-message-expand]')!;
		const message = root.querySelector<HTMLElement>('[data-session-find-text]')!;
		const order: string[] = [];
		vi.spyOn(expand, 'click').mockImplementation(() => order.push('expand'));
		vi.mocked(HTMLElement.prototype.scrollIntoView).mockImplementation(function (this: HTMLElement) {
			if (this === message) order.push('scroll');
		});
		let controller!: ReturnType<typeof createSessionFindController>;
		const cleanup = $effect.root(() => {
			controller = createSessionFindController(() => root);
		});

		controller.openFind();
		controller.setQuery('hidden');
		await vi.waitFor(() => expect(order).toEqual(['expand', 'scroll']));

		cleanup();
	});
});
