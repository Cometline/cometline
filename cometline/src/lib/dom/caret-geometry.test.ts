// @vitest-environment jsdom

import { describe, expect, it } from 'vitest';
import {
	readTextFieldCaretClientRect,
	readTextFieldCaretLocal,
	textFieldCaretLineHeight,
	viewportDeltaToLocal
} from './caret-geometry';

function mockWrap(opts: { offsetWidth: number; offsetHeight: number; wrapRect: DOMRect }) {
	const wrap = document.createElement('div');
	Object.defineProperty(wrap, 'offsetWidth', { value: opts.offsetWidth });
	Object.defineProperty(wrap, 'offsetHeight', { value: opts.offsetHeight });
	wrap.getBoundingClientRect = () => opts.wrapRect;
	return wrap;
}

describe('viewportDeltaToLocal', () => {
	it('keeps ratio ~1 when viewport size matches layout size', () => {
		const wrap = mockWrap({
			offsetWidth: 400,
			offsetHeight: 100,
			wrapRect: new DOMRect(10, 20, 400, 100)
		});
		const rect = new DOMRect(50, 30, 0, 22.5);

		const result = viewportDeltaToLocal(wrap, rect, 22.5);

		expect(result.x).toBeCloseTo(40);
		expect(result.y).toBeCloseTo(10);
		expect(result.h).toBeCloseTo(22.5);
	});

	it('applies inverse scale when viewport size is enlarged by transform', () => {
		const wrap = mockWrap({
			offsetWidth: 400,
			offsetHeight: 100,
			wrapRect: new DOMRect(10, 20, 404, 101)
		});
		const rect = new DOMRect(50, 30, 0, 0);

		const result = viewportDeltaToLocal(wrap, rect, 22.5);

		expect(result.x).toBeCloseTo(40 * (400 / 404), 5);
		expect(result.y).toBeCloseTo(10 * (100 / 101), 5);
		expect(result.h).toBeCloseTo(22.5 * (100 / 101), 5);
	});

	it('falls back to scale 1 when wrap viewport dimensions are zero', () => {
		const wrap = mockWrap({
			offsetWidth: 400,
			offsetHeight: 100,
			wrapRect: new DOMRect(10, 20, 0, 0)
		});
		const rect = new DOMRect(50, 30, 0, 18);

		const result = viewportDeltaToLocal(wrap, rect, 22.5);

		expect(result.x).toBeCloseTo(40);
		expect(result.y).toBeCloseTo(10);
		expect(result.h).toBeCloseTo(18);
	});
});

describe('readTextFieldCaretClientRect', () => {
	it('returns null when the field is not laid out', () => {
		const field = document.createElement('input');
		field.getBoundingClientRect = () => new DOMRect(0, 0, 0, 0);
		expect(readTextFieldCaretClientRect(field)).toBeNull();
	});

	it('measures at the selection end and removes the mirror', () => {
		const field = document.createElement('input');
		field.value = 'hello';
		field.selectionStart = 5;
		field.selectionEnd = 5;
		field.getBoundingClientRect = () => new DOMRect(20, 40, 200, 24);
		document.body.append(field);

		const originalRect = HTMLElement.prototype.getBoundingClientRect;
		HTMLElement.prototype.getBoundingClientRect = function () {
			if (this instanceof HTMLSpanElement && this.textContent === '\u200b') {
				return new DOMRect(88, 44, 0, 16);
			}
			return originalRect.call(this);
		};

		try {
			const rect = readTextFieldCaretClientRect(field);
			expect(rect?.left).toBe(88);
			expect(rect?.top).toBe(44);
			expect(document.body.querySelector('[aria-hidden="true"]')).toBeNull();
		} finally {
			HTMLElement.prototype.getBoundingClientRect = originalRect;
			field.remove();
		}
	});
});

describe('readTextFieldCaretLocal', () => {
	it('converts the mirrored caret into wrap-local coordinates', () => {
		const wrap = mockWrap({
			offsetWidth: 320,
			offsetHeight: 48,
			wrapRect: new DOMRect(10, 20, 320, 48)
		});
		const field = document.createElement('input');
		field.value = 'hi';
		field.selectionEnd = 2;
		field.getBoundingClientRect = () => new DOMRect(40, 28, 240, 24);
		Object.defineProperty(field, 'clientHeight', { value: 20 });
		wrap.append(field);
		document.body.append(wrap);

		const originalRect = HTMLElement.prototype.getBoundingClientRect;
		HTMLElement.prototype.getBoundingClientRect = function () {
			if (this instanceof HTMLSpanElement && this.textContent === '\u200b') {
				return new DOMRect(70, 32, 0, 18);
			}
			if (this === wrap) return new DOMRect(10, 20, 320, 48);
			return originalRect.call(this);
		};

		try {
			const result = readTextFieldCaretLocal(wrap, field);
			expect(result?.x).toBeCloseTo(60);
			expect(result?.y).toBeCloseTo(12);
			expect(result?.h).toBeCloseTo(18);
		} finally {
			HTMLElement.prototype.getBoundingClientRect = originalRect;
			wrap.remove();
		}
	});
});

describe('textFieldCaretLineHeight', () => {
	it('falls back to the field height when line-height is normal', () => {
		const field = document.createElement('input');
		Object.defineProperty(field, 'clientHeight', { value: 19 });
		expect(textFieldCaretLineHeight(field)).toBe(19);
	});
});
