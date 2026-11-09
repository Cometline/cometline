import { describe, expect, it } from 'vitest';
import { computeIntroV2Motion, introV2Complete } from './intro-v2-motion';
import { INTRO_V2_BEATS } from './intro-timeline-v2';

describe('intro v2 motion', () => {
	it('starts full-bleed with heavy sky blur', () => {
		const m = computeIntroV2Motion(0, false);
		expect(m.windowScale).toBe(1);
		expect(m.windowRadius).toBe(0);
		expect(m.skyBlur).toBeGreaterThan(40);
		expect(m.openerOpacity).toBe(1);
	});

	it('settles the floating window mid-sequence', () => {
		const m = computeIntroV2Motion(INTRO_V2_BEATS.windowSettle, false);
		expect(m.windowScale).toBeLessThan(1);
		expect(m.windowRadius).toBeGreaterThan(18);
		expect(m.backdropAlpha).toBeGreaterThan(0.3);
	});

	it('reveals welcome panel near the end', () => {
		const m = computeIntroV2Motion(INTRO_V2_BEATS.welcomeIn + 500, false);
		expect(m.welcomeOpacity).toBeGreaterThan(0.3);
		expect(m.openerOpacity).toBe(0);
	});

	it('shows a strong brand beat mid-sequence', () => {
		const m = computeIntroV2Motion(3600, false);
		expect(m.wordmarkOpacity).toBeGreaterThan(0.45);
		expect(m.iconOpacity).toBeGreaterThan(0.3);
	});

	it('sharpens the sky after brand fade', () => {
		const m = computeIntroV2Motion(INTRO_V2_BEATS.skyReveal, false);
		expect(m.skyBlur).toBeLessThan(6);
		expect(m.skySharpOpacity).toBeGreaterThan(0.7);
	});

	it('completes on schedule', () => {
		expect(introV2Complete(INTRO_V2_BEATS.total, false)).toBe(true);
		expect(introV2Complete(INTRO_V2_BEATS.total - 1, false)).toBe(false);
	});

	it('honors reduced motion with a short path', () => {
		expect(introV2Complete(INTRO_V2_BEATS.reducedTotal, true)).toBe(true);
	});
});
