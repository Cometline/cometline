import { describe, expect, it } from 'vitest';
import { shouldUseWebPanelHistory } from './focus-nav';

describe('shouldUseWebPanelHistory', () => {
	it('uses web history whenever the web panel is open', () => {
		expect(shouldUseWebPanelHistory(true)).toBe(true);
		expect(shouldUseWebPanelHistory(false)).toBe(false);
	});
});
