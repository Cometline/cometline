<script lang="ts">
	import { fly } from 'svelte/transition';

	let {
		name,
		selected = false,
		enabled = false,
		onclick
	}: {
		name: string;
		selected?: boolean;
		enabled?: boolean;
		onclick: () => void;
	} = $props();
</script>

<button
	class="provider-card"
	class:selected
	class:enabled
	{onclick}
	transition:fly={{ y: 4, duration: 100 }}
>
	<span>
		<strong>{name}</strong>
	</span>
	<span class="provider-dot" aria-hidden="true"></span>
</button>

<style>
	.provider-card {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 8px;
		width: 100%;
		padding: 10px 11px;
		border: 1px solid var(--border-soft);
		border-radius: 11px;
		background: rgba(255, 255, 255, 0.55);
		text-align: left;
		cursor: pointer;
	}

	.provider-card.selected {
		border-color: rgba(0, 102, 204, 0.35);
		background: rgba(0, 102, 204, 0.06);
	}

	.provider-card strong {
		font-size: 12px;
		color: var(--text-main);
	}

	.provider-dot {
		width: 8px;
		height: 8px;
		border-radius: 999px;
		background: rgba(148, 163, 184, 0.8);
	}

	.provider-card.enabled .provider-dot {
		background: #22c55e;
	}
</style>
