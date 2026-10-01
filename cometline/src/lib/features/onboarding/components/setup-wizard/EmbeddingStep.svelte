<script lang="ts">
	import { LoaderCircle, RefreshCw } from '@lucide/svelte';
	import SettingsButton from '$lib/features/settings/components/SettingsButton.svelte';
	import { embeddingOptionKey } from '$lib/embedding-models';
	import { openOllamaDownloadPage } from '$lib/ollama/client';
	import { PRIVATE_MEMORY, PRIVATE_MEMORY_KEY } from '$lib/features/onboarding/setup-wizard';
	import type { SetupWizardMemory } from '$lib/features/onboarding/setup-wizard-memory.svelte';
	import StepIntro from './StepIntro.svelte';
	import WizardField from './WizardField.svelte';

	let { memory }: { memory: SetupWizardMemory } = $props();
</script>

<StepIntro>
	Most people skip cloud API keys at first. We recommend local
	<strong>Private Memory</strong>
	({PRIVATE_MEMORY.pullName}, {PRIVATE_MEMORY.sizeLabel}) via Ollama — no API key required. You
	can still skip or pick a hosted embedding model.
</StepIntro>
<div class="recommend-card">
	<div>
		<strong>Recommended: Private Memory</strong>
		<p>Local embeddings on your Mac. Install/start Ollama if needed, then pull this model.</p>
		<div class="ollama-check-status" aria-live="polite">
			{#if memory.checkingOllama}
				<p>Checking for Ollama…</p>
			{:else if memory.ollamaHealth}
				<p class="codex-status" class:ok={memory.ollamaHealth.ok}>
					{memory.ollamaHealth.ok
						? 'Ollama is ready.'
						: 'Ollama is not installed or not running.'}
				</p>
			{/if}
		</div>
		{#if memory.ollamaWizardError}
			<p class="wizard-error">{memory.ollamaWizardError}</p>
		{/if}
	</div>
	<div class="inline-actions">
		{#if !memory.ollamaHealth?.ok}
			<SettingsButton variant="secondary" onclick={() => void openOllamaDownloadPage()}>
				Install Ollama
			</SettingsButton>
			<SettingsButton
				variant="secondary"
				onclick={() => void memory.refreshOllamaHealth()}
				disabled={memory.checkingOllama}
			>
				{#if memory.checkingOllama}<LoaderCircle size={14} class="spin" />{:else}<RefreshCw
						size={14}
					/>{/if}
				{memory.checkingOllama ? 'Checking…' : 'Check Ollama'}
			</SettingsButton>
		{/if}
		{#if memory.ollamaHealth?.ok}
			<SettingsButton
				variant="primary"
				onclick={() => void memory.recommendPrivateMemory()}
				disabled={memory.pullingPrivateMemory}
			>
				{#if memory.pullingPrivateMemory}<LoaderCircle size={14} class="spin" />{/if}
				{memory.selectedEmbeddingKey === PRIVATE_MEMORY_KEY
					? 'Selected'
					: 'Use Private Memory'}
			</SettingsButton>
		{/if}
	</div>
</div>
{#if memory.memoryLoading}
	<p class="embedding-loading">
		<LoaderCircle size={14} class="spin" /> Loading memory settings…
	</p>
{:else if memory.memoryError}
	<p class="wizard-error">{memory.memoryError}</p>
	<StepIntro>You can skip this step and configure memory later in Settings.</StepIntro>
{:else}
	<WizardField label="Or choose another embedding model">
		<select
			class="field-input"
			value={memory.selectedEmbeddingKey}
			onchange={(e) => memory.selectEmbedding(e.currentTarget.value)}
		>
			<option value="">— Skip (configure later) —</option>
			{#if memory.ollamaHealth?.ok}
				<option value={PRIVATE_MEMORY_KEY}
					>Ollama Local · {PRIVATE_MEMORY.pullName} (recommended)</option
				>
			{/if}
			{#each memory.availableEmbeddingOptions as opt (embeddingOptionKey(opt))}
				<option value={embeddingOptionKey(opt)}>{opt.providerName} · {opt.model}</option>
			{/each}
		</select>
	</WizardField>
{/if}

<style>
	.field-input {
		width: 100%;
		padding: 9px 11px;
		border: 1px solid var(--border-soft);
		border-radius: 8px;
		background: var(--panel-bg, var(--panel-bg));
		color: var(--text-main);
		font: inherit;
		font-size: 13px;
	}

	.field-input:focus {
		outline: none;
		border-color: var(--accent);
		box-shadow: 0 0 0 2px rgba(0, 102, 204, 0.12);
	}

	.wizard-error {
		margin: 12px 0 0;
		font-size: 12px;
		color: var(--status-error);
	}

	.codex-status {
		margin: 0 0 14px;
		font-size: 13px;
		color: var(--status-error);
	}

	.codex-status.ok {
		color: var(--status-success);
	}

	.ollama-check-status p {
		margin: 0 0 10px;
		font-size: 12px;
		color: var(--text-muted);
	}

	.recommend-card {
		display: grid;
		gap: 8px;
		padding: 12px 14px;
		border: 1px solid var(--border-soft);
		border-radius: 11px;
		background: rgba(255, 255, 255, 0.55);
		margin-bottom: 14px;
	}

	.recommend-card strong {
		font-size: 13px;
		font-weight: 650;
		color: var(--text-main);
	}

	.recommend-card p {
		margin: 0;
		font-size: 12px;
		color: var(--text-muted);
		line-height: 1.45;
	}

	.inline-actions {
		display: flex;
		gap: 10px;
		flex-wrap: wrap;
	}

	.embedding-loading {
		display: flex;
		align-items: center;
		gap: 8px;
		font-size: 13px;
		color: var(--text-muted);
	}
</style>
