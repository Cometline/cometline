<script lang="ts">
	import { fade } from 'svelte/transition';
	import { onMount } from 'svelte';
	import { shellStore } from '#lib/stores/shell.svelte.js';
	import { settingsStore } from '#lib/stores/settings.svelte.js';
	import {
		resolvePersona,
		personaAvatarSrcset as builtinAvatarSrcset
	} from '#lib/personas/index.js';
	import { personaAvatarCache } from '#lib/personas/avatar-cache.svelte.js';
	import { rectStyle } from '#lib/features/chat/first-turn-flight.js';
	import { normalizeHeroComposerAppearance } from '#lib/hero-composer-appearance.js';
	import { INTRO_BEATS as T } from '#lib/features/shell/intro-timeline.js';
	import { startIntroCanvas, type IntroCanvasRun } from '#lib/features/shell/intro-canvas.js';
	import IntroTitleCard from './intro/IntroTitleCard.svelte';

	// ──────────────────────────────────────────────────────────────────────────
	// Cometline first-run intro.
	// Aesthetic: warm ivory paper, cobalt editorial print, film grain, vignette,
	// and a hairline double-rule frame. A printed comet field sits as a faint
	// wash behind the title card; a single ink stroke then streaks in and
	// ignites the wordmark, ringed by the user's configured hero-glow color.
	//
	// The sequence is timeline-driven via requestAnimationFrame so every beat is
	// eased; text reveals layer on top with CSS. Honors prefers-reduced-motion,
	// is skippable (Esc / click), and replayable from Settings → About.
	// ──────────────────────────────────────────────────────────────────────────

	let canvas = $state<HTMLCanvasElement | null>(null);
	let phase = $state<'run' | 'exit'>('run');

	// Reactive text reveals are driven off a single elapsed clock.
	let elapsed = $state(0);
	let reducedMotion = false;
	let canvasRun: IntroCanvasRun | null = null;
	let finished = false;

	// Project icon that appears in the title card and flies to the hero avatar.
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
		canvasRun?.cancelFrame();
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

		canvasRun = startIntroCanvas(el, ctx0, {
			heroGlowColor,
			onFrame: (now) => {
				elapsed = now;
			},
			onComplete: complete
		});

		return () => {
			canvasRun?.destroy();
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
		<img src={avatarSrc} srcset={avatarSrcset} sizes="82px" alt="" />
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

	<IntroTitleCard
		bind:projectIconRef
		{avatarSrc}
		{avatarSrcset}
		{showWordmark}
		{showTagline}
		{showFlyIcon}
	/>

	<button class="skip" class:in={showHint} onclick={skip}>Press Esc to skip</button>
</div>

<style>
	.intro {
		position: fixed;
		inset: 0;
		z-index: 90;
		background: var(--intro-bg, var(--color-fafafa));
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

	/* Vintage double-rule border in hero-glow ink, inset from the edges. */
	.frame {
		position: absolute;
		inset: 26px;
		border: 1px solid
			color-mix(
				in srgb,
				var(--hero-composer-glow-color, var(--color-72c0ff)) 38%,
				transparent
			);
		border-radius: 4px;
		pointer-events: none;
		opacity: 0;
		animation: frame-in 1.2s var(--ease-intro, ease) 0.5s forwards;
	}

	.frame::after {
		content: '';
		position: absolute;
		inset: 5px;
		border: 1px solid
			color-mix(
				in srgb,
				var(--hero-composer-glow-color, var(--color-72c0ff)) 18%,
				transparent
			);
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

	.skip {
		position: absolute;
		bottom: 42px;
		left: 50%;
		transform: translateX(-50%);
		z-index: 3;
		border: 0;
		background: transparent;
		color: color-mix(
			in srgb,
			var(--hero-composer-glow-color, var(--color-72c0ff)) 35%,
			var(--intro-ink, var(--intro-ink))
		);
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
		color: var(--hero-composer-glow-color, var(--color-72c0ff));
	}

	.fly-icon {
		position: fixed;
		z-index: 100;
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
		.frame,
		.skip {
			animation: none !important;
			transition: opacity 0.3s linear !important;
		}
	}
</style>
