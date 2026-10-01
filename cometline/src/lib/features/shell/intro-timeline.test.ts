import { describe, expect, it } from 'vitest';
import { clamp01, darkenRgb, easeOut, seg } from './intro-timeline';

describe('intro timeline helpers', () => {
	it('clamps progress into 0..1', () => {
		expect(clamp01(-0.5)).toBe(0);
		expect(clamp01(0.25)).toBe(0.25);
		expect(clamp01(2)).toBe(1);
	});

	it('normalizes elapsed time across a segment', () => {
		expect(seg(0, 100, 300)).toBe(0);
		expect(seg(200, 100, 300)).toBe(0.5);
		expect(seg(400, 100, 300)).toBe(1);
	});

	it('eases out with a cubic curve', () => {
		expect(easeOut(0)).toBe(0);
		expect(easeOut(0.5)).toBe(0.875);
		expect(easeOut(1)).toBe(1);
	});

	it('darkens each channel by the factor', () => {
		expect(darkenRgb({ r: 100, g: 200, b: 255 }, 0.5)).toEqual({ r: 50, g: 100, b: 128 });
	});
});
