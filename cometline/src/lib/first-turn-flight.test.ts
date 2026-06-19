// @vitest-environment jsdom

import { describe, expect, it } from 'vitest';
import { blendFlightOrigin, dockUserOrigin, textareaUserOrigin } from './first-turn-flight';

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
