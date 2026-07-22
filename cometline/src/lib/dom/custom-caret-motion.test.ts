import { describe, expect, it } from 'vitest';
import { resolveCaretMotion } from './custom-caret';

const caretH = 22.5;
const sameLineDy = 2;
const wrapDy = caretH;

describe('resolveCaretMotion', () => {
	it('uses typingTrail for recent typing on the same line', () => {
		expect(resolveCaretMotion({ dy: sameLineDy, caretH, recentlyTyped: true })).toBe(
			'typingTrail'
		);
	});

	it('uses fullMove for typing wrap (Enter / soft wrap)', () => {
		expect(resolveCaretMotion({ dy: wrapDy, caretH, recentlyTyped: true })).toBe('fullMove');
	});

	it('uses fullMove for arrow line crosses (diagonal)', () => {
		expect(resolveCaretMotion({ dy: wrapDy, caretH, recentlyTyped: false })).toBe('fullMove');
	});

	it('keeps fullMove for same-line arrow / selection moves', () => {
		expect(resolveCaretMotion({ dy: sameLineDy, caretH, recentlyTyped: false })).toBe(
			'fullMove'
		);
	});
});
