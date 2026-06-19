<script lang="ts">
	import { connectionState } from '$lib/stores/runtime.svelte';
	import ThinkingIndicator from '$lib/components/ThinkingIndicator.svelte';
</script>

{#if connectionState.status === 'connecting'}
	<div class="overlay">
		<div class="overlay-card">
			<ThinkingIndicator size={24} label="Starting CometMind" />
			<p>Starting CometMind…</p>
		</div>
	</div>
{/if}

<style>
	.overlay {
		position: absolute;
		inset: 0;
		background: rgba(251, 251, 250, 0.86);
		backdrop-filter: blur(4px);
		display: grid;
		place-items: center;
		z-index: 100;
	}

	.overlay-card {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 12px;
		padding: 28px 34px;
		background: var(--panel-bg);
		border: 1px solid var(--border-soft);
		border-radius: var(--radius-card);
		box-shadow: var(--shadow-card);
		font-size: 14px;
		color: var(--text-muted);
	}
</style>
