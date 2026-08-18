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

export const PAD_LEFT = 36;
export const PAD_RIGHT = 20;
const PAD_TOP = 12;
const PAD_BOTTOM = 24;

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
		for (let i = 0; i < points.length; i += 1) {
			let below = 0;
			for (let j = 0; j < seriesIndex; j += 1) {
				below += points[i]?.cumulative[keys[j] ?? ''] ?? 0;
			}
			const value = points[i]?.cumulative[key] ?? 0;
			tops.push({ x: xAt(i, points.length, width), y: yAt(below + value) });
			bottoms.push({ x: xAt(i, points.length, width), y: yAt(below) });
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
