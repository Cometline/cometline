<script lang="ts">
	import ThinkingIndicator from '$lib/components/ThinkingIndicator.svelte';
	import { createStickyThinkingIndicator } from '$lib/components/sticky-thinking-variant.svelte';

	let {
		label,
		detail,
		color,
		phase
	}: {
		label: string;
		detail: string;
		color?: string;
		/** Wire turn-status phase; drives sticky celestial variant. */
		phase?: string;
	} = $props();

	const indicator = createStickyThinkingIndicator(() => phase);
</script>

<div class="assistant-thinking-wait" aria-live="polite" aria-busy="true">
	<ThinkingIndicator size={24} {label} {color} variant={indicator.variant} />
	<div class="assistant-thinking-copy">
		<p class="assistant-thinking-detail">{detail}</p>
	</div>
</div>

<style>
	.assistant-thinking-wait {
		display: flex;
		align-items: center;
		gap: 10px;
		padding: 8px 2px 2px;
	}

	.assistant-thinking-copy {
		display: flex;
		flex-direction: column;
		gap: 2px;
		min-width: 0;
	}

	.assistant-thinking-detail {
		margin: 0;
		font-size: 12px;
		line-height: 1.45;
		color: var(--text-soft, rgba(0, 0, 0, 0.55));
		background-image: linear-gradient(
			90deg,
			var(--text-soft, rgba(0, 0, 0, 0.45)) 0%,
			var(--text-soft, rgba(0, 0, 0, 0.45)) 40%,
			color-mix(in srgb, var(--text-main, #111) 78%, white) 50%,
			var(--text-soft, rgba(0, 0, 0, 0.45)) 60%,
			var(--text-soft, rgba(0, 0, 0, 0.45)) 100%
		);
		background-size: 220% 100%;
		background-clip: text;
		-webkit-background-clip: text;
		color: transparent;
		-webkit-text-fill-color: transparent;
		animation: thinking-detail-shimmer 3.6s ease-in-out infinite;
	}

	@keyframes thinking-detail-shimmer {
		0% {
			background-position: 100% 0;
		}
		100% {
			background-position: -100% 0;
		}
	}

	@media (prefers-reduced-motion: reduce) {
		.assistant-thinking-detail {
			animation: none;
			background: none;
			color: var(--text-soft, rgba(0, 0, 0, 0.55));
			-webkit-text-fill-color: unset;
		}
	}
</style>
