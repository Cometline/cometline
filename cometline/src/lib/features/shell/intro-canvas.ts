import { INTRO_BEATS as T, darkenRgb, easeOut, seg, type Rgb } from './intro-timeline';

export type IntroCanvasRun = {
	cancelFrame: () => void;
	destroy: () => void;
};

function readCssVar(name: string, fallback: string): string {
	if (typeof window === 'undefined') return fallback;
	const v = getComputedStyle(document.documentElement).getPropertyValue(name).trim();
	return v || fallback;
}

// Convert any CSS color to {r,g,b} via a throwaway canvas pixel.
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

export function startIntroCanvas(
	el: HTMLCanvasElement,
	ctx: CanvasRenderingContext2D,
	options: {
		heroGlowColor: string;
		onFrame: (now: number) => void;
		onComplete: () => void;
	}
): IntroCanvasRun {
	let raf = 0;
	const dpr = Math.min(window.devicePixelRatio || 1, 2);
	let W = 0;
	let H = 0;
	let sheetGradient: CanvasGradient | null = null;
	let vignetteGradient: CanvasGradient | null = null;
	let grainOffsetX = 0;
	let grainOffsetY = 0;
	let fieldImage: HTMLImageElement | null = null;
	let fieldReady = false;

	function rebuildStaticCanvasState() {
		const cx = W / 2;
		const cy = H / 2;
		sheetGradient = ctx.createLinearGradient(0, 0, 0, H);
		sheetGradient.addColorStop(0, bg);
		sheetGradient.addColorStop(1, bgDeep);

		vignetteGradient = ctx.createRadialGradient(
			cx,
			cy,
			H * 0.25,
			cx,
			cy,
			Math.max(W, H) * 0.78
		);
		vignetteGradient.addColorStop(0, 'rgba(40,46,70,0)');
		vignetteGradient.addColorStop(1, 'rgba(40,46,70,0.14)');
	}

	const resize = () => {
		W = window.innerWidth;
		H = window.innerHeight;
		el.width = Math.floor(W * dpr);
		el.height = Math.floor(H * dpr);
		el.style.width = W + 'px';
		el.style.height = H + 'px';
		ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
		if (sheetGradient && vignetteGradient) rebuildStaticCanvasState();
	};
	resize();
	window.addEventListener('resize', resize);

	const field = new Image();
	field.decoding = 'async';
	field.onload = () => {
		fieldImage = field;
		fieldReady = true;
	};
	field.src = '/intro/visual-comet.jpg';

	// Palette: user's hero-glow preset on warm paper.
	const glowHex = readCssVar('--hero-composer-glow-color', options.heroGlowColor);
	const glow = toRgb(glowHex);
	const ink = glow;
	const inkDeep = darkenRgb(glow, 0.58);
	const bg = readCssVar('--intro-bg', '#fafafa');
	const bgDeep = readCssVar('--intro-bg-deep', '#eef1f6');

	// Cross-hatched "blue sky" strokes — the signature texture from the
	// project icon's background. Each is a short diagonal pen mark; they
	// fade in as a field, then settle behind the title card.
	type Hatch = { x: number; y: number; len: number; ang: number; w: number; a: number };
	const HATCH_COUNT = Math.min(260, Math.floor((W * H) / 5200));
	const hatches: Hatch[] = Array.from({ length: HATCH_COUNT }, () => {
		const diagonal = Math.random() < 0.5 ? -0.86 : -0.62; // two hatch angles
		return {
			x: Math.random() * W,
			y: Math.random() * H,
			len: 16 + Math.random() * 34,
			ang: diagonal + (Math.random() - 0.5) * 0.18,
			w: 0.6 + Math.random() * 0.9,
			a: 0.05 + Math.random() * 0.12
		};
	});

	// Precompute a paper-fiber grain tile (used with 'multiply' so it
	// reads as paper texture darkening the sheet, not film highlight).
	const grain = document.createElement('canvas');
	grain.width = grain.height = 160;
	const gctx = grain.getContext('2d');
	if (gctx) {
		const img = gctx.createImageData(160, 160);
		for (let i = 0; i < img.data.length; i += 4) {
			const v = 205 + Math.random() * 50;
			img.data[i] = img.data[i + 1] = img.data[i + 2] = v;
			img.data[i + 3] = 255;
		}
		gctx.putImageData(img, 0, 0);
	}
	grainOffsetX = (Math.random() * 160) | 0;
	grainOffsetY = (Math.random() * 160) | 0;
	rebuildStaticCanvasState();

	const start = performance.now();

	function frame(nowAbs: number) {
		const now = nowAbs - start;
		options.onFrame(now);
		const cx = W / 2;
		const cy = H / 2;

		const sheetFade = seg(now, 0, T.spaceIn);
		const ringForm = seg(now, T.comet, T.ring);

		// 1. Warm paper sheet — soft top-down gradient like the icon ground.
		ctx.fillStyle = sheetGradient ?? bg;
		ctx.fillRect(0, 0, W, H);

		// Printed comet field from the landing-page editorial set.
		// Keep it faint so the title card stays on paper, not a poster.
		if (fieldReady && fieldImage && fieldImage.naturalWidth > 0) {
			const fieldAlpha = (0.22 + 0.16 * easeOut(ringForm)) * sheetFade;
			const scale = Math.max(W / fieldImage.naturalWidth, H / fieldImage.naturalHeight);
			const dw = fieldImage.naturalWidth * scale;
			const dh = fieldImage.naturalHeight * scale;
			// Bias toward the comet, but clamp so the print still covers the sheet.
			const dx = Math.min(0, Math.max(W - dw, (W - dw) / 2 + dw * 0.06));
			const dy = Math.min(0, Math.max(H - dh, (H - dh) / 2 - dh * 0.04));
			ctx.save();
			ctx.globalAlpha = fieldAlpha;
			ctx.drawImage(fieldImage, dx, dy, dw, dh);
			ctx.restore();

			const veil = ctx.createRadialGradient(cx, cy, H * 0.16, cx, cy, Math.max(W, H) * 0.62);
			veil.addColorStop(0, `rgba(250,250,250,${0.42 * sheetFade})`);
			veil.addColorStop(0.55, `rgba(250,250,250,${0.12 * sheetFade})`);
			veil.addColorStop(1, 'rgba(250,250,250,0)');
			ctx.fillStyle = veil;
			ctx.fillRect(0, 0, W, H);
		}

		// Gentle blue ink wash that breathes in behind the mark.
		const washA = (0.05 + 0.1 * easeOut(ringForm)) * sheetFade;
		const wash = ctx.createRadialGradient(cx, cy, 0, cx, cy, Math.max(W, H) * 0.6);
		wash.addColorStop(0, `rgba(${glow.r},${glow.g},${glow.b},${washA})`);
		wash.addColorStop(0.55, `rgba(${glow.r},${glow.g},${glow.b},${washA * 0.3})`);
		wash.addColorStop(1, 'rgba(255,255,255,0)');
		ctx.fillStyle = wash;
		ctx.fillRect(0, 0, W, H);

		// 2. Cross-hatched blue ink sky (the icon's signature texture).
		//    Strokes "ink in" progressively, then ease back so they sit
		//    quietly behind the title card.
		const hatchIn = easeOut(seg(now, 150, T.comet));
		const hatchSettle = 1 - 0.55 * easeOut(seg(now, T.ring, T.hold));
		ctx.lineCap = 'round';
		for (let i = 0; i < hatches.length; i++) {
			const h = hatches[i];
			// Stagger reveal across the field for a hand-drawn, inked-in feel.
			if (hatchIn <= i / hatches.length) continue;
			const a = h.a * sheetFade * hatchSettle;
			ctx.strokeStyle = `rgba(${ink.r},${ink.g},${ink.b},${a})`;
			ctx.lineWidth = h.w;
			ctx.beginPath();
			ctx.moveTo(h.x, h.y);
			ctx.lineTo(h.x + Math.cos(h.ang) * h.len, h.y + Math.sin(h.ang) * h.len);
			ctx.stroke();
		}

		// 3. Comet — a hero-glow-blue ink stroke drawn toward the center.
		const cometP = seg(now, 400, T.comet);
		if (cometP > 0 && cometP < 1) {
			const e = easeOut(cometP);
			const ox = -W * 0.15;
			const oy = -H * 0.12;
			const sx = ox + (cx - ox) * e;
			const sy = oy + (cy - oy) * e;
			const tailLen = 280 * (0.4 + 0.6 * (1 - cometP));
			const ang = Math.atan2(cy - oy, cx - ox);
			const tx = sx - Math.cos(ang) * tailLen;
			const ty = sy - Math.sin(ang) * tailLen;
			const grad = ctx.createLinearGradient(tx, ty, sx, sy);
			grad.addColorStop(0, 'rgba(255,255,255,0)');
			grad.addColorStop(1, `rgba(${glow.r},${glow.g},${glow.b},0.95)`);
			ctx.strokeStyle = grad;
			ctx.lineWidth = 2.6;
			ctx.beginPath();
			ctx.moveTo(tx, ty);
			ctx.lineTo(sx, sy);
			ctx.stroke();
			ctx.fillStyle = `rgba(${inkDeep.r},${inkDeep.g},${inkDeep.b},0.95)`;
			ctx.beginPath();
			ctx.arc(sx, sy, 3.4, 0, Math.PI * 2);
			ctx.fill();
		}

		// 4. Ink bloom — a soft blue wash blooms outward on arrival (light).
		const ignite = seg(now, T.comet - 120, T.comet + 380);
		if (ignite > 0) {
			const pulse = Math.sin(ignite * Math.PI) * (1 - seg(now, T.ring, T.hold));
			const bloom = ctx.createRadialGradient(cx, cy, 0, cx, cy, 240);
			bloom.addColorStop(0, `rgba(${glow.r},${glow.g},${glow.b},${0.4 * pulse})`);
			bloom.addColorStop(0.45, `rgba(${ink.r},${ink.g},${ink.b},${0.14 * pulse})`);
			bloom.addColorStop(1, 'rgba(255,255,255,0)');
			ctx.fillStyle = bloom;
			ctx.fillRect(0, 0, W, H);
		}

		// 5. Orbital ring — blue ink ring + deep-blue hairline rule.
		const ringP = seg(now, T.comet, T.ring);
		if (ringP > 0) {
			const er = easeOut(ringP);
			const radius = 132;
			const sweep = Math.PI * 2 * er;
			const rot = now * 0.00035;
			ctx.save();
			ctx.translate(cx, cy);
			ctx.rotate(rot);
			ctx.lineWidth = 1.6;
			ctx.strokeStyle = `rgba(${glow.r},${glow.g},${glow.b},${0.62 * er})`;
			ctx.shadowBlur = 14;
			ctx.shadowColor = `rgba(${glow.r},${glow.g},${glow.b},0.45)`;
			ctx.beginPath();
			ctx.arc(0, 0, radius, -Math.PI / 2, -Math.PI / 2 + sweep);
			ctx.stroke();
			// Inner deep-blue hairline — the vintage engraving rule.
			ctx.shadowBlur = 0;
			ctx.lineWidth = 1;
			ctx.strokeStyle = `rgba(${inkDeep.r},${inkDeep.g},${inkDeep.b},${0.4 * er})`;
			ctx.beginPath();
			ctx.arc(0, 0, radius - 10, -Math.PI / 2, -Math.PI / 2 + sweep);
			ctx.stroke();
			// Orbiting node at the sweep head.
			const hx = Math.cos(-Math.PI / 2 + sweep) * radius;
			const hy = Math.sin(-Math.PI / 2 + sweep) * radius;
			ctx.fillStyle = `rgba(${glow.r},${glow.g},${glow.b},${0.95 * er})`;
			ctx.shadowBlur = 12;
			ctx.shadowColor = `rgba(${glow.r},${glow.g},${glow.b},0.85)`;
			ctx.beginPath();
			ctx.arc(hx, hy, 3, 0, Math.PI * 2);
			ctx.fill();
			ctx.restore();
		}

		// 6. Paper vignette — soft warm edges darkening toward the corners.
		ctx.fillStyle = vignetteGradient ?? 'rgba(40,46,70,0.14)';
		ctx.fillRect(0, 0, W, H);

		// 7. Paper-fiber grain — multiplied so it darkens like real stock.
		if (grain) {
			ctx.globalAlpha = 0.05;
			ctx.globalCompositeOperation = 'multiply';
			for (let x = -grainOffsetX; x < W; x += 160) {
				for (let y = -grainOffsetY; y < H; y += 160) {
					ctx.drawImage(grain, x, y);
				}
			}
			ctx.globalCompositeOperation = 'source-over';
			ctx.globalAlpha = 1;
		}

		if (now >= T.total) {
			options.onComplete();
			return;
		}
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
