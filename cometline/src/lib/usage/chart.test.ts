import { describe, expect, it } from 'vitest';
import { nearestPointIndex, stackedAreaPaths } from './chart';

describe('usage chart', () => {
	it('builds a closed stacked path per series', () => {
		const paths = stackedAreaPaths(
			[
				{ date: '2026-08-12', cumulative: { a: 10, b: 2 } },
				{ date: '2026-08-13', cumulative: { a: 20, b: 5 } }
			],
			['a', 'b'],
			400,
			160
		);
		expect(paths).toHaveLength(2);
		expect(paths[0]?.d.startsWith('M ')).toBe(true);
		expect(paths[0]?.d.endsWith('Z')).toBe(true);
	});

	it('snaps hover x to the nearest day', () => {
		const points = [
			{ date: '2026-08-12', cumulative: { a: 1 } },
			{ date: '2026-08-13', cumulative: { a: 2 } },
			{ date: '2026-08-14', cumulative: { a: 3 } }
		];
		expect(nearestPointIndex(points, 400, 36)).toBe(0);
		expect(nearestPointIndex(points, 400, 400)).toBe(2);
	});
});
