/** Minimum usable web/file panel width while dragging or restoring. */
export const WORKSPACE_PANEL_MIN_WIDTH = 320;

/** Workspace panel may occupy at most this fraction of the content row. */
export const WORKSPACE_PANEL_MAX_RATIO = 2 / 3;

/** Matches the CSS default `--workspace-panel-width: 50vw` when nothing is persisted. */
export const WORKSPACE_PANEL_DEFAULT_RATIO = 0.5;

/** Slim strip for the collapsed shell titlebar when the sidebar is hidden. */
export const COLLAPSED_MAIN_MIN_WIDTH = 72;

/**
 * Minimum main-pane width when the sidebar (or fullscreen chrome) is visible,
 * so the hero composer stays usable instead of being clipped at a 2/3 workspace panel.
 */
export const MAIN_PANEL_USABLE_MIN = 400;

export type WorkspacePanelWidthChrome = {
	sidebarOpen: boolean;
	fullscreen: boolean;
};

export type WorkspacePanelSizePrefs = {
	/** Preferred fraction of the content row. 0 means unset. */
	workspacePanelRatio: number;
	/** Legacy / last-applied absolute width in px. 0 means unset. */
	workspacePanelWidth: number;
};

function mainPaneReservation(chrome: WorkspacePanelWidthChrome): number {
	if (chrome.sidebarOpen || chrome.fullscreen) return MAIN_PANEL_USABLE_MIN;
	return COLLAPSED_MAIN_MIN_WIDTH;
}

/** Upper bound for the workspace panel given the content-row width and shell chrome. */
export function workspacePanelMaxWidth(rowWidth: number, chrome: WorkspacePanelWidthChrome): number {
	const byRatio = Math.floor(rowWidth * WORKSPACE_PANEL_MAX_RATIO);
	const byMain = Math.max(0, rowWidth - mainPaneReservation(chrome));
	return Math.min(byRatio, byMain);
}

/** Clamp a desired workspace panel width into the allowed range. */
export function clampWorkspacePanelWidth(
	width: number,
	rowWidth: number,
	chrome: WorkspacePanelWidthChrome
): number {
	const max = workspacePanelMaxWidth(rowWidth, chrome);
	return Math.min(Math.max(width, WORKSPACE_PANEL_MIN_WIDTH), max);
}

/** Clamp a stored/dragged ratio into the legal preference range. */
export function clampWorkspacePanelRatio(ratio: number): number {
	if (!Number.isFinite(ratio) || ratio <= 0) return WORKSPACE_PANEL_DEFAULT_RATIO;
	return Math.min(Math.max(ratio, 0), WORKSPACE_PANEL_MAX_RATIO);
}

/** Convert an absolute width into a content-row ratio. */
export function widthToRatio(width: number, rowWidth: number): number {
	if (!(rowWidth > 0) || !Number.isFinite(width)) return WORKSPACE_PANEL_DEFAULT_RATIO;
	return clampWorkspacePanelRatio(width / rowWidth);
}

/**
 * Resolve the user's preferred ratio from settings.
 * Prefers an explicit ratio; falls back to legacy absolute width, then 50%.
 */
export function resolveWorkspacePanelRatio(prefs: WorkspacePanelSizePrefs, rowWidth: number): number {
	if (prefs.workspacePanelRatio > 0) return clampWorkspacePanelRatio(prefs.workspacePanelRatio);
	if (prefs.workspacePanelWidth > 0 && rowWidth > 0) {
		return widthToRatio(prefs.workspacePanelWidth, rowWidth);
	}
	return WORKSPACE_PANEL_DEFAULT_RATIO;
}

/** Map a preferred ratio onto a legal pixel width for the current layout. */
export function widthFromRatio(
	ratio: number,
	rowWidth: number,
	chrome: WorkspacePanelWidthChrome
): number {
	const preferred = Math.round(rowWidth * clampWorkspacePanelRatio(ratio));
	return clampWorkspacePanelWidth(preferred, rowWidth, chrome);
}
