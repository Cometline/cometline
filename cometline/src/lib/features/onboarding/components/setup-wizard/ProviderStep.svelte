<script lang="ts">
	import { Check } from '@lucide/svelte';
	import type { ProviderConfig } from '#lib/types.js';
	import { providerLabel } from '#lib/features/onboarding/setup-wizard.js';
	import StepIntro from './StepIntro.svelte';

	let {
		providers,
		selectedProviderIds,
		defaultProviderId,
		onToggle,
		onSetDefault
	}: {
		providers: ProviderConfig[];
		selectedProviderIds: string[];
		defaultProviderId: string;
		onToggle: (id: string) => void;
		onSetDefault: (id: string) => void;
	} = $props();
</script>

<StepIntro>
	Choose one or more LLM providers. Your default provider is used for new chats, and you can
	switch between every configured model later.
</StepIntro>
<div class="provider-list">
	{#each providers as provider (provider.id)}
		<div
			class="provider-option"
			class:selected={selectedProviderIds.includes(provider.id)}
			class:is-default={provider.id === defaultProviderId}
		>
			<button
				class="provider-select"
				aria-pressed={selectedProviderIds.includes(provider.id)}
				onclick={() => onToggle(provider.id)}
			>
				<div class="provider-option-copy">
					<strong>{providerLabel(provider)}</strong>
					<span>{provider.baseURL || 'Custom endpoint'}</span>
				</div>
				{#if selectedProviderIds.includes(provider.id)}
					<Check size={16} />
				{/if}
			</button>
			{#if selectedProviderIds.includes(provider.id)}
				<button
					class="default-provider"
					class:active={provider.id === defaultProviderId}
					onclick={() => onSetDefault(provider.id)}
				>
					{provider.id === defaultProviderId ? 'Default' : 'Make default'}
				</button>
			{/if}
		</div>
	{/each}
</div>

<style>
	.provider-list {
		display: grid;
		gap: 8px;
	}

	.provider-option {
		display: flex;
		align-items: center;
		gap: 8px;
		border: 1px solid var(--border-soft);
		border-radius: 10px;
		background: var(--panel-bg, var(--panel-bg));
		transition: border-color 0.15s ease;
	}

	.provider-option:hover {
		border-color: rgba(0, 102, 204, 0.3);
	}

	.provider-option.is-default {
		border-color: var(--accent);
		box-shadow: 0 0 0 2px rgba(0, 102, 204, 0.12);
	}

	.provider-select {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 12px;
		min-width: 0;
		flex: 1;
		padding: 12px 14px;
		border: 0;
		background: transparent;
		color: inherit;
		cursor: pointer;
		text-align: left;
	}

	.provider-option.selected .provider-select {
		color: var(--accent);
	}

	.default-provider {
		flex-shrink: 0;
		margin-right: 10px;
		padding: 5px 8px;
		border: 1px solid var(--border-soft);
		border-radius: 7px;
		background: transparent;
		color: var(--text-muted);
		font: inherit;
		font-size: 10px;
		cursor: pointer;
	}

	.default-provider.active {
		border-color: transparent;
		background: rgba(0, 102, 204, 0.09);
		color: var(--accent);
	}

	.provider-option-copy {
		display: grid;
		gap: 2px;
	}

	.provider-option-copy strong {
		font-size: 13px;
		font-weight: 600;
		color: var(--text-main);
	}

	.provider-option-copy span {
		font-size: 11px;
		color: var(--text-muted);
	}
</style>
