<script lang="ts">
	import { onMount } from 'svelte';
	import { featuredOllamaCatalog } from '$lib/ollama/catalog';
	import { onOllamaPullProgress } from '$lib/ollama/client';
	import type { ProviderConfig } from '$lib/types';
	import { createOllamaPanelController } from '$lib/features/settings/ollama-panel-controller.svelte';
	import SettingsButton from './SettingsButton.svelte';
	import OllamaCatalogItem from './providers/OllamaCatalogItem.svelte';
	import OllamaInstalledModels from './providers/OllamaInstalledModels.svelte';
	import OllamaRuntimeStatus from './providers/OllamaRuntimeStatus.svelte';

	let {
		provider,
		onUpdate
	}: {
		provider: ProviderConfig;
		onUpdate: (patch: Partial<ProviderConfig>) => void;
	} = $props();

	let showAdvanced = $state(false);
	let showGuide = $state(false);
	const featured = featuredOllamaCatalog();
	const panel = createOllamaPanelController({
		getProvider: () => provider,
		onUpdate: (patch) => onUpdate(patch)
	});

	onMount(() => {
		void panel.refresh();
		return onOllamaPullProgress((payload) => panel.handlePullProgress(payload));
	});
</script>

<div class="ollama-panel">
	<OllamaRuntimeStatus
		health={panel.health}
		checking={panel.checking}
		onRefresh={() => void panel.refresh()}
	/>

	{#if !panel.health?.ok}
		<button class="link-button" type="button" onclick={() => (showGuide = !showGuide)}>
			{showGuide ? 'Hide' : 'Show'} setup guide
		</button>
		{#if showGuide}
			<ol class="guide settings-field-hint">
				<li>Open the official Ollama download page and install for macOS.</li>
				<li>Launch Ollama once so the local daemon listens on port 11434.</li>
				<li>Return here and click Check again — no API key is required.</li>
			</ol>
		{/if}
	{/if}

	{#if panel.error}
		<p class="settings-field-hint error">{panel.error}</p>
	{/if}

	{#if panel.health?.ok}
		<section class="settings-section catalog-section">
			<div class="settings-section-heading">
				<div>
					<h4>Recommended models</h4>
					<p>Pull, then enable below and assign roles in Model Roles.</p>
				</div>
			</div>

			<div class="catalog-list">
				{#each featured as entry (entry.id)}
					<OllamaCatalogItem
						{entry}
						installedAlready={panel.installedNames.has(entry.pullName)}
						isPulling={panel.pullingId === entry.id}
						pullProgress={panel.pullingId === entry.id ? panel.pullProgress : null}
						pullDisabled={Boolean(panel.pullingId)}
						onPull={() => void panel.pullEntry(entry)}
						onCancel={() => void panel.cancelPull()}
					/>
				{/each}
			</div>
		</section>

		<button class="link-button" type="button" onclick={() => (showAdvanced = !showAdvanced)}>
			{showAdvanced ? 'Hide' : 'Show'} advanced custom pull
		</button>
		{#if showAdvanced}
			<div class="advanced-row">
				<input
					class="advanced-input"
					value={panel.customModel}
					oninput={(event) => (panel.customModel = event.currentTarget.value)}
					placeholder="model:tag (e.g. llama3.2:3b)"
					spellcheck="false"
					disabled={panel.pullingId.startsWith('custom:')}
				/>
				{#if panel.pullingId.startsWith('custom:')}
					{#if panel.pullProgress?.percent != null}
						<span class="settings-field-hint custom-progress"
							>{panel.pullProgress.percent}%</span
						>
					{/if}
					<SettingsButton variant="secondary" onclick={() => void panel.cancelPull()}
						>Cancel</SettingsButton
					>
				{:else}
					<SettingsButton
						variant="secondary"
						disabled={Boolean(panel.pullingId) || !panel.customModel.trim()}
						onclick={() => void panel.pullCustom()}
					>
						Pull custom
					</SettingsButton>
				{/if}
			</div>
		{/if}

		<OllamaInstalledModels
			{provider}
			installed={panel.installed}
			onToggleModel={panel.toggleModel}
		/>
	{/if}
</div>

<style>
	.ollama-panel {
		display: flex;
		flex-direction: column;
		gap: 12px;
		min-width: 0;
	}

	.link-button {
		appearance: none;
		border: none;
		background: transparent;
		padding: 0;
		margin: 0;
		font: inherit;
		font-size: 12px;
		font-weight: 600;
		color: var(--accent);
		cursor: pointer;
		text-align: left;
	}

	.link-button:hover {
		text-decoration: underline;
	}

	.guide {
		margin: 0;
		padding-left: 1.15rem;
		display: grid;
		gap: 4px;
	}

	.settings-field-hint.error {
		color: var(--status-error);
	}

	.catalog-section {
		margin-top: 4px;
		padding-top: 16px;
		border-top: 1px solid var(--border-soft);
	}

	.catalog-list {
		display: flex;
		flex-direction: column;
		gap: 6px;
	}

	.custom-progress {
		white-space: nowrap;
	}

	.advanced-row {
		display: flex;
		flex-wrap: wrap;
		gap: 8px;
		align-items: center;
	}

	.advanced-input {
		flex: 1;
		min-width: 180px;
	}
</style>
