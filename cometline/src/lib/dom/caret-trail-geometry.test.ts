import { describe, expect, it } from 'vitest';
import {
	clampUnit,
	easeOutCirc,
	headTailProgress,
	isStraightMove,
	pointsToSvg,
	trailPolygonPoints
} from './caret-trail-geometry';

describe('caret-trail-geometry', () => {
	it('clamps non-finite and out-of-range values', () => {
		expect(clampUnit(Number.NaN)).toBe(0);
		expect(clampUnit(-1)).toBe(0);
		expect(clampUnit(2)).toBe(1);
		expect(clampUnit(0.4)).toBe(0.4);
	});

	it('easeOutCirc stays in unit range', () => {
		expect(easeOutCirc(0)).toBe(0);
		expect(easeOutCirc(1)).toBe(1);
		expect(easeOutCirc(0.5)).toBeGreaterThan(0.5);
	});

	it('treats axis-aligned deltas as straight moves', () => {
		expect(isStraightMove(10, 0)).toBe(true);
		expect(isStraightMove(0, 10)).toBe(true);
		expect(isStraightMove(10, 10)).toBe(false);
	});

	it('snaps head for short moves and delays tail for long moves', () => {
		const short = headTailProgress({ progress: 0.5, lineLength: 10, maxTrailLength: 40 });
		expect(short.head).toBe(1);
		expect(short.tail).toBeGreaterThan(0);

		const long = headTailProgress({ progress: 0.2, lineLength: 100, maxTrailLength: 40 });
		expect(long.head).toBeLessThan(1);
		expect(long.tail).toBeLessThan(long.head);
	});

	it('builds svg point strings for trail polygons', () => {
		const pts = trailPolygonPoints({ x: 0, y: 0 }, { x: 10, y: 0 }, 2, 20);
		expect(pointsToSvg(pts).split(' ')).toHaveLength(4);
	});
});
