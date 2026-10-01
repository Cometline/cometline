export type Rgb = { r: number; g: number; b: number };

// Beat timing (ms) — the spine of the cinematic.
export const INTRO_BEATS = {
	spaceIn: 700, // paper sheet + printed field fade in
	comet: 1500, // comet streaks toward center and ignites
	ring: 2300, // orbital ring forms around the mark
	wordmark: 2300, // "Cometline" resolves
	tagline: 3100, // tagline types in
	hold: 4500, // hold the title card
	total: 5400 // begin graceful exit
};

export function darkenRgb({ r, g, b }: Rgb, factor: number): Rgb {
	return {
		r: Math.round(r * factor),
		g: Math.round(g * factor),
		b: Math.round(b * factor)
	};
}

// easeOutCubic for cinematic deceleration.
export const easeOut = (t: number) => 1 - Math.pow(1 - t, 3);
export const clamp01 = (t: number) => Math.max(0, Math.min(1, t));
// Normalize an absolute elapsed time into a 0..1 progress across [start,end].
export const seg = (now: number, start: number, end: number) =>
	clamp01((now - start) / (end - start));
