import { describe, expect, it } from 'vitest';
import { selectionPopupPosition } from './selection-popup';

describe('selectionPopupPosition', () => {
	it('places the action above the first selected line', () => {
		expect(selectionPopupPosition({ left: 120, top: 200 }, 800)).toEqual({
			left: 120,
			top: 164
		});
	});

	it('keeps the action inside the viewport', () => {
		expect(selectionPopupPosition({ left: 790, top: 20 }, 800)).toEqual({
			left: 668,
			top: 8
		});
	});
});
