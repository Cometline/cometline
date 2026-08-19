export type SeriesPoint = {
	date: string;
	cumulative: Record<string, number>;
};

export type AreaPath = {
	key: string;
	d: string;
};

export type AxisAnchor = 'start' | 'middle' | 'end';

export type XLabel = {
	x: number;
	label: string;
	anchor: AxisAnchor;
};

function seriesHsl(index: number, total: number): { h: number; s: number; l: number } {
	const count = Math.max(1, Math.trunc(total) || 1);
	return {
		h: (12 + (index * 360) / count) % 360,
		s: 36,
		l: index % 2 === 0 ? 39 : 57
	};
}

function hsl({ h, s, l }: { h: number; s: number; l: number }): string {
	return `hsl(${h} ${s}% ${l}%)`;
}

export function seriesColor(index: number, total: number): string {
	return hsl(seriesHsl(index, total));
}

export function seriesStroke(index: number, total: number): string {
	const tone = seriesHsl(index, total);
	return hsl({ ...tone, l: Math.max(22, tone.l - 14) });
}

export const PAD_LEFT = 36;
export const PAD_RIGHT = 20;
const PAD_TOP = 12;
const PAD_BOTTOM = 24;

export function singleDayBarBounds(width: number): { x0: number; x1: number } {
	const innerW = Math.max(1, width - PAD_LEFT - PAD_RIGHT);
	const barW = Math.min(72, Math.max(36, innerW * 0.16));
	const cx = PAD_LEFT + innerW / 2;
	return { x0: cx - barW / 2, x1: cx + barW / 2 };
}

function xAt(index: number, count: number, width: number): number {
	const innerW = Math.max(1, width - PAD_LEFT - PAD_RIGHT);
	if (count <= 1) return PAD_LEFT + innerW / 2;
	return PAD_LEFT + (index / (count - 1)) * innerW;
}

function labelAnchor(index: number, count: number): AxisAnchor {
	if (count <= 1) return 'middle';
	if (index === 0) return 'start';
	if (index === count - 1) return 'end';
	return 'middle';
}

export function stackedAreaPaths(
	points: SeriesPoint[],
	keys: string[],
	width: number,
	height: number
): AreaPath[] {
	if (points.length === 0 || keys.length === 0 || width <= 0 || height <= 0) {
		return [];
	}
	const innerH = Math.max(1, height - PAD_TOP - PAD_BOTTOM);
	const max = Math.max(
		1,
		...points.map((point) => keys.reduce((sum, key) => sum + (point.cumulative[key] ?? 0), 0))
	);
	const yAt = (value: number) => PAD_TOP + innerH * (1 - value / max);

	return keys.map((key, seriesIndex) => {
		const tops: Array<{ x: number; y: number }> = [];
		const bottoms: Array<{ x: number; y: number }> = [];
		const count = points.length;
		for (let i = 0; i < count; i += 1) {
			let below = 0;
			for (let j = 0; j < seriesIndex; j += 1) {
				below += points[i]?.cumulative[keys[j] ?? ''] ?? 0;
			}
			const value = points[i]?.cumulative[key] ?? 0;
			const yTop = yAt(below + value);
			const yBottom = yAt(below);
			if (count === 1) {
				const { x0, x1 } = singleDayBarBounds(width);
				tops.push({ x: x0, y: yTop }, { x: x1, y: yTop });
				bottoms.push({ x: x0, y: yBottom }, { x: x1, y: yBottom });
				continue;
			}
			tops.push({ x: xAt(i, count, width), y: yTop });
			bottoms.push({ x: xAt(i, count, width), y: yBottom });
		}
		const d = [
			`M ${tops[0]?.x ?? 0} ${tops[0]?.y ?? 0}`,
			...tops.slice(1).map((p) => `L ${p.x} ${p.y}`),
			...bottoms
				.slice()
				.reverse()
				.map((p) => `L ${p.x} ${p.y}`),
			'Z'
		].join(' ');
		return { key, d };
	});
}

export function xLabels(points: SeriesPoint[], width: number): XLabel[] {
	if (points.length === 0) return [];
	const step = points.length <= 8 ? 1 : Math.ceil(points.length / 7);
	return points.flatMap((point, index) => {
		if (index % step !== 0 && index !== points.length - 1) return [];
		return [
			{
				x: xAt(index, points.length, width),
				label: point.date.slice(5),
				anchor: labelAnchor(index, points.length)
			}
		];
	});
}

export function yLabels(points: SeriesPoint[], keys: string[], height: number): Array<{ y: number; label: number }> {
	const max = Math.max(
		1,
		...points.map((point) => keys.reduce((sum, key) => sum + (point.cumulative[key] ?? 0), 0))
	);
	const innerH = Math.max(1, height - PAD_TOP - PAD_BOTTOM);
	return [0, 0.5, 1].map((frac) => ({
		y: PAD_TOP + innerH * (1 - frac),
		label: Math.round(max * frac)
	}));
}

export function nearestPointIndex(points: SeriesPoint[], width: number, clientX: number): number {
	if (points.length === 0) return -1;
	if (points.length === 1) return 0;
	const innerW = Math.max(1, width - PAD_LEFT - PAD_RIGHT);
	const ratio = (clientX - PAD_LEFT) / innerW;
	return Math.min(points.length - 1, Math.max(0, Math.round(ratio * (points.length - 1))));
}
