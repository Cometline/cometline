import { describe, expect, it } from 'vitest';
import { shouldUseWorkspacePanelHistory } from './focus-nav';

describe('shouldUseWorkspacePanelHistory', () => {
	it('uses panel history only when the open panel is mounted', () => {
		expect(shouldUseWorkspacePanelHistory(true, true)).toBe(true);
		expect(shouldUseWorkspacePanelHistory(true, false)).toBe(false);
		expect(shouldUseWorkspacePanelHistory(false, true)).toBe(false);
	});
});
