import { darkenRgb, easeOut, seg, type Rgb } from './intro-timeline';
import { INTRO_V2_BEATS as T } from './intro-timeline-v2';

export type IntroV2CanvasRun = {
	cancelFrame: () => void;
	destroy: () => void;
};

function toRgb(color: string): Rgb {
	const c = document.createElement('canvas');
	c.width = c.height = 1;
	const ctx = c.getContext('2d');
	if (!ctx) return { r: 114, g: 192, b: 255 };
	ctx.fillStyle = color;
	ctx.fillRect(0, 0, 1, 1);
	const [r, g, b] = ctx.getImageData(0, 0, 1, 1).data;
	return { r, g, b };
}

/** Transparent overlay: animated comet head + tail sweeping the sky. */
export function startIntroV2CometCanvas(
	el: HTMLCanvasElement,
	ctx: CanvasRenderingContext2D,
	options: {
		heroGlowColor: string;
		getElapsed: () => number;
	}
): IntroV2CanvasRun {
	let raf = 0;
	const dpr = Math.min(window.devicePixelRatio || 1, 2);
	let W = 0;
	let H = 0;

	const resize = () => {
		const parent = el.parentElement;
		W = parent?.clientWidth ?? window.innerWidth;
		H = parent?.clientHeight ?? window.innerHeight;
		el.width = Math.floor(W * dpr);
		el.height = Math.floor(H * dpr);
		el.style.width = W + 'px';
		el.style.height = H + 'px';
		ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
	};
	resize();
	window.addEventListener('resize', resize);

	const glow = toRgb(options.heroGlowColor);
	const inkDeep = darkenRgb(glow, 0.52);

	function paint(now: number) {
		ctx.clearRect(0, 0, W, H);

		const streakAlpha =
			(1 - easeOut(seg(now, T.brandOut, T.brandOut + 700))) *
			(1 - easeOut(seg(now, T.welcomeIn - 400, T.welcomeOut)));
		const cometP = seg(now, T.cometStreakStart, T.cometStreakEnd);
		if (cometP <= 0 || streakAlpha <= 0.01) return;

		const e = easeOut(Math.min(1, cometP));
		// Sweep from top-left across the sky toward center-right (like a meteor).
		const sx = W * (-0.08 + 0.92 * e);
		const sy = H * (-0.06 + 0.72 * e);
		const ox = W * 0.02;
		const oy = H * 0.02;
		const tailLen = Math.max(W, H) * (0.36 + 0.26 * (1 - cometP));
		const ang = Math.atan2(sy - oy, sx - ox);
		const tx = sx - Math.cos(ang) * tailLen;
		const ty = sy - Math.sin(ang) * tailLen;

		ctx.save();
		ctx.globalAlpha = streakAlpha;

		const grad = ctx.createLinearGradient(tx, ty, sx, sy);
		grad.addColorStop(0, 'rgba(255,255,255,0)');
		grad.addColorStop(0.55, `rgba(${glow.r},${glow.g},${glow.b},0.35)`);
		grad.addColorStop(1, `rgba(${glow.r},${glow.g},${glow.b},0.98)`);
		ctx.strokeStyle = grad;
		ctx.lineWidth = 3.6;
		ctx.lineCap = 'round';
		ctx.beginPath();
		ctx.moveTo(tx, ty);
		ctx.lineTo(sx, sy);
		ctx.stroke();

		// Secondary filament tail for depth.
		ctx.strokeStyle = `rgba(255,255,255,${0.5 * streakAlpha})`;
		ctx.lineWidth = 1.2;
		ctx.beginPath();
		ctx.moveTo(tx + 8, ty + 5);
		ctx.lineTo(sx, sy);
		ctx.stroke();

		// Head bloom
		const bloom = ctx.createRadialGradient(sx, sy, 0, sx, sy, 42);
		bloom.addColorStop(0, `rgba(255,255,255,${0.85 * streakAlpha})`);
		bloom.addColorStop(0.45, `rgba(${glow.r},${glow.g},${glow.b},${0.45 * streakAlpha})`);
		bloom.addColorStop(1, 'rgba(255,255,255,0)');
		ctx.fillStyle = bloom;
		ctx.beginPath();
		ctx.arc(sx, sy, 42, 0, Math.PI * 2);
		ctx.fill();

		ctx.fillStyle = `rgba(${inkDeep.r},${inkDeep.g},${inkDeep.b},0.96)`;
		ctx.beginPath();
		ctx.arc(sx, sy, 4.6, 0, Math.PI * 2);
		ctx.fill();

		ctx.restore();
	}

	function frame() {
		paint(options.getElapsed());
		raf = requestAnimationFrame(frame);
	}

	raf = requestAnimationFrame(frame);

	return {
		cancelFrame: () => cancelAnimationFrame(raf),
		destroy: () => {
			cancelAnimationFrame(raf);
			window.removeEventListener('resize', resize);
		}
	};
}
