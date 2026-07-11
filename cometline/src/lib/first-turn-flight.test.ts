// @vitest-environment jsdom

import { describe, expect, it, vi } from 'vitest';
import {
	blendFlightOrigin,
	dockUserOrigin,
	measureStableUserBubble,
	rectTransform,
	textareaUserOrigin,
	translateStyle
} from './first-turn-flight';

describe('rectTransform', () => {
	it('calculates the inverse transform from the final layout to the source rect', () => {
		const from = new DOMRect(100, 200, 400, 120);
		const to = new DOMRect(300, 80, 400, 60);

		expect(rectTransform(from, to)).toEqual({ dx: -200, dy: 120, sx: 1, sy: 2 });
	});
});

describe('translateStyle', () => {
	it('locks position and measured target dimensions for the flight particle', () => {
		const from = new DOMRect(100, 200, 120, 40);
		const to = new DOMRect(300, 80, 280, 72);

		const style = translateStyle(from, to);

		expect(style).toContain('--flight-x:200px');
		expect(style).toContain('--flight-y:-120px');
		expect(style).toContain('left:100px');
		expect(style).toContain('top:200px');
		expect(style).toContain('width:280px');
		expect(style).toContain('height:72px');
		expect(style).toContain('box-sizing:border-box');
	});
});

describe('measureStableUserBubble', () => {
	it('returns the target rect once the user stack width stabilizes', async () => {
		const target = document.createElement('div');
		const stack = document.createElement('div');
		stack.className = 'user-stack';
		stack.appendChild(target);
		document.body.appendChild(stack);

		let width = 200;
		vi.spyOn(stack, 'getBoundingClientRect').mockImplementation(() => {
			const rect = new DOMRect(0, 0, width, 48);
			if (width < 400) width = 400;
			return rect;
		});
		vi.spyOn(target, 'getBoundingClientRect').mockReturnValue(new DOMRect(40, 10, 320, 56));

		const rect = await measureStableUserBubble(target);

		expect(rect.width).toBe(320);
		expect(rect.height).toBe(56);
		stack.remove();
	});
});

describe('blendFlightOrigin', () => {
	it('shortens horizontal and vertical travel toward the thread target', () => {
		const from = new DOMRect(320, 616, 280, 72);
		const to = new DOMRect(280, 120, 280, 72);

		const blended = blendFlightOrigin(from, to);

		expect(blended.left).toBeCloseTo(294, 5);
		expect(blended.top).toBeCloseTo(293.6, 5);
		expect(blended.width).toBe(280);
	});
});

describe('textareaUserOrigin', () => {
	it('right-aligns to the textarea and vertically centers on it', () => {
		const textarea = new DOMRect(100, 500, 400, 48);
		const target = new DOMRect(0, 0, 280, 72);

		const origin = textareaUserOrigin(textarea, target);

		expect(origin.width).toBe(280);
		expect(origin.height).toBe(72);
		expect(origin.left).toBe(220);
		expect(origin.top).toBe(488);
	});
});

describe('dockUserOrigin', () => {
	it('right-aligns to the composer wrapper and sits above it', () => {
		const composerWrapper = new DOMRect(80, 700, 520, 120);
		const target = new DOMRect(0, 0, 280, 72);

		const origin = dockUserOrigin(composerWrapper, target, 12);

		expect(origin.width).toBe(280);
		expect(origin.height).toBe(72);
		expect(origin.left).toBe(320);
		expect(origin.top).toBe(616);
	});

	it('uses a custom gap above the composer wrapper', () => {
		const composerWrapper = new DOMRect(80, 700, 520, 120);
		const target = new DOMRect(0, 0, 280, 72);

		const origin = dockUserOrigin(composerWrapper, target, 20);

		expect(origin.top).toBe(608);
	});
});
