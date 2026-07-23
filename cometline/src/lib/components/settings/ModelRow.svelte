<script lang="ts">
	import { fly } from 'svelte/transition';
	import { Check } from '@lucide/svelte';
	import { formatContextWindow } from '$lib/context-window';
	import ModelCapabilityIcons from '$lib/components/model/ModelCapabilityIcons.svelte';

	let {
		model,
		providerId,
		enabled = false,
		context = null,
		inputModalities = null,
		modalitiesKnown = null,
		onclick
	}: {
		model: string;
		providerId: string;
		enabled?: boolean;
		context?: number | null;
		inputModalities?: readonly string[] | null;
		modalitiesKnown?: boolean | null;
		onclick: () => void;
	} = $props();
</script>

<button class="model-row" class:enabled {onclick} transition:fly={{ y: 4, duration: 100 }}>
	<span class="model-row-copy">
		<span class="model-row-title">
			<strong>{model}</strong>
			<ModelCapabilityIcons modalities={inputModalities} known={modalitiesKnown} />
		</span>
		<small>
			{providerId}:{model}{#if context && context > 0}
				 · {formatContextWindow(context)} ctx{/if}
		</small>
	</span>
	<span class="model-toggle" aria-hidden="true">
		{#if enabled}
			<Check size={13} />
		{/if}
	</span>
</button>

<style>
	.model-row {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 10px;
		width: 100%;
		padding: 9px 10px;
		border: 1px solid var(--border-soft);
		border-radius: 10px;
		background: rgba(255, 255, 255, 0.55);
		text-align: left;
		cursor: pointer;
	}

	.model-row.enabled {
		border-color: rgba(0, 102, 204, 0.28);
		background: rgba(0, 102, 204, 0.05);
	}

	.model-row-copy {
		min-width: 0;
	}

	.model-row-title {
		display: flex;
		align-items: center;
		gap: 5px;
		min-width: 0;
	}

	.model-row strong {
		display: block;
		font-size: 12px;
		color: var(--text-main);
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.model-row small {
		display: block;
		margin-top: 2px;
		font-size: 10px;
		color: var(--text-soft);
	}

	.model-toggle {
		display: grid;
		place-items: center;
		width: 18px;
		height: 18px;
		color: var(--accent);
	}
</style>
