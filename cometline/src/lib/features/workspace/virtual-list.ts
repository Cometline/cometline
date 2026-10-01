export const FILE_TREE_SEARCH_ROW_HEIGHT = 22;
export const FILE_TREE_SEARCH_LIMIT = 80;

export function virtualWindow(
	count: number,
	scrollTop: number,
	viewportHeight: number,
	rowHeight = FILE_TREE_SEARCH_ROW_HEIGHT,
	overscan = 8
): { start: number; end: number; offset: number; height: number } {
	if (count <= 0) {
		return { start: 0, end: 0, offset: 0, height: 0 };
	}
	const safeRow = Math.max(1, rowHeight);
	const start = Math.max(0, Math.floor(Math.max(0, scrollTop) / safeRow) - overscan);
	const visible = Math.ceil(Math.max(0, viewportHeight) / safeRow) + overscan * 2;
	const end = Math.min(count, start + Math.max(visible, 1));
	return {
		start,
		end,
		offset: start * safeRow,
		height: count * safeRow
	};
}
