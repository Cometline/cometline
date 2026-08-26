import { describe, expect, it } from 'vitest';
import {
	cacheHitRate,
	clampUsageRange,
	formatCacheHit,
	formatKind,
	formatRangeLabel,
	formatTokens,
	formatUSD,
	legendRowsForSeries,
	rangeForPreset,
	seriesKeyForBucket
} from './format';

describe('usage format', () => {
	it('formats token counts', () => {
		expect(formatTokens(0)).toBe('0');
		expect(formatTokens(850)).toBe('850');
		expect(formatTokens(12_400)).toBe('12.4k');
		expect(formatTokens(2_100_000)).toBe('2.1M');
	});

	it('formats usd', () => {
		expect(formatUSD(0)).toBe('$0.00');
		expect(formatUSD(12.4)).toBe('$12.40');
		expect(formatUSD(0.0042)).toBe('$0.0042');
	});

	it('labels call kinds', () => {
		expect(formatKind('agent_step')).toBe('Agent');
		expect(formatKind('embedding')).toBe('Embedding');
	});

	it('builds today as the current local day', () => {
		const now = new Date(2026, 7, 19, 11, 8, 37);
		const range = rangeForPreset('today', now);
		expect(range.from).toBe(new Date(2026, 7, 19).getTime());
		expect(range.to).toBe(now.getTime() + 1);
		expect(formatRangeLabel(range.from, range.to)).toBe(
			new Date(2026, 7, 19).toLocaleDateString(undefined, {
				month: 'short',
				day: 'numeric',
				year: 'numeric'
			})
		);
	});

	it('builds last-month as the previous calendar month', () => {
		const range = rangeForPreset('month', new Date(2026, 7, 18));
		expect(new Date(range.from).getMonth()).toBe(6);
		expect(new Date(range.to).getMonth()).toBe(7);
	});

	it('maps summary buckets onto series keys with estimated spend', () => {
		expect(
			seriesKeyForBucket(
				{ key: 'gpt-5.6-luna', provider_id: 'codex', model_id: 'gpt-5.6-luna', tokens: 1, estimated_usd: 0.12, priced: true },
				'model'
			)
		).toBe('codex/gpt-5.6-luna');
		expect(
			legendRowsForSeries(
				['codex/gpt-5.6-luna', 'xai/grok-4.6'],
				[
					{
						key: 'gpt-5.6-luna',
						provider_id: 'codex',
						model_id: 'gpt-5.6-luna',
						tokens: 8800,
						estimated_usd: 0.12,
						priced: true
					},
					{
						key: 'grok-4.6',
						provider_id: 'xai',
						model_id: 'grok-4.6',
						tokens: 528800,
						estimated_usd: 0,
						priced: false
					}
				],
				'model'
			)
		).toEqual([
			{
				key: 'codex/gpt-5.6-luna',
				label: 'codex/gpt-5.6-luna',
				tokens: 8800,
				cache: '—',
				cost: '$0.12'
			},
			{ key: 'xai/grok-4.6', label: 'xai/grok-4.6', tokens: 528800, cache: '—', cost: '—' }
		]);
	});

	it('formats cache hit rate and hides zero as unknown', () => {
		expect(formatCacheHit(cacheHitRate(800, 200))).toBe('20%');
		expect(formatCacheHit(cacheHitRate(1000, 0))).toBe('—');
	});

	it('clamps custom ranges to one year', () => {
		const now = new Date(2026, 7, 18);
		const clamped = clampUsageRange(new Date(2020, 0, 1).getTime(), now.getTime() + 1, now);
		expect(clamped.to - clamped.from).toBeLessThanOrEqual(366 * 24 * 60 * 60 * 1000);
		expect(clamped.from).toBeGreaterThan(new Date(2020, 0, 1).getTime());
	});

	it('builds the past-year window from the retained limit', () => {
		const now = new Date(2026, 7, 18);
		const range = rangeForPreset('year', now);
		expect(range.to - range.from).toBeLessThanOrEqual(366 * 24 * 60 * 60 * 1000);
		expect(formatRangeLabel(range.from, range.to)).toMatch(/2025|2026/);
	});
});
