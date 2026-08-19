import { describe, expect, it } from 'vitest';
import { virtualWindow } from './virtual-list';

describe('virtualWindow', () => {
	it('returns an empty window for no rows', () => {
		expect(virtualWindow(0, 0, 200)).toEqual({ start: 0, end: 0, offset: 0, height: 0 });
	});

	it('only includes rows around the viewport plus overscan', () => {
		const window = virtualWindow(80, 220, 200, 22, 2);
		expect(window.start).toBe(8);
		expect(window.end).toBe(22);
		expect(window.offset).toBe(8 * 22);
		expect(window.height).toBe(80 * 22);
	});

	it('clamps to the list bounds', () => {
		const window = virtualWindow(5, 0, 400, 22, 8);
		expect(window.start).toBe(0);
		expect(window.end).toBe(5);
	});
});
