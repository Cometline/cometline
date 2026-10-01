<script lang="ts">
	import { fly } from 'svelte/transition';
	import type { ProviderMethod } from '$lib/types';
	import ProviderLogo from './ProviderLogo.svelte';

	let {
		name,
		method,
		selected = false,
		enabled = false,
		onclick
	}: {
		name: string;
		method: ProviderMethod;
		selected?: boolean;
		enabled?: boolean;
		onclick: () => void;
	} = $props();

	let displayName = $derived(name.trim().toLowerCase() === 'openai' ? 'OpenAI' : name);
</script>

<button
	class="provider-card"
	class:selected
	class:enabled
	{onclick}
	transition:fly={{ y: 4, duration: 100 }}
>
	<span class="provider-identity">
		<ProviderLogo {method} />
		<strong>{displayName}</strong>
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

	.provider-identity {
		display: flex;
		align-items: center;
		min-width: 0;
		gap: 8px;
	}

	.provider-identity strong {
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
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
