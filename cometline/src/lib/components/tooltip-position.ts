export type TooltipPlacement = 'above' | 'below';

export type TooltipBox = {
	width: number;
	height: number;
};

export type TooltipViewport = {
	width: number;
	height: number;
};

export type TooltipPosition = {
	top: number;
	left: number;
	placement: TooltipPlacement;
};

/**
 * Prefer above the anchor; flip below if needed. Horizontally center on the
 * anchor, then clamp so the tip stays inside the viewport with a margin.
 */
export function clampTooltipPosition(input: {
	anchor: { top: number; left: number; width: number; height: number; bottom: number };
	tip: TooltipBox;
	viewport: TooltipViewport;
	gap?: number;
	margin?: number;
}): TooltipPosition {
	const gap = input.gap ?? 6;
	const margin = input.margin ?? 8;
	const { anchor, tip, viewport } = input;

	let placement: TooltipPlacement = 'above';
	let top = anchor.top - tip.height - gap;
	if (top < margin) {
		placement = 'below';
		top = anchor.bottom + gap;
		if (top + tip.height > viewport.height - margin) {
			top = Math.max(margin, viewport.height - tip.height - margin);
		}
	}

	let left = anchor.left + anchor.width / 2 - tip.width / 2;
	const minLeft = margin;
	const maxLeft = Math.max(margin, viewport.width - tip.width - margin);
	left = Math.min(Math.max(left, minLeft), maxLeft);

	return { top, left, placement };
}
