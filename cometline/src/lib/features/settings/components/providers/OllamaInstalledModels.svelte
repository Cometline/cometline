<script lang="ts">
	import type { ProviderConfig } from '$lib/types';
	import { formatBytes, type OllamaInstalledModel } from '$lib/ollama/client';
	import { modelStore } from '$lib/stores/model.svelte';
	import ModelRow from '../ModelRow.svelte';

	let {
		provider,
		installed,
		onToggleModel
	}: {
		provider: ProviderConfig;
		installed: OllamaInstalledModel[];
		onToggleModel: (model: string) => void;
	} = $props();
</script>

<section class="settings-section installed-section">
	<div class="settings-section-heading">
		<div>
			<h4>Installed models</h4>
			<p>
				Disk usage shown when available. To free space, remove models in Ollama (Cometline
				does not delete shared model files).
			</p>
		</div>
	</div>

	{#if installed.length === 0}
		<p class="settings-field-hint">No models installed yet.</p>
	{:else}
		<div class="settings-scroll-list model-list scrollbar-none">
			{#each installed as model (model.name)}
				{@const limits = modelStore.limitFor(provider.id, model.name)}
				<div class="installed-row">
					<ModelRow
						model={model.name}
						providerId={provider.id}
						enabled={provider.enabledModels.includes(model.name)}
						context={limits?.context}
						inputModalities={limits?.inputModalities}
						modalitiesKnown={limits?.visionKnown}
						onclick={() => onToggleModel(model.name)}
					/>
					{#if model.size}
						<small class="size-meta">{formatBytes(model.size)}</small>
					{/if}
				</div>
			{/each}
		</div>
	{/if}
</section>

<style>
	.installed-section {
		margin-top: 4px;
		padding-top: 16px;
		border-top: 1px solid var(--border-soft);
	}

	.size-meta {
		font-size: 11px;
		color: var(--text-muted);
	}

	.model-list {
		display: flex;
		flex-direction: column;
		gap: 6px;
	}

	.installed-row {
		display: grid;
		gap: 2px;
	}

	.installed-row .size-meta {
		padding-left: 2px;
	}
</style>
