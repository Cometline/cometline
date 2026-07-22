const MAIN_MIN_WIDTH_FALLBACK = 560;
const MINI_WIDTH_FLOOR = 360;
const MINI_HEIGHT_FLOOR = 440;
/** Legacy 460×640 mini window aspect ratio. */
const MINI_HEIGHT_PER_WIDTH = 640 / 460;

/** Main window minimum width as one third of the display work area. */
export function mainWindowMinWidthForWorkArea(workAreaWidth: number): number {
	const width = Number(workAreaWidth);
	if (!Number.isFinite(width) || width <= 0) return MAIN_MIN_WIDTH_FALLBACK;
	return Math.max(1, Math.round(width / 3));
}

/** Mini window size from the cursor display work area (width ≈ ⅓, height by aspect, capped). */
export function miniWindowSizeForWorkArea(
	workAreaWidth: number,
	workAreaHeight: number,
	screenMargin = 18
): { width: number; height: number } {
	const margin = Math.max(0, Number(screenMargin) || 0);
	const areaH = Number(workAreaHeight);
	const width = Math.max(MINI_WIDTH_FLOOR, mainWindowMinWidthForWorkArea(workAreaWidth));
	const maxHeight =
		Number.isFinite(areaH) && areaH > 0
			? Math.max(MINI_HEIGHT_FLOOR, areaH - margin * 2)
			: MINI_HEIGHT_FLOOR;
	let height = Math.round(width * MINI_HEIGHT_PER_WIDTH);
	height = Math.min(Math.max(MINI_HEIGHT_FLOOR, height), maxHeight);
	return { width, height };
}

export function miniWindowOriginForWorkArea(
	workArea: { x: number; y: number; width: number; height: number },
	size: { width: number; height: number },
	screenMargin = 18
): { x: number; y: number } {
	const margin = Math.max(0, Number(screenMargin) || 0);
	return {
		x: Math.round(workArea.x + workArea.width - size.width - margin),
		y: Math.round(workArea.y + workArea.height - size.height - margin)
	};
}
