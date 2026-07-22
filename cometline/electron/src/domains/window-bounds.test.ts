import { describe, expect, it } from 'vitest';
import {
	mainWindowMinWidthForWorkArea,
	miniWindowOriginForWorkArea,
	miniWindowSizeForWorkArea
} from './window-bounds.js';

describe('mainWindowMinWidthForWorkArea', () => {
	it('uses one third of the work area width', () => {
		expect(mainWindowMinWidthForWorkArea(1440)).toBe(480);
		expect(mainWindowMinWidthForWorkArea(1920)).toBe(640);
		expect(mainWindowMinWidthForWorkArea(1680)).toBe(560);
	});

	it('falls back when work area width is invalid', () => {
		expect(mainWindowMinWidthForWorkArea(0)).toBe(560);
		expect(mainWindowMinWidthForWorkArea(Number.NaN)).toBe(560);
	});
});

describe('miniWindowSizeForWorkArea', () => {
	it('scales width to one third and height by legacy aspect ratio', () => {
		expect(miniWindowSizeForWorkArea(1440, 900, 18)).toEqual({ width: 480, height: 668 });
	});

	it('caps height to fit within the work area margins', () => {
		expect(miniWindowSizeForWorkArea(1440, 500, 18)).toEqual({ width: 480, height: 464 });
	});
});

describe('miniWindowOriginForWorkArea', () => {
	it('places the window in the bottom-right of the work area', () => {
		expect(
			miniWindowOriginForWorkArea(
				{ x: 0, y: 25, width: 1440, height: 875 },
				{ width: 480, height: 668 },
				18
			)
		).toEqual({ x: 942, y: 214 });
	});
});
