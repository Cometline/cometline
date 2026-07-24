import { describe, expect, it } from 'vitest';
import {
	WORKSPACE_PANEL_DEFAULT_RATIO,
	WORKSPACE_PANEL_MAX_RATIO,
	WORKSPACE_PANEL_MIN_WIDTH,
	clampWorkspacePanelRatio,
	clampWorkspacePanelWidth,
	resolveWorkspacePanelRatio,
	workspacePanelMaxWidth,
	widthFromRatio,
	widthToRatio
} from './workspace-panel-width';

describe('workspace-panel-width', () => {
	it('caps the workspace panel at 2/3 of the content row on wide windows', () => {
		const rowWidth = 1800;
		const max = workspacePanelMaxWidth(rowWidth, { sidebarOpen: false, fullscreen: false });
		expect(max).toBe(Math.floor(rowWidth * WORKSPACE_PANEL_MAX_RATIO));
		expect(max).toBe(1200);
	});

	it('keeps the same 2/3 cap when the sidebar opens on a wide row', () => {
		const rowWidth = 1800;
		// 1800 - 400 usable main = 1400 > 1200 (2/3), so ratio still wins.
		expect(workspacePanelMaxWidth(rowWidth, { sidebarOpen: true, fullscreen: false })).toBe(
			Math.floor(rowWidth * WORKSPACE_PANEL_MAX_RATIO)
		);
	});

	it('yields to a usable main pane when 2/3 would crush the composer', () => {
		const rowWidth = 1100;
		// 2/3 ≈ 733, but 1100 - 400 = 700 → usable main binds.
		expect(workspacePanelMaxWidth(rowWidth, { sidebarOpen: true, fullscreen: false })).toBe(700);
	});

	it('clamps drag targets into [min, max]', () => {
		const rowWidth = 1800;
		const chrome = { sidebarOpen: false, fullscreen: false };
		expect(clampWorkspacePanelWidth(100, rowWidth, chrome)).toBe(WORKSPACE_PANEL_MIN_WIDTH);
		expect(clampWorkspacePanelWidth(1500, rowWidth, chrome)).toBe(1200);
		expect(clampWorkspacePanelWidth(900, rowWidth, chrome)).toBe(900);
	});

	it('scales a preferred ratio across content-row sizes without baking in clamps', () => {
		const chrome = { sidebarOpen: true, fullscreen: false };
		const preferred = WORKSPACE_PANEL_MAX_RATIO;
		expect(widthFromRatio(preferred, 1800, chrome)).toBe(1200);
		expect(widthFromRatio(preferred, 1500, chrome)).toBe(1000);
		expect(widthFromRatio(preferred, 1800, { sidebarOpen: false, fullscreen: false })).toBe(
			1200
		);
	});

	it('resolves ratio from settings with legacy width fallback', () => {
		expect(
			resolveWorkspacePanelRatio({ workspacePanelRatio: 0.4, workspacePanelWidth: 900 }, 1800)
		).toBe(0.4);
		expect(
			resolveWorkspacePanelRatio({ workspacePanelRatio: 0, workspacePanelWidth: 720 }, 1800)
		).toBe(0.4);
		expect(
			resolveWorkspacePanelRatio({ workspacePanelRatio: 0, workspacePanelWidth: 0 }, 1800)
		).toBe(WORKSPACE_PANEL_DEFAULT_RATIO);
		expect(clampWorkspacePanelRatio(0.9)).toBe(WORKSPACE_PANEL_MAX_RATIO);
		expect(widthToRatio(900, 1800)).toBe(0.5);
	});
});
