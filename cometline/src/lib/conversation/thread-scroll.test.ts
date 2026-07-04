// @vitest-environment jsdom
import { describe, expect, it } from 'vitest';
import { followUpPinScrollMargin, isNearBottom } from './thread-scroll';

describe('isNearBottom', () => {
	it('returns true when within threshold of bottom', () => {
		const el = {
			scrollHeight: 1000,
			scrollTop: 904,
			clientHeight: 96
		} as HTMLElement;
		expect(isNearBottom(el, 96)).toBe(true);
	});
});

describe('followUpPinScrollMargin', () => {
	it('uses the upper-third ratio when viewport height is known', () => {
		expect(followUpPinScrollMargin(800)).toBe(224);
	});

	it('falls back when viewport height is zero', () => {
		expect(followUpPinScrollMargin(0)).toBe(100);
	});
});
