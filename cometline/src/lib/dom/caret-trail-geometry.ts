/** Geometry + easing helpers ported from Ghostty `cursor_tail.glsl`. */

export type Point = { x: number; y: number };

export function clampUnit(value: number): number {
	if (!Number.isFinite(value)) return 0;
	return Math.min(1, Math.max(0, value));
}

export function easeOutCirc(value: number): number {
	const x = clampUnit(value);
	return Math.sqrt(1 - (x - 1) * (x - 1));
}

function smoothstep(edge0: number, edge1: number, x: number): number {
	if (edge0 === edge1) return x >= edge1 ? 1 : 0;
	const t = clampUnit((x - edge0) / (edge1 - edge0));
	return t * t * (3 - 2 * t);
}

/**
 * DOM Y-down translation of `determineIfTopRightIsLeading` from cursor_tail.glsl
 * (shader uses Y-up, so the vertical comparisons are flipped).
 */
export function isTopRightLeading(current: Point, previous: Point): boolean {
	const leftAndAbove = current.x < previous.x && current.y < previous.y;
	const rightAndBelow = current.x > previous.x && current.y > previous.y;
	return !(leftAndAbove || rightAndBelow);
}

export function isStraightMove(dx: number, dy: number, threshold = 0.5): boolean {
	return Math.abs(dx) <= threshold || Math.abs(dy) <= threshold;
}

/**
 * Head/tail eased progress from cursor_tail.glsl.
 * Short moves: head already at destination, trail catches up.
 * Longer moves: head eases, trail delayed by maxTrailLength/lineLength.
 */
export function headTailProgress(opts: {
	progress: number;
	lineLength: number;
	maxTrailLength: number;
}): { head: number; tail: number } {
	const progress = clampUnit(opts.progress);
	const lineLength = Math.max(opts.lineLength, 1e-6);
	const maxTrailLength = Math.max(opts.maxTrailLength, 1e-6);
	const isLongMove = lineLength >= maxTrailLength;

	if (!isLongMove) {
		return { head: 1, tail: easeOutCirc(progress) };
	}

	const delay = Math.min(0.95, maxTrailLength / lineLength);
	return {
		head: easeOutCirc(progress),
		tail: easeOutCirc(smoothstep(delay, 1, progress))
	};
}

/** Full caret-cell parallelogram for diagonal moves (cursor_tail.glsl). */
export function diagonalTrailPoints(
	head: Point,
	tail: Point,
	w: number,
	h: number
): [Point, Point, Point, Point] {
	const topRightLeading = isTopRightLeading(head, tail);
	const bottomLeftLeading = !topRightLeading;
	const tr = topRightLeading ? 1 : 0;
	const bl = bottomLeftLeading ? 1 : 0;

	return [
		{ x: head.x + w * tr, y: head.y + h },
		{ x: head.x + w * bl, y: head.y },
		{ x: tail.x + w * bl, y: tail.y },
		{ x: tail.x + w * tr, y: tail.y + h }
	];
}

/** Axis-aligned smear between head and tail centers (cursor_tail.glsl straight path). */
export function straightTrailPoints(
	head: Point,
	tail: Point,
	w: number,
	h: number
): [Point, Point, Point, Point] {
	const headCX = head.x + w / 2;
	const headCY = head.y + h / 2;
	const tailCX = tail.x + w / 2;
	const tailCY = tail.y + h / 2;
	const minX = Math.min(headCX, tailCX) - w / 2;
	const maxX = Math.max(headCX, tailCX) + w / 2;
	const minY = Math.min(headCY, tailCY) - h / 2;
	const maxY = Math.max(headCY, tailCY) + h / 2;
	return [
		{ x: minX, y: minY },
		{ x: maxX, y: minY },
		{ x: maxX, y: maxY },
		{ x: minX, y: maxY }
	];
}

export function trailPolygonPoints(
	head: Point,
	tail: Point,
	w: number,
	h: number
): [Point, Point, Point, Point] {
	const dx = head.x - tail.x;
	const dy = head.y - tail.y;
	if (isStraightMove(dx, dy)) {
		return straightTrailPoints(head, tail, w, h);
	}
	return diagonalTrailPoints(head, tail, w, h);
}

export function pointsToSvg(points: readonly Point[]): string {
	return points.map((p) => `${p.x.toFixed(1)},${p.y.toFixed(1)}`).join(' ');
}
