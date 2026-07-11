<script lang="ts">
	import type { Snippet } from 'svelte';
	import { tick } from 'svelte';

	let {
		active = true,
		children
	}: {
		active?: boolean;
		children: Snippet;
	} = $props();

	const showEffects = $derived(active);

	let frameEl = $state<HTMLDivElement | null>(null);
	let glowTravelPx = $state(0);
	let glowReady = $state(false);
	function measureGlowTravel() {
		if (!frameEl) return;
		const home = frameEl.closest('.chat-home');
		if (!home) return;
		const homeRect = home.getBoundingClientRect();
		const frameRect = frameEl.getBoundingClientRect();
		glowTravelPx = Math.max(0, Math.round(homeRect.bottom - frameRect.bottom));
		glowReady = true;
	}

	$effect(() => {
		if (!frameEl) return;
		void tick().then(measureGlowTravel);
		const ro = new ResizeObserver(() => measureGlowTravel());
		ro.observe(frameEl);
		const home = frameEl.closest('.chat-home');
		if (home instanceof HTMLElement) ro.observe(home);
		return () => ro.disconnect();
	});

	$effect(() => {
		if (active) {
			void tick().then(measureGlowTravel);
			return;
		}
		glowReady = false;
	});
</script>

<div
	class="hero-composer-frame"
	class:impact-ready={glowReady && active}
	bind:this={frameEl}
	style:--hero-glow-travel="{glowTravelPx}px"
>
	{#if showEffects}
		<div class="hero-composer-glow" class:ready={glowReady} aria-hidden="true"></div>
		<div class="hero-composer-ring" class:ready={glowReady} aria-hidden="true"></div>
	{/if}
	<div class="hero-composer-slot">
		{@render children()}
	</div>
</div>

<style>
	.hero-composer-frame {
		position: relative;
		width: min(var(--chat-composer-width), 100%);
		max-width: 100%;
		overflow: visible;
		transform-origin: center center;
	}

	.hero-composer-frame.impact-ready {
		animation: hero-composer-impact var(--duration-hero-impact-rise) var(--ease-smooth)
			var(--duration-hero-hit-delay) forwards;
	}

	.hero-composer-slot {
		position: relative;
		z-index: 1;
	}

	/* Popovers live inside the slot; lift it above the ring while any menu is open. */
	.hero-composer-frame:has(:global(.model-menu, .skill-command-menu)) .hero-composer-slot {
		z-index: 3;
	}

	.hero-composer-slot :global(.composer) {
		width: 100%;
	}

	.hero-composer-ring {
		position: absolute;
		inset: 0;
		z-index: 2;
		pointer-events: none;
		border-radius: 24px;
		border: 1px solid var(--hero-composer-ring);
		box-shadow: 0 0 0 1px rgba(255, 255, 255, 0.42) inset;
		clip-path: inset(100% 0 0 0 round 24px);
		opacity: 0;
	}

	.hero-composer-ring.ready {
		animation: hero-composer-ring-rise var(--duration-hero-ring-rise) var(--ease-smooth)
			var(--duration-hero-hit-delay) forwards;
	}

	.hero-composer-glow {
		position: absolute;
		inset: -16px -12px -10px;
		z-index: 0;
		pointer-events: none;
		border-radius: 24px;
		background:
			radial-gradient(
				ellipse 118% 92% at 50% 100%,
				var(--hero-composer-glow-strong),
				transparent 70%
			),
			radial-gradient(
				ellipse 88% 68% at 50% 0%,
				var(--hero-composer-glow-soft),
				transparent 74%
			);
		filter: blur(20px);
		opacity: 0;
		transform: translateY(var(--hero-glow-travel, 0px)) scaleY(0.35);
		transform-origin: center bottom;
	}

	.hero-composer-glow.ready {
		animation: hero-composer-glow-rise var(--duration-hero-glow-rise) var(--ease-smooth)
			forwards;
	}

	@keyframes hero-composer-glow-rise {
		from {
			opacity: 0;
			transform: translateY(var(--hero-glow-travel, 0px)) scaleY(0.35);
		}
		to {
			opacity: 1;
			transform: translateY(0) scaleY(1);
			box-shadow: 0 0 44px var(--hero-composer-glow-ring);
		}
	}

	@keyframes hero-composer-impact {
		from {
			transform: scale(1);
		}
		to {
			transform: scale(var(--hero-composer-impact-scale, 1.01));
		}
	}

	@keyframes hero-composer-ring-rise {
		from {
			opacity: 0;
		}
		to {
			clip-path: inset(0 0 0 0 round 24px);
			opacity: 1;
		}
	}

	@media (prefers-reduced-motion: reduce) {
		.hero-composer-frame.impact-ready {
			animation: none;
			transform: scale(var(--hero-composer-impact-scale, 1.01));
		}

		.hero-composer-ring {
			animation: none;
			clip-path: inset(0 0 0 0 round 24px);
			opacity: 1;
		}

		.hero-composer-glow {
			animation: none;
			opacity: 1;
			transform: none;
			box-shadow: 0 0 44px var(--hero-composer-glow-ring);
		}
	}
</style>
