<script lang="ts">
	import type { Snippet } from 'svelte';
	import type { ProviderConfig } from '$lib/types';
	import { providerLabel } from '$lib/features/onboarding/setup-wizard';

	let {
		providers,
		activeProviderId,
		onSelect,
		badge
	}: {
		providers: ProviderConfig[];
		activeProviderId: string;
		onSelect: (id: string) => void;
		badge: Snippet<[ProviderConfig]>;
	} = $props();
</script>

<div class="provider-tabs" aria-label="Selected providers">
	{#each providers as provider (provider.id)}
		<button
			class:active={provider.id === activeProviderId}
			onclick={() => onSelect(provider.id)}
		>
			{providerLabel(provider)}
			{@render badge(provider)}
		</button>
	{/each}
</div>

<style>
	.provider-tabs {
		display: flex;
		gap: 6px;
		margin-bottom: 16px;
		overflow-x: auto;
	}

	.provider-tabs button {
		display: flex;
		align-items: center;
		gap: 5px;
		flex-shrink: 0;
		padding: 6px 9px;
		border: 1px solid var(--border-soft);
		border-radius: 8px;
		background: var(--panel-bg, var(--panel-bg));
		color: var(--text-muted);
		font: inherit;
		font-size: 11px;
		cursor: pointer;
	}

	.provider-tabs button.active {
		border-color: var(--accent);
		color: var(--accent);
	}
</style>
