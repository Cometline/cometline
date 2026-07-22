import { describe, expect, it } from 'vitest';
import { resolveCaretMotion } from './custom-caret';

const caretH = 22.5;
const sameLineDy = 2;
const wrapDy = caretH;

describe('resolveCaretMotion', () => {
	it('uses typingTrail for recent typing on the same line', () => {
		expect(
			resolveCaretMotion({ dy: sameLineDy, caretH, recentlyTyped: true })
		).toEqual({ mode: 'typingTrail', trailOnly: true });
	});

	it('uses fullMove for typing wrap (Enter / soft wrap)', () => {
		expect(resolveCaretMotion({ dy: wrapDy, caretH, recentlyTyped: true })).toEqual({
			mode: 'fullMove',
			trailOnly: false
		});
	});

	it('uses fullMove for arrow line crosses (diagonal)', () => {
		expect(resolveCaretMotion({ dy: wrapDy, caretH, recentlyTyped: false })).toEqual({
			mode: 'fullMove',
			trailOnly: false
		});
	});

	it('keeps fullMove for same-line arrow / selection moves', () => {
		expect(
			resolveCaretMotion({ dy: sameLineDy, caretH, recentlyTyped: false })
		).toEqual({ mode: 'fullMove', trailOnly: false });
	});
});
