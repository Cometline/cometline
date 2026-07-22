/** Minimum usable web/file panel width while dragging or restoring. */
export const WEB_PANEL_MIN_WIDTH = 320;

/** Web panel may occupy at most this fraction of the content row. */
export const WEB_PANEL_MAX_RATIO = 2 / 3;

/** Matches the CSS default `--web-panel-width: 50vw` when nothing is persisted. */
export const WEB_PANEL_DEFAULT_RATIO = 0.5;

/** Slim strip for the collapsed shell titlebar when the sidebar is hidden. */
export const COLLAPSED_MAIN_MIN_WIDTH = 72;

/**
 * Minimum main-pane width when the sidebar (or fullscreen chrome) is visible,
 * so the hero composer stays usable instead of being clipped at a 2/3 web panel.
 */
export const MAIN_PANEL_USABLE_MIN = 400;

export type WebPanelWidthChrome = {
	sidebarOpen: boolean;
	fullscreen: boolean;
};

export type WebPanelSizePrefs = {
	/** Preferred fraction of the content row. 0 means unset. */
	webPanelRatio: number;
	/** Legacy / last-applied absolute width in px. 0 means unset. */
	webPanelWidth: number;
};

function mainPaneReservation(chrome: WebPanelWidthChrome): number {
	if (chrome.sidebarOpen || chrome.fullscreen) return MAIN_PANEL_USABLE_MIN;
	return COLLAPSED_MAIN_MIN_WIDTH;
}

/** Upper bound for the web panel given the content-row width and shell chrome. */
export function webPanelMaxWidth(rowWidth: number, chrome: WebPanelWidthChrome): number {
	const byRatio = Math.floor(rowWidth * WEB_PANEL_MAX_RATIO);
	const byMain = Math.max(0, rowWidth - mainPaneReservation(chrome));
	return Math.min(byRatio, byMain);
}

/** Clamp a desired web panel width into the allowed range. */
export function clampWebPanelWidth(
	width: number,
	rowWidth: number,
	chrome: WebPanelWidthChrome
): number {
	const max = webPanelMaxWidth(rowWidth, chrome);
	return Math.min(Math.max(width, WEB_PANEL_MIN_WIDTH), max);
}

/** Clamp a stored/dragged ratio into the legal preference range. */
export function clampWebPanelRatio(ratio: number): number {
	if (!Number.isFinite(ratio) || ratio <= 0) return WEB_PANEL_DEFAULT_RATIO;
	return Math.min(Math.max(ratio, 0), WEB_PANEL_MAX_RATIO);
}

/** Convert an absolute width into a content-row ratio. */
export function widthToRatio(width: number, rowWidth: number): number {
	if (!(rowWidth > 0) || !Number.isFinite(width)) return WEB_PANEL_DEFAULT_RATIO;
	return clampWebPanelRatio(width / rowWidth);
}

/**
 * Resolve the user's preferred ratio from settings.
 * Prefers an explicit ratio; falls back to legacy absolute width, then 50%.
 */
export function resolveWebPanelRatio(prefs: WebPanelSizePrefs, rowWidth: number): number {
	if (prefs.webPanelRatio > 0) return clampWebPanelRatio(prefs.webPanelRatio);
	if (prefs.webPanelWidth > 0 && rowWidth > 0) {
		return widthToRatio(prefs.webPanelWidth, rowWidth);
	}
	return WEB_PANEL_DEFAULT_RATIO;
}

/** Map a preferred ratio onto a legal pixel width for the current layout. */
export function widthFromRatio(
	ratio: number,
	rowWidth: number,
	chrome: WebPanelWidthChrome
): number {
	const preferred = Math.round(rowWidth * clampWebPanelRatio(ratio));
	return clampWebPanelWidth(preferred, rowWidth, chrome);
}
