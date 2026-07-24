import { describe, expect, it } from 'vitest';
import { shouldUseWorkspacePanelHistory } from './focus-nav';

describe('shouldUseWorkspacePanelHistory', () => {
	it('uses web history whenever the workspace panel is open', () => {
		expect(shouldUseWorkspacePanelHistory(true)).toBe(true);
		expect(shouldUseWorkspacePanelHistory(false)).toBe(false);
	});
});
