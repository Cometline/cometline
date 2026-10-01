const POPUP_WIDTH = 132;
const POPUP_GAP_AND_HEIGHT = 36;
const VIEWPORT_MARGIN = 8;

export function firstSelectionClientRect(range: Range): DOMRect {
	const rects = range.getClientRects();
	for (let i = 0; i < rects.length; i += 1) {
		const rect = rects.item(i);
		if (rect && (rect.width > 0 || rect.height > 0)) return rect;
	}
	return range.getBoundingClientRect();
}

export function selectionPopupPosition(rect: Pick<DOMRect, 'left' | 'top'>, viewportWidth: number) {
	return {
		top: Math.max(VIEWPORT_MARGIN, rect.top - POPUP_GAP_AND_HEIGHT),
		left: Math.min(
			Math.max(VIEWPORT_MARGIN, rect.left),
			Math.max(VIEWPORT_MARGIN, viewportWidth - POPUP_WIDTH)
		)
	};
}
