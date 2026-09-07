<script lang="ts">
	import { Volume2, VolumeX } from '@lucide/svelte';
	let { muted = false }: { muted?: boolean } = $props();
</script>

<span class="audio-icon" aria-hidden="true">
	{#if muted}
		<VolumeX size={14} />
	{:else}
		<span class="equalizer"><i></i><i></i><i></i></span>
		<span class="still"><Volume2 size={14} /></span>
	{/if}
</span>

<style>
	.audio-icon {
		display: inline-grid;
		place-items: center;
		width: 14px;
		height: 14px;
		flex-shrink: 0;
	}
	.equalizer {
		display: flex;
		align-items: center;
		justify-content: center;
		gap: 2px;
		height: 12px;
	}
	.equalizer i {
		width: 2px;
		height: 10px;
		border-radius: 1px;
		background: currentColor;
		animation: audio-bars 900ms ease-in-out infinite alternate;
	}
	.equalizer i:nth-child(2) {
		animation-delay: -300ms;
	}
	.equalizer i:nth-child(3) {
		animation-delay: -600ms;
	}
	.still {
		display: none;
	}
	@keyframes audio-bars {
		from {
			transform: scaleY(0.3);
		}
		to {
			transform: scaleY(1);
		}
	}
	@media (prefers-reduced-motion: reduce) {
		.equalizer {
			display: none;
		}
		.equalizer i {
			animation: none;
		}
		.still {
			display: inline-flex;
		}
	}
</style>
