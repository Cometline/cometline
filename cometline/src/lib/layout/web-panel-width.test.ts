import { describe, expect, it } from 'vitest';
import {
	WEB_PANEL_DEFAULT_RATIO,
	WEB_PANEL_MAX_RATIO,
	WEB_PANEL_MIN_WIDTH,
	clampWebPanelRatio,
	clampWebPanelWidth,
	resolveWebPanelRatio,
	webPanelMaxWidth,
	widthFromRatio,
	widthToRatio
} from './web-panel-width';

describe('web-panel-width', () => {
	it('caps the web panel at 2/3 of the content row on wide windows', () => {
		const rowWidth = 1800;
		const max = webPanelMaxWidth(rowWidth, { sidebarOpen: false, fullscreen: false });
		expect(max).toBe(Math.floor(rowWidth * WEB_PANEL_MAX_RATIO));
		expect(max).toBe(1200);
	});

	it('keeps the same 2/3 cap when the sidebar opens on a wide row', () => {
		const rowWidth = 1800;
		// 1800 - 400 usable main = 1400 > 1200 (2/3), so ratio still wins.
		expect(webPanelMaxWidth(rowWidth, { sidebarOpen: true, fullscreen: false })).toBe(
			Math.floor(rowWidth * WEB_PANEL_MAX_RATIO)
		);
	});

	it('yields to a usable main pane when 2/3 would crush the composer', () => {
		const rowWidth = 1100;
		// 2/3 ≈ 733, but 1100 - 400 = 700 → usable main binds.
		expect(webPanelMaxWidth(rowWidth, { sidebarOpen: true, fullscreen: false })).toBe(700);
	});

	it('clamps drag targets into [min, max]', () => {
		const rowWidth = 1800;
		const chrome = { sidebarOpen: false, fullscreen: false };
		expect(clampWebPanelWidth(100, rowWidth, chrome)).toBe(WEB_PANEL_MIN_WIDTH);
		expect(clampWebPanelWidth(1500, rowWidth, chrome)).toBe(1200);
		expect(clampWebPanelWidth(900, rowWidth, chrome)).toBe(900);
	});

	it('scales a preferred ratio across content-row sizes without baking in clamps', () => {
		const chrome = { sidebarOpen: true, fullscreen: false };
		const preferred = WEB_PANEL_MAX_RATIO;
		expect(widthFromRatio(preferred, 1800, chrome)).toBe(1200);
		expect(widthFromRatio(preferred, 1500, chrome)).toBe(1000);
		expect(widthFromRatio(preferred, 1800, { sidebarOpen: false, fullscreen: false })).toBe(
			1200
		);
	});

	it('resolves ratio from settings with legacy width fallback', () => {
		expect(
			resolveWebPanelRatio({ webPanelRatio: 0.4, webPanelWidth: 900 }, 1800)
		).toBe(0.4);
		expect(
			resolveWebPanelRatio({ webPanelRatio: 0, webPanelWidth: 720 }, 1800)
		).toBe(0.4);
		expect(
			resolveWebPanelRatio({ webPanelRatio: 0, webPanelWidth: 0 }, 1800)
		).toBe(WEB_PANEL_DEFAULT_RATIO);
		expect(clampWebPanelRatio(0.9)).toBe(WEB_PANEL_MAX_RATIO);
		expect(widthToRatio(900, 1800)).toBe(0.5);
	});
});
