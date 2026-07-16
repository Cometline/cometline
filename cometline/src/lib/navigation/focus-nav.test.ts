import { describe, expect, it } from 'vitest';
import { shouldUseWebPanelHistory } from './focus-nav';

describe('shouldUseWebPanelHistory', () => {
	it('uses web history only when the web panel is open and focused', () => {
		expect(shouldUseWebPanelHistory(true, 'web')).toBe(true);
		expect(shouldUseWebPanelHistory(true, 'chat')).toBe(false);
		expect(shouldUseWebPanelHistory(false, 'web')).toBe(false);
		expect(shouldUseWebPanelHistory(false, 'chat')).toBe(false);
	});
});
