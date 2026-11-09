<script lang="ts">
	import { fade } from 'svelte/transition';
	import { onMount, tick } from 'svelte';
	import { shellStore } from '#lib/stores/shell.svelte.js';
	import { settingsStore } from '#lib/stores/settings.svelte.js';
	import {
		resolvePersona,
		personaAvatarSrcset as builtinAvatarSrcset
	} from '#lib/personas/index.js';
	import { personaAvatarCache } from '#lib/personas/avatar-cache.svelte.js';
	import { rectStyle } from '#lib/features/chat/first-turn-flight.js';
	import { normalizeHeroComposerAppearance } from '#lib/hero-composer-appearance.js';
	import {
		computeIntroV2Motion,
		introV2Complete,
		type IntroV2Motion
	} from '#lib/features/shell/intro-v2-motion.js';
	import {
		startIntroV2CometCanvas,
		type IntroV2CanvasRun
	} from '#lib/features/shell/intro-v2-canvas.js';
	import IntroTitleCard from './intro/IntroTitleCard.svelte';

	let {
		debugFrozenElapsed = null,
		previewMode = false
	}: {
		debugFrozenElapsed?: number | null;
		previewMode?: boolean;
	} = $props();

	let phase = $state<'run' | 'exit'>('run');
	let motion = $state<IntroV2Motion>(computeIntroV2Motion(0, false));
	let elapsed = $state(0);
	let reducedMotion = false;
	let finished = false;
	let skipRef = $state<HTMLButtonElement | null>(null);
	let cometCanvas = $state<HTMLCanvasElement | null>(null);
	let canvasRun: IntroV2CanvasRun | null = null;
	let raf = 0;
	let start = 0;

	let projectIconRef = $state<HTMLImageElement | null>(null);
	let showFlyIcon = $state(false);
	let flyIconStyle = $state('');

	let resolvedPersona = $derived(
		resolvePersona(
			settingsStore.settings.app.personaId,
			settingsStore.settings.app.personas.custom
		)
	);
	let avatarSrc = $derived(personaAvatarCache.avatarSrcFor(resolvedPersona, 192));
	let avatarSrcset = $derived(
		resolvedPersona.kind === 'builtin' ? builtinAvatarSrcset(resolvedPersona) : undefined
	);
	let heroGlowColor = $derived(
		normalizeHeroComposerAppearance(settingsStore.settings.appearance.heroComposer).glowColor
	);

	let showWordmark = $derived(motion.wordmarkOpacity > 0.08 || motion.iconOpacity > 0.15);
	let showTagline = $derived(motion.wordmarkOpacity > 0.65 && motion.welcomeOpacity < 0.2);
	let showHint = $derived(elapsed > 2200 && phase === 'run');

	let shellStyle = $derived(
		[
			`--win-scale:${motion.windowScale}`,
			`--win-radius:${motion.windowRadius}px`,
			`--backdrop-alpha:${motion.backdropAlpha}`,
			`--wordmark-opacity:${motion.wordmarkOpacity}`,
			`--icon-opacity:${motion.iconOpacity}`,
			`--sky-blur:${motion.skyBlur}px`,
			`--sky-wash:${motion.skyWash}`,
			`--sky-sharp-opacity:${motion.skySharpOpacity}`,
			`--welcome-opacity:${motion.welcomeOpacity}`,
			`--welcome-scale:${motion.welcomeScale}`,
			`--shadow-strength:${motion.shadowStrength}`,
			`--hero-glow:${heroGlowColor}`
		].join(';')
	);

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
		canvasRun?.cancelFrame();
		captureIconFlightOrigin();
		phase = 'exit';
		if (flyIconStyle) showFlyIcon = true;
		if (!previewMode) {
			void settingsStore.markIntroSeen().catch(() => {});
			setTimeout(() => shellStore.closeIntro(), reducedMotion ? 0 : 760);
		}
	}

	function skip(event?: Event) {
		event?.stopPropagation();
		complete();
	}

	function onKeydown(event: KeyboardEvent) {
		if (event.key === 'Escape' || event.key === 'Enter' || event.key === ' ') {
			event.preventDefault();
			skip();
		}
	}

	function frame(now: number) {
		const t = now - start;
		elapsed = t;
		motion = computeIntroV2Motion(t, reducedMotion);
		if (introV2Complete(t, reducedMotion)) {
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
			elapsed = debugFrozenElapsed;
			motion = computeIntroV2Motion(debugFrozenElapsed, reducedMotion);
			return () => window.removeEventListener('keydown', onKeydown, true);
		}

		if (reducedMotion) {
			elapsed = 1200;
			motion = computeIntroV2Motion(elapsed, true);
			const t = setTimeout(complete, 1500);
			return () => {
				clearTimeout(t);
				window.removeEventListener('keydown', onKeydown, true);
			};
		}

		start = performance.now();
		motion = computeIntroV2Motion(0, reducedMotion);

		raf = requestAnimationFrame(frame);

		return () => {
			cancelAnimationFrame(raf);
			canvasRun?.destroy();
			window.removeEventListener('keydown', onKeydown, true);
		};
	});

	$effect(() => {
		const el = cometCanvas;
		if (!el) return;
		const ctx = el.getContext('2d', { alpha: true });
		if (!ctx) return;

		canvasRun?.destroy();
		if (debugFrozenElapsed != null && Number.isFinite(debugFrozenElapsed)) {
			canvasRun = startIntroV2CometCanvas(el, ctx, {
				heroGlowColor,
				getElapsed: () => debugFrozenElapsed
			});
			return () => canvasRun?.destroy();
		}

		if (reducedMotion || finished) return;

		canvasRun = startIntroV2CometCanvas(el, ctx, {
			heroGlowColor,
			getElapsed: () => elapsed
		});
		return () => canvasRun?.destroy();
	});
</script>

{#if showFlyIcon}
	<div class="fly-icon" style={flyIconStyle} aria-hidden="true">
		<img src={avatarSrc} srcset={avatarSrcset} sizes="82px" alt="" />
	</div>
{/if}

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
			<div class="sky-layer sky-blurred">
				<div class="sky-gradient"></div>
				<div class="sky-clouds"></div>
			</div>
			<div
				class="sky-layer sky-sharp"
				style:background-image="url('/intro/visual-comet.jpg')"
			></div>
		</div>

		<canvas bind:this={cometCanvas} class="comet-canvas" aria-hidden="true"></canvas>

		{#if motion.openerOpacity > 0.04}
			<div class="opener" style:opacity={motion.openerOpacity} aria-hidden="true">
				<p class="opener-kicker">COMETLINE</p>
				<p class="opener-sub">presents</p>
			</div>
		{/if}

		<div
			class="brand-stage"
			style:opacity={Math.max(motion.wordmarkOpacity, motion.iconOpacity)}
		>
			<IntroTitleCard
				bind:projectIconRef
				{avatarSrc}
				{avatarSrcset}
				{showWordmark}
				{showTagline}
				{showFlyIcon}
			/>
		</div>

		{#if motion.welcomeOpacity > 0.05}
			<div
				class="welcome-panel"
				style:opacity={motion.welcomeOpacity}
				style:transform="translate(-50%, -50%) scale({motion.welcomeScale})"
			>
				<h2>Welcome to Cometline</h2>
				<p>
					Your local agent for chat, files, and long-running tasks — entirely on your
					machine.
				</p>
				<button type="button" class="continue" onclick={() => skip()}>Continue</button>
			</div>
		{/if}
	</div>

	<button
		bind:this={skipRef}
		type="button"
		class="skip-pill"
		class:in={showHint}
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
		z-index: 100000;
		overflow: hidden;
		pointer-events: auto;
		transition: opacity 760ms var(--ease-intro, cubic-bezier(0.16, 1, 0.3, 1));
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

	.sky-layer {
		position: absolute;
		inset: -10%;
	}

	.sky-blurred {
		opacity: calc(1 - var(--sky-sharp-opacity, 0) * 0.92);
		filter: blur(var(--sky-blur, 44px));
		transform: scale(1.06);
	}

	.sky-gradient {
		position: absolute;
		inset: 0;
		background: linear-gradient(165deg, #d4e2f4 0%, #a5bdd9 36%, #6f91bc 68%, #3d5f88 100%);
		opacity: calc(0.75 + var(--sky-wash, 0) * 0.25);
	}

	.sky-clouds {
		position: absolute;
		inset: 0;
		background:
			radial-gradient(58% 40% at 20% 26%, rgba(255, 255, 255, 0.82), transparent 72%),
			radial-gradient(46% 34% at 72% 20%, rgba(255, 255, 255, 0.62), transparent 74%),
			radial-gradient(52% 38% at 50% 64%, rgba(255, 255, 255, 0.46), transparent 76%);
		mix-blend-mode: screen;
		opacity: calc(var(--sky-wash, 0.5) * 0.98);
	}

	.sky-sharp {
		background-size: cover;
		background-position: center 42%;
		opacity: var(--sky-sharp-opacity, 0);
		filter: saturate(1.12) contrast(1.04);
		transform: scale(1.02);
	}

	.comet-canvas {
		position: absolute;
		inset: 0;
		width: 100%;
		height: 100%;
		pointer-events: none;
		z-index: 3;
	}

	.opener {
		position: absolute;
		inset: 0;
		display: grid;
		place-content: center;
		text-align: center;
		color: white;
		z-index: 4;
		pointer-events: none;
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

	.brand-stage {
		position: absolute;
		inset: 0;
		z-index: 5;
		display: grid;
		place-items: center;
		pointer-events: none;
		opacity: var(--wordmark-opacity, 0);
	}

	.brand-stage :global(.card) {
		transform: translateY(26px);
	}

	.brand-stage :global(.wordmark) {
		color: white;
		text-shadow:
			0 2px 0 rgba(0, 0, 0, 0.14),
			0 16px 48px rgba(0, 0, 0, 0.35);
		font-size: clamp(48px, 8.8vw, 104px);
		opacity: var(--wordmark-opacity, 0);
		filter: blur(0);
	}

	.brand-stage :global(.wordmark .trail) {
		color: color-mix(in srgb, var(--hero-glow) 70%, white);
	}

	.brand-stage :global(.tagline) {
		color: rgba(255, 255, 255, 0.82);
	}

	.brand-stage :global(.project-icon) {
		opacity: var(--icon-opacity, 0);
	}

	.welcome-panel {
		position: absolute;
		left: 50%;
		top: 50%;
		z-index: 6;
		max-width: min(400px, calc(100% - 56px));
		padding: 22px 24px 20px;
		border-radius: 16px;
		background: color-mix(in srgb, white 78%, transparent);
		backdrop-filter: blur(14px);
		box-shadow: 0 18px 42px rgba(15, 23, 42, 0.16);
		color: var(--intro-ink, #2a2d35);
		text-align: left;
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

	.continue {
		border: 0;
		border-radius: 999px;
		padding: 9px 16px;
		font-size: 12px;
		font-weight: 600;
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
		z-index: 8;
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
		opacity: 0.85;
	}

	.skip-pill.in {
		opacity: 1;
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

	.fly-icon {
		position: fixed;
		z-index: 100001;
		pointer-events: none;
		transform-origin: top left;
		border-radius: 50%;
		overflow: hidden;
		background: linear-gradient(145deg, var(--panel-bg), var(--color-eef2f6));
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
		.float-window,
		.sky-blurred,
		.brand-stage {
			transition: none !important;
		}
	}
</style>
