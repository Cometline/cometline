// @vitest-environment jsdom

import { describe, expect, it } from 'vitest';
import { createMenuHighlight } from './menu-highlight.svelte';

describe('createMenuHighlight', () => {
	it('resets on query or open changes and clamps when the list shrinks', () => {
		let query = $state('a');
		let open = $state(true);
		let count = $state(5);
		let menu!: ReturnType<typeof createMenuHighlight>;
		const cleanup = $effect.root(() => {
			menu = createMenuHighlight({
				getQuery: () => query,
				getOpen: () => open,
				getCount: () => count
			});
		});

		expect(menu.index).toBe(0);
		menu.set(3);
		expect(menu.index).toBe(3);

		count = 2;
		expect(menu.index).toBe(1);

		query = 'b';
		expect(menu.index).toBe(0);

		menu.move(1);
		expect(menu.index).toBe(1);
		menu.move(-1);
		expect(menu.index).toBe(0);

		menu.set(1);
		open = false;
		expect(menu.index).toBe(0);
		open = true;
		expect(menu.index).toBe(0);

		cleanup();
	});
});
