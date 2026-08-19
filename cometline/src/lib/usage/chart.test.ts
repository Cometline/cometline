import { describe, expect, it } from 'vitest';
import {
	nearestPointIndex,
	PAD_RIGHT,
	seriesColor,
	singleDayBarBounds,
	stackedAreaPaths,
	xLabels
} from './chart';

describe('usage chart', () => {
	it('keeps adjacent series distinguishable by hue and lightness', () => {
		const colors = Array.from({ length: 8 }, (_, index) => seriesColor(index, 8));
		expect(new Set(colors).size).toBe(8);
		const lights = colors.map((color) => Number(color.match(/% (\d+)%/)?.[1]));
		for (let i = 1; i < lights.length; i += 1) {
			expect(Math.abs((lights[i] ?? 0) - (lights[i - 1] ?? 0))).toBeGreaterThanOrEqual(10);
		}
	});

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

	it('gives a single day a centered stacked bar', () => {
		const width = 400;
		const paths = stackedAreaPaths(
			[{ date: '2026-08-19', cumulative: { a: 10, b: 5 } }],
			['a', 'b'],
			width,
			160
		);
		const { x0, x1 } = singleDayBarBounds(width);
		expect(paths).toHaveLength(2);
		const xs = [...(paths[0]?.d.matchAll(/(?:M|L) ([\d.]+)/g) ?? [])].map((match) => Number(match[1]));
		expect(new Set(xs)).toEqual(new Set([x0, x1]));
		expect(x1 - x0).toBeLessThan(width / 2);
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

	it('keeps the last x-axis label inside the plot and end-anchored', () => {
		const labels = xLabels(
			[
				{ date: '2026-07-20', cumulative: { a: 0 } },
				{ date: '2026-08-04', cumulative: { a: 0 } },
				{ date: '2026-08-18', cumulative: { a: 1 } }
			],
			400
		);
		expect(labels[0]).toMatchObject({ label: '07-20', anchor: 'start' });
		const last = labels.at(-1);
		expect(last).toMatchObject({ label: '08-18', anchor: 'end' });
		expect(last?.x).toBe(400 - PAD_RIGHT);
		expect(last?.x).toBeLessThan(400);
	});
});
