<script lang="ts">
	import type { ProviderConfig } from '#lib/types.js';
	import StepIntro from './StepIntro.svelte';

	let {
		selectedProviders,
		defaultProvider,
		embeddingLabel,
		saveError
	}: {
		selectedProviders: ProviderConfig[];
		defaultProvider: ProviderConfig | undefined;
		embeddingLabel: string;
		saveError: string;
	} = $props();
</script>

<StepIntro>
	Your providers are ready to go. Save your settings and Cometline will connect. Screen capture is
	optional and comes next.
</StepIntro>
<div class="review">
	<div class="review-row">
		<span>Providers</span>
		<strong>{selectedProviders.length} configured</strong>
	</div>
	<div class="review-row">
		<span>Default</span>
		<strong>{defaultProvider?.name || '—'}</strong>
	</div>
	<div class="review-row">
		<span>Models</span>
		<strong
			>{selectedProviders.reduce(
				(total, provider) => total + provider.enabledModels.length,
				0
			)} selected</strong
		>
	</div>
	<div class="review-row">
		<span>Embedding</span>
		<strong>{embeddingLabel}</strong>
	</div>
</div>
{#if saveError}
	<p class="wizard-error">{saveError}</p>
{/if}

<style>
	.review {
		display: grid;
		gap: 10px;
		padding: 16px;
		border: 1px solid var(--border-soft);
		border-radius: 10px;
	}

	.review-row {
		display: flex;
		justify-content: space-between;
		align-items: center;
		font-size: 13px;
	}

	.review-row span {
		color: var(--text-muted);
	}

	.review-row strong {
		color: var(--text-main);
		font-weight: 600;
	}

	.wizard-error {
		margin: 12px 0 0;
		font-size: 12px;
		color: var(--status-error);
	}
</style>
