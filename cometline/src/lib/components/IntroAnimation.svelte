<script lang="ts">
	import { fade } from 'svelte/transition';
	import { onMount } from 'svelte';
	import { shellStore } from '$lib/stores/shell.svelte';
	import { settingsStore } from '$lib/stores/settings.svelte';
	import { projectAvatarSrc, projectAvatarSrcset } from '$lib/project-icon';
	import { rectStyle } from '$lib/first-turn-flight';

	// ──────────────────────────────────────────────────────────────────────────
	// Cometline first-run intro.
	// Aesthetic: elegant + vintage (warm cream/gold title card, film grain,
	// vignette, hairline double-rule frame) over heavy-tech motion (a warp
	// starfield and a comet that streaks in and ignites the wordmark, ringed by
	// the user's configured hero-glow color).
	//
	// The sequence is timeline-driven via requestAnimationFrame so every beat is
	// eased; text reveals layer on top with CSS. Honors prefers-reduced-motion,
	// is skippable (Esc / click), and replayable from Settings → About.
	// ──────────────────────────────────────────────────────────────────────────

	// Beat timing (ms) — the spine of the cinematic.
	const T = {
		spaceIn: 700, // deep-space + grain fade in
		comet: 1500, // comet streaks toward center and ignites
		ring: 2300, // orbital ring forms around the mark
		wordmark: 2300, // "Cometline" resolves
		tagline: 3100, // tagline types in
		hold: 4500, // hold the title card
		total: 5400 // begin graceful exit
	};

	let canvas = $state<HTMLCanvasElement | null>(null);
	let phase = $state<'run' | 'exit'>('run');

	// Reactive text reveals are driven off a single elapsed clock.
	let elapsed = $state(0);
	let reducedMotion = false;
	let raf = 0;
	let finished = false;

	// Project icon that appears in the title card and flies to the hero avatar.
	let projectIconRef = $state<HTMLImageElement | null>(null);
	let showFlyIcon = $state(false);
	let flyIconStyle = $state('');

	let iconVariant = $derived(settingsStore.settings.app.iconVariant);

	const GLOW = () => settingsStore.settings.appearance.heroComposer.glowColor || '#72c0ff';

	function readCssVar(name: string, fallback: string): string {
		if (typeof window === 'undefined') return fallback;
		const v = getComputedStyle(document.documentElement).getPropertyValue(name).trim();
		return v || fallback;
	}

	// Convert any CSS color to {r,g,b} via a throwaway canvas pixel.
	function toRgb(color: string): { r: number; g: number; b: number } {
		const c = document.createElement('canvas');
		c.width = c.height = 1;
		const ctx = c.getContext('2d');
		if (!ctx) return { r: 114, g: 192, b: 255 };
		ctx.fillStyle = color;
		ctx.fillRect(0, 0, 1, 1);
		const [r, g, b] = ctx.getImageData(0, 0, 1, 1).data;
		return { r, g, b };
	}

	// easeOutCubic for cinematic deceleration.
	const easeOut = (t: number) => 1 - Math.pow(1 - t, 3);
	const clamp01 = (t: number) => Math.max(0, Math.min(1, t));
	// Normalize an absolute elapsed time into a 0..1 progress across [start,end].
	const seg = (now: number, start: number, end: number) =>
		clamp01((now - start) / (end - start));

	function captureIconFlightOrigin() {
		if (reducedMotion || !projectIconRef) return;
		const from = projectIconRef.getBoundingClientRect();
		const target = document.querySelector('.empty-state .avatar');
		if (!(target instanceof HTMLElement)) return;
		flyIconStyle = rectStyle(from, target.getBoundingClientRect());
	}

	function complete() {
		if (finished) return;
		finished = true;
		cancelAnimationFrame(raf);
		// Capture the icon origin before exit transitions change layout.
		captureIconFlightOrigin();
		phase = 'exit';
		// Hand the intro icon off to a fixed flying particle.
		if (flyIconStyle) showFlyIcon = true;
		// Persist the "seen" flag (no-op if already seen / replay).
		void settingsStore.markIntroSeen().catch(() => {});
		// Let the fade-out transition play before unmounting.
		setTimeout(() => shellStore.closeIntro(), reducedMotion ? 0 : 760);
	}

	function skip() {
		complete();
	}

	onMount(() => {
		reducedMotion =
			typeof window !== 'undefined' &&
			window.matchMedia('(prefers-reduced-motion: reduce)').matches;

		const onKey = (e: KeyboardEvent) => {
			if (e.key === 'Escape' || e.key === 'Enter' || e.key === ' ') {
				e.preventDefault();
				skip();
			}
		};
		window.addEventListener('keydown', onKey, true);

		if (reducedMotion) {
			// Reduced motion: show the static title card briefly, then leave.
			elapsed = T.tagline + 200;
			const t = setTimeout(complete, 1800);
			return () => {
				clearTimeout(t);
				window.removeEventListener('keydown', onKey, true);
			};
		}

		const el = canvas;
		const ctx0 = el?.getContext('2d', { alpha: false }) ?? null;
		if (!el || !ctx0) {
			const t = setTimeout(complete, 1800);
			return () => {
				clearTimeout(t);
				window.removeEventListener('keydown', onKey, true);
			};
		}

		const ctx: CanvasRenderingContext2D = ctx0;
		const dpr = Math.min(window.devicePixelRatio || 1, 2);
		let W = 0;
		let H = 0;
		const resize = () => {
			W = window.innerWidth;
			H = window.innerHeight;
			el.width = Math.floor(W * dpr);
			el.height = Math.floor(H * dpr);
			el.style.width = W + 'px';
			el.style.height = H + 'px';
			ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
		};
		resize();
		window.addEventListener('resize', resize);

		// Palette: hero glow blue + tomlord.io ink, on warm paper.
		const glow = toRgb(GLOW()); // hero glow blue (#72c0ff)
		const ink = toRgb(readCssVar('--intro-blue', '#4078f2')); // tomlord accent
		const inkDeep = toRgb(readCssVar('--intro-blue-deep', '#0184bc'));
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

		const start = performance.now();

		function frame(nowAbs: number) {
			const now = nowAbs - start;
			elapsed = now;
			const cx = W / 2;
			const cy = H / 2;

			const sheetFade = seg(now, 0, T.spaceIn);
			const ringForm = seg(now, T.comet, T.ring);

			// 1. Warm paper sheet — soft top-down gradient like the icon ground.
			const sheet = ctx.createLinearGradient(0, 0, 0, H);
			sheet.addColorStop(0, bg);
			sheet.addColorStop(1, bgDeep);
			ctx.fillStyle = sheet;
			ctx.fillRect(0, 0, W, H);

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
			const vig = ctx.createRadialGradient(cx, cy, H * 0.25, cx, cy, Math.max(W, H) * 0.78);
			vig.addColorStop(0, 'rgba(40,46,70,0)');
			vig.addColorStop(1, 'rgba(40,46,70,0.14)');
			ctx.fillStyle = vig;
			ctx.fillRect(0, 0, W, H);

			// 7. Paper-fiber grain — multiplied so it darkens like real stock.
			if (grain) {
				ctx.globalAlpha = 0.05;
				ctx.globalCompositeOperation = 'multiply';
				const ox = (Math.random() * 160) | 0;
				const oy = (Math.random() * 160) | 0;
				for (let x = -ox; x < W; x += 160) {
					for (let y = -oy; y < H; y += 160) {
						ctx.drawImage(grain, x, y);
					}
				}
				ctx.globalCompositeOperation = 'source-over';
				ctx.globalAlpha = 1;
			}

			if (now >= T.total) {
				complete();
				return;
			}
			raf = requestAnimationFrame(frame);
		}

		raf = requestAnimationFrame(frame);

		return () => {
			cancelAnimationFrame(raf);
			window.removeEventListener('resize', resize);
			window.removeEventListener('keydown', onKey, true);
		};
	});

	// CSS-driven text reveals, gated on the same clock as the canvas.
	let showWordmark = $derived(elapsed >= T.wordmark - 250);
	let showTagline = $derived(elapsed >= T.tagline - 150);
	let showHint = $derived(elapsed >= T.spaceIn + 200 && phase === 'run');
</script>

{#if showFlyIcon}
	<div class="fly-icon" style={flyIconStyle} aria-hidden="true">
		<img
			src={projectAvatarSrc(iconVariant, 192)}
			srcset={projectAvatarSrcset(iconVariant)}
			sizes="82px"
			alt=""
		/>
	</div>
{/if}

<div
	class="intro"
	class:is-exit={phase === 'exit'}
	role="button"
	tabindex="0"
	aria-label="Skip intro"
	onclick={skip}
	onkeydown={() => {}}
	transition:fade={{ duration: phase === 'exit' ? 0 : 200 }}
>
	<canvas bind:this={canvas} class="stage" aria-hidden="true"></canvas>

	<!-- Hairline double-rule frame: old-cinema title card. -->
	<div class="frame" aria-hidden="true"></div>

	<div class="card">
		<img
			bind:this={projectIconRef}
			class="project-icon"
			class:in={showWordmark}
			class:is-flying={showFlyIcon}
			src={projectAvatarSrc(iconVariant, 192)}
			srcset={projectAvatarSrcset(iconVariant)}
			sizes="82px"
			alt=""
		/>
		<h1 class="wordmark" class:in={showWordmark}>
			<span class="lead">Comet</span><span class="trail">line</span>
		</h1>
		<p class="tagline" class:in={showTagline}>A thought, a task, a file — Cometline continues.</p>
	</div>

	<button class="skip" class:in={showHint} onclick={skip}>Press Esc to skip</button>
</div>

<style>
	.intro {
		position: fixed;
		inset: 0;
		z-index: 90;
		background: var(--intro-bg, #fafafa);
		overflow: hidden;
		cursor: pointer;
		display: grid;
		place-items: center;
		transition: opacity 760ms var(--ease-intro, ease);
	}

	.intro.is-exit {
		opacity: 0;
	}

	.stage {
		position: absolute;
		inset: 0;
		display: block;
	}

	/* Vintage double-rule border in blue ink, inset from the edges. */
	.frame {
		position: absolute;
		inset: 26px;
		border: 1px solid color-mix(in srgb, var(--intro-blue, #4078f2) 38%, transparent);
		border-radius: 4px;
		pointer-events: none;
		opacity: 0;
		animation: frame-in 1.2s var(--ease-intro, ease) 0.5s forwards;
	}

	.frame::after {
		content: '';
		position: absolute;
		inset: 5px;
		border: 1px solid color-mix(in srgb, var(--intro-blue, #4078f2) 18%, transparent);
		border-radius: 2px;
	}

	@keyframes frame-in {
		from {
			opacity: 0;
			transform: scale(1.02);
		}
		to {
			opacity: 1;
			transform: scale(1);
		}
	}

	.card {
		position: relative;
		z-index: 2;
		text-align: center;
		transform: translateY(58px);
		pointer-events: none;
		user-select: none;
	}

	.project-icon {
		display: block;
		width: 120px;
		height: 120px;
		margin: 0 auto 22px;
		border-radius: 50%;
		box-shadow: var(--shadow-card);
		opacity: 0;
		transform: scale(0.92);
		transition:
			opacity 0.9s var(--ease-intro, ease),
			transform 0.9s var(--ease-intro, ease);
	}

	.project-icon.in {
		opacity: 1;
		transform: scale(1);
	}

	.project-icon.is-flying {
		opacity: 0;
		transition: none;
	}

	.wordmark {
		margin: 0;
		font-family: 'Hoefler Text', 'Iowan Old Style', 'Palatino Linotype', Palatino, Georgia, serif;
		font-weight: 600;
		letter-spacing: 0.12em;
		font-size: clamp(38px, 6.4vw, 76px);
		color: var(--intro-ink, #383a42);
		opacity: 0;
		filter: blur(8px);
		transform: translateY(10px);
		transition:
			opacity 0.9s var(--ease-intro, ease),
			filter 0.9s var(--ease-intro, ease),
			transform 0.9s var(--ease-intro, ease);
		text-shadow: 0 1px 0 rgba(255, 255, 255, 0.6);
	}

	.wordmark.in {
		opacity: 1;
		filter: blur(0);
		transform: translateY(0);
	}

	.wordmark .lead {
		color: var(--intro-ink, #383a42);
	}

	.wordmark .trail {
		color: var(--intro-ink, #383a42);
	}

	.tagline {
		margin: 18px 0 0;
		font-family: 'Hoefler Text', 'Iowan Old Style', Georgia, serif;
		font-style: italic;
		font-size: clamp(13px, 1.7vw, 17px);
		letter-spacing: 0.04em;
		color: var(--intro-ink-soft, rgba(56, 58, 66, 0.55));
		opacity: 0;
		transform: translateY(8px);
		transition:
			opacity 1s var(--ease-intro, ease),
			transform 1s var(--ease-intro, ease);
	}

	.tagline.in {
		opacity: 1;
		transform: translateY(0);
	}

	.skip {
		position: absolute;
		bottom: 42px;
		left: 50%;
		transform: translateX(-50%);
		z-index: 3;
		border: 0;
		background: transparent;
		color: var(--intro-ink-soft, rgba(56, 58, 66, 0.55));
		font-size: 11px;
		letter-spacing: 0.16em;
		text-transform: uppercase;
		opacity: 0;
		transition: opacity 0.8s var(--ease-intro, ease);
	}

	.skip.in {
		opacity: 0.7;
	}

	.skip:hover {
		opacity: 1;
		color: var(--intro-blue, #4078f2);
	}

	.is-exit .card {
		transition:
			transform 0.76s var(--ease-intro, ease),
			opacity 0.6s var(--ease-intro, ease);
		transform: translateY(58px) scale(1.04);
		opacity: 0;
	}

	.fly-icon {
		position: fixed;
		z-index: 100;
		pointer-events: none;
		transform-origin: top left;
		border-radius: 50%;
		overflow: hidden;
		background: linear-gradient(145deg, #ffffff, #eef2f6);
		box-shadow: 0 5px 14px rgba(15, 23, 42, 0.06);
		animation: intro-icon-flight 560ms var(--ease-smooth) forwards;
	}

	.fly-icon img {
		width: 100%;
		height: 100%;
		object-fit: cover;
		border-radius: 50%;
		display: block;
	}

	@keyframes intro-icon-flight {
		from {
			transform: translate3d(0, 0, 0) scale(1, 1);
		}
		to {
			transform: translate3d(var(--flight-x), var(--flight-y), 0)
				scale(var(--flight-sx), var(--flight-sy));
		}
	}

	@media (prefers-reduced-motion: reduce) {
		.frame,
		.wordmark,
		.tagline,
		.skip {
			animation: none !important;
			transition: opacity 0.3s linear !important;
		}
		.wordmark,
		.tagline {
			filter: none;
			transform: none;
		}
	}
</style>
