<script lang="ts">
	import { fade } from 'svelte/transition';
	import { onMount, tick } from 'svelte';
	import { shellStore } from '#lib/stores/shell.svelte.js';
	import { settingsStore } from '#lib/stores/settings.svelte.js';
	import { normalizeHeroComposerAppearance } from '#lib/hero-composer-appearance.js';
	import {
		computeIntroV2Motion,
		introV2Complete,
		type IntroV2Motion
	} from '#lib/features/shell/intro-v2-motion.js';
	/** When set (intro lab only), freeze the timeline at this elapsed ms. */
	let { debugFrozenElapsed = null }: { debugFrozenElapsed?: number | null } = $props();

	let phase = $state<'run' | 'exit'>('run');
	let motion = $state<IntroV2Motion>(computeIntroV2Motion(0, false));
	let reducedMotion = false;
	let finished = false;
	let skipRef = $state<HTMLButtonElement | null>(null);
	let raf = 0;
	let start = 0;

	let heroGlowColor = $derived(
		normalizeHeroComposerAppearance(settingsStore.settings.appearance.heroComposer).glowColor
	);

	let shellStyle = $derived(
		[
			`--win-scale:${motion.windowScale}`,
			`--win-radius:${motion.windowRadius}px`,
			`--backdrop-alpha:${motion.backdropAlpha}`,
			`--opener-opacity:${motion.openerOpacity}`,
			`--comet-opacity:${motion.cometOpacity}`,
			`--comet-lift:${motion.cometLift}`,
			`--wordmark-opacity:${motion.wordmarkOpacity}`,
			`--wordmark-blur:${motion.wordmarkBlur}px`,
			`--sky-blur:${motion.skyBlur}px`,
			`--sky-wash:${motion.skyWash}`,
			`--welcome-opacity:${motion.welcomeOpacity}`,
			`--welcome-scale:${motion.welcomeScale}`,
			`--shadow-strength:${motion.shadowStrength}`,
			`--hero-glow:${heroGlowColor}`
		].join(';')
	);

	function complete() {
		if (finished) return;
		finished = true;
		cancelAnimationFrame(raf);
		phase = 'exit';
		void settingsStore.markIntroSeen().catch(() => {});
		setTimeout(() => shellStore.closeIntro(), reducedMotion ? 0 : 720);
	}

	function skip(event?: Event) {
		event?.stopPropagation();
		complete();
	}

	function onKeydown(event: KeyboardEvent) {
		if (event.key === 'Escape' || event.key === 'Enter') {
			event.preventDefault();
			skip();
		}
	}

	function frame(now: number) {
		const elapsed = now - start;
		motion = computeIntroV2Motion(elapsed, reducedMotion);
		if (introV2Complete(elapsed, reducedMotion)) {
			complete();
			return;
		}
		raf = requestAnimationFrame(frame);
	}

	onMount(() => {
		reducedMotion =
			typeof window !== 'undefined' &&
			window.matchMedia('(prefers-reduced-motion: reduce)').matches;

		window.addEventListener('keydown', onKeydown, true);
		void tick().then(() => skipRef?.focus());

		if (debugFrozenElapsed != null && Number.isFinite(debugFrozenElapsed)) {
			motion = computeIntroV2Motion(debugFrozenElapsed, reducedMotion);
			return () => window.removeEventListener('keydown', onKeydown, true);
		}

		start = performance.now();
		motion = computeIntroV2Motion(0, reducedMotion);
		raf = requestAnimationFrame(frame);

		return () => {
			cancelAnimationFrame(raf);
			window.removeEventListener('keydown', onKeydown, true);
		};
	});
</script>

<div
	class="intro-v2"
	class:is-exit={phase === 'exit'}
	style={shellStyle}
	role="presentation"
	transition:fade={{ duration: phase === 'exit' ? 0 : 160 }}
>
	<div class="backdrop" aria-hidden="true"></div>

	<div class="float-window">
		<div class="sky" aria-hidden="true">
			<div class="sky-gradient"></div>
			<div class="sky-clouds"></div>
			<div class="sky-sharp" style:background-image="url('/intro/visual-comet.jpg')"></div>
		</div>

		<div class="opener" class:hidden={motion.openerOpacity < 0.02} aria-hidden="true">
			<p class="opener-kicker">COMETLINE</p>
			<p class="opener-sub">presents</p>
		</div>

		<div class="brand" aria-hidden="true">
			<div class="comet-rise">
				<svg class="comet-glyph" viewBox="0 0 240 160" aria-hidden="true">
					<defs>
						<linearGradient id="comet-tail" x1="0%" y1="50%" x2="100%" y2="50%">
							<stop offset="0%" stop-color="rgba(255,255,255,0)" />
							<stop offset="100%" stop-color="var(--hero-glow)" />
						</linearGradient>
					</defs>
					<path
						d="M12 118 C 70 104, 118 86, 196 42"
						stroke="url(#comet-tail)"
						stroke-width="3"
						stroke-linecap="round"
						fill="none"
					/>
					<circle cx="196" cy="42" r="5.5" fill="white" />
					<circle cx="196" cy="42" r="12" fill="var(--hero-glow)" opacity="0.35" />
				</svg>
			</div>
			<h1 class="wordmark">
				<span class="lead">Comet</span><span class="trail">line</span>
			</h1>
		</div>

		<div class="welcome-panel" class:visible={motion.welcomeOpacity > 0.08}>
			<h2>Welcome to Cometline</h2>
			<p>
				Your local agent for chat, files, and long-running tasks — entirely on your machine.
			</p>
			<ul class="welcome-points" aria-hidden="true">
				<li>Connect providers and models</li>
				<li>Work in your project workspace</li>
				<li>Pick up where you left off</li>
			</ul>
			<button type="button" class="continue" onclick={() => skip()}> Continue </button>
		</div>
	</div>

	<button
		bind:this={skipRef}
		type="button"
		class="skip-pill"
		onclick={skip}
		aria-keyshortcuts="Enter Escape"
	>
		Skip Intro
		<span class="key" aria-hidden="true">↵</span>
	</button>
</div>

<style>
	.intro-v2 {
		position: fixed;
		inset: 0;
		z-index: 92;
		overflow: hidden;
		pointer-events: auto;
		transition: opacity 720ms var(--ease-intro, cubic-bezier(0.16, 1, 0.3, 1));
	}

	.intro-v2.is-exit {
		opacity: 0;
	}

	.backdrop {
		position: absolute;
		inset: 0;
		background: radial-gradient(
			120% 90% at 50% 20%,
			color-mix(in srgb, var(--hero-glow) 12%, #0c1018),
			#06080d 70%
		);
		opacity: var(--backdrop-alpha, 0);
		transition: opacity 80ms linear;
	}

	.float-window {
		position: absolute;
		left: 50%;
		top: 50%;
		width: 100%;
		height: 100%;
		transform: translate3d(-50%, -50%, 0) scale(var(--win-scale, 1));
		transform-origin: center center;
		border-radius: var(--win-radius, 0);
		overflow: hidden;
		box-shadow: 0 32px 120px rgba(0, 0, 0, calc(var(--shadow-strength, 0) * 0.85));
		will-change: transform, border-radius;
	}

	.sky {
		position: absolute;
		inset: 0;
		overflow: hidden;
	}

	.sky-gradient,
	.sky-clouds,
	.sky-sharp {
		position: absolute;
		inset: -8%;
	}

	.sky-gradient {
		background: linear-gradient(175deg, #c9d8ec 0%, #9eb4d4 38%, #6f8fb8 70%, #4b688f 100%);
		opacity: calc(0.65 + var(--sky-wash, 0) * 0.35);
		filter: blur(var(--sky-blur, 36px));
		transform: scale(1.05);
		will-change: filter;
	}

	.sky-clouds {
		background:
			radial-gradient(60% 42% at 22% 28%, rgba(255, 255, 255, 0.68), transparent 70%),
			radial-gradient(48% 36% at 68% 22%, rgba(255, 255, 255, 0.52), transparent 72%),
			radial-gradient(55% 40% at 48% 62%, rgba(255, 255, 255, 0.34), transparent 74%);
		mix-blend-mode: screen;
		opacity: calc(var(--sky-wash, 0.4) * 0.95);
		filter: blur(calc(var(--sky-blur, 36px) * 0.65));
		will-change: filter, opacity;
	}

	.sky-sharp {
		background-size: cover;
		background-position: center 40%;
		opacity: calc(var(--sky-wash, 0) * 0.42);
		filter: blur(calc(var(--sky-blur, 36px) * 0.35)) saturate(1.05);
	}

	.opener {
		position: absolute;
		inset: 0;
		display: grid;
		place-content: center;
		text-align: center;
		color: white;
		opacity: var(--opener-opacity, 1);
		transform: translate3d(0, calc((1 - var(--opener-opacity, 1)) * -12px), 0);
		pointer-events: none;
	}

	.opener.hidden {
		visibility: hidden;
	}

	.opener-kicker {
		margin: 0;
		font-family: 'Hoefler Text', 'Iowan Old Style', Georgia, serif;
		font-size: clamp(13px, 1.6vw, 16px);
		letter-spacing: 0.34em;
		text-transform: uppercase;
		font-weight: 500;
	}

	.opener-sub {
		margin: 10px 0 0;
		font-family: 'Hoefler Text', 'Iowan Old Style', Georgia, serif;
		font-style: italic;
		font-size: clamp(18px, 2.4vw, 24px);
		letter-spacing: 0.08em;
		opacity: 0.92;
	}

	.brand {
		position: absolute;
		inset: 0;
		display: grid;
		place-content: center;
		text-align: center;
		gap: 6px;
		pointer-events: none;
	}

	.comet-rise {
		opacity: var(--comet-opacity, 0);
		transform: translate3d(0, calc(26px + (1 - var(--comet-lift, 0)) * 120px), 0);
	}

	.comet-glyph {
		width: min(240px, 54vw);
		height: auto;
		display: block;
		margin: 0 auto;
	}

	.wordmark {
		margin: 0;
		font-family:
			'Hoefler Text', 'Iowan Old Style', 'Palatino Linotype', Palatino, Georgia, serif;
		font-weight: 600;
		letter-spacing: 0.14em;
		font-size: clamp(40px, 7vw, 84px);
		color: white;
		opacity: var(--wordmark-opacity, 0);
		filter: blur(var(--wordmark-blur, 8px));
		text-shadow: 0 10px 40px rgba(0, 0, 0, 0.25);
	}

	.wordmark .trail {
		color: color-mix(in srgb, var(--hero-glow) 82%, white);
	}

	.welcome-panel {
		position: absolute;
		left: clamp(18px, 6vw, 54px);
		bottom: clamp(18px, 6vh, 42px);
		max-width: min(420px, 82vw);
		padding: 22px 24px 20px;
		border-radius: 16px;
		background: color-mix(in srgb, white 76%, transparent);
		backdrop-filter: blur(14px);
		box-shadow: 0 18px 42px rgba(15, 23, 42, 0.16);
		color: var(--intro-ink, #2a2d35);
		opacity: 0;
		transform: translate3d(0, 12px, 0) scale(var(--welcome-scale, 0.96));
		pointer-events: none;
	}

	.welcome-panel.visible {
		opacity: var(--welcome-opacity, 0);
		transform: translate3d(0, 0, 0) scale(var(--welcome-scale, 1));
		pointer-events: auto;
	}

	.welcome-panel h2 {
		margin: 0 0 8px;
		font-family: 'Hoefler Text', 'Iowan Old Style', Georgia, serif;
		font-size: clamp(22px, 3vw, 30px);
		font-weight: 600;
	}

	.welcome-panel p {
		margin: 0 0 14px;
		font-size: 13px;
		line-height: 1.55;
		color: color-mix(in srgb, var(--intro-ink, #2a2d35) 72%, white);
	}

	.welcome-points {
		margin: 0 0 18px;
		padding: 0 0 0 18px;
		font-size: 12px;
		line-height: 1.45;
		color: color-mix(in srgb, var(--intro-ink, #2a2d35) 62%, white);
	}

	.continue {
		border: 0;
		border-radius: 999px;
		padding: 9px 16px;
		font-size: 12px;
		font-weight: 600;
		letter-spacing: 0.04em;
		cursor: pointer;
		color: #0b1220;
		background: linear-gradient(
			135deg,
			color-mix(in srgb, var(--hero-glow) 88%, white),
			color-mix(in srgb, var(--hero-glow) 54%, white)
		);
	}

	.skip-pill {
		position: absolute;
		right: clamp(16px, 3vw, 28px);
		bottom: clamp(16px, 3vh, 28px);
		z-index: 3;
		display: inline-flex;
		align-items: center;
		gap: 8px;
		border: 0;
		border-radius: 999px;
		padding: 8px 14px;
		font-size: 12px;
		font-weight: 500;
		color: white;
		background: rgba(12, 16, 24, 0.72);
		cursor: pointer;
		box-shadow: 0 8px 24px rgba(0, 0, 0, 0.24);
	}

	.skip-pill:focus-visible {
		outline: 2px solid color-mix(in srgb, var(--hero-glow) 70%, white);
		outline-offset: 2px;
	}

	.skip-pill .key {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		min-width: 18px;
		height: 18px;
		border-radius: 4px;
		background: rgba(255, 255, 255, 0.14);
		font-size: 11px;
	}

	@media (prefers-reduced-motion: reduce) {
		.float-window,
		.sky-gradient,
		.sky-clouds,
		.comet-rise,
		.welcome-panel {
			transition: none !important;
		}
	}
</style>
