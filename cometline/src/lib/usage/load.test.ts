import { describe, expect, it } from 'vitest';
import { collectAllUsageEvents, isCurrentRefresh, localTZOffsetMin } from './load';

describe('usage load helpers', () => {
	it('drops stale refresh results', () => {
		expect(isCurrentRefresh(1, 2)).toBe(false);
		expect(isCurrentRefresh(3, 3)).toBe(true);
	});

	it('pages CSV export until total', async () => {
		const pages = [
			{ items: [{ id: 'a' }, { id: 'b' }], total: 3 },
			{ items: [{ id: 'c' }], total: 3 }
		];
		const items = await collectAllUsageEvents(async (offset) => pages[offset / 2] ?? { items: [], total: 3 }, 2);
		expect(items.map((item) => item.id)).toEqual(['a', 'b', 'c']);
	});

	it('returns minutes east of UTC', () => {
		const date = new Date('2026-08-18T00:00:00+08:00');
		expect(localTZOffsetMin(date)).toBe(-date.getTimezoneOffset());
	});
});
