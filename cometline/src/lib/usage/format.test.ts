import { describe, expect, it } from 'vitest';
import { clampUsageRange, formatKind, formatRangeLabel, formatTokens, formatUSD, rangeForPreset } from './format';

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

	it('builds last-month as the previous calendar month', () => {
		const range = rangeForPreset('month', new Date(2026, 7, 18));
		expect(new Date(range.from).getMonth()).toBe(6);
		expect(new Date(range.to).getMonth()).toBe(7);
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
