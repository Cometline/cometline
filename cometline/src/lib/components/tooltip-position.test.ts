import { describe, expect, it } from 'vitest';
import { clampTooltipPosition } from './tooltip-position';

const tip = { width: 120, height: 28 };
const viewport = { width: 800, height: 600 };

describe('clampTooltipPosition', () => {
	it('centers above a mid-screen anchor', () => {
		const pos = clampTooltipPosition({
			anchor: { top: 200, left: 340, width: 28, height: 28, bottom: 228 },
			tip,
			viewport
		});
		expect(pos.placement).toBe('above');
		expect(pos.top).toBe(200 - 28 - 6);
		expect(pos.left).toBe(340 + 14 - 60);
	});

	it('shifts right when the tip would overflow the left edge', () => {
		const pos = clampTooltipPosition({
			anchor: { top: 500, left: 8, width: 28, height: 28, bottom: 528 },
			tip,
			viewport
		});
		expect(pos.left).toBe(8);
		expect(pos.placement).toBe('above');
	});

	it('shifts left when the tip would overflow the right edge', () => {
		const pos = clampTooltipPosition({
			anchor: { top: 500, left: 780, width: 28, height: 28, bottom: 528 },
			tip,
			viewport
		});
		expect(pos.left).toBe(800 - 120 - 8);
	});

	it('flips below when there is not enough room above', () => {
		const pos = clampTooltipPosition({
			anchor: { top: 20, left: 200, width: 28, height: 28, bottom: 48 },
			tip,
			viewport
		});
		expect(pos.placement).toBe('below');
		expect(pos.top).toBe(48 + 6);
	});
});
