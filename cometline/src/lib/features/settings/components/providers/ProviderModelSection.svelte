<script lang="ts">
	import { LoaderCircle } from '@lucide/svelte';
	import type { ProviderConfig } from '#lib/types.js';
	import { modelStore } from '#lib/stores/model.svelte.js';
	import { settingsStore } from '#lib/stores/settings.svelte.js';
	import ModelRow from '../ModelRow.svelte';
	import OllamaProviderPanel from '../OllamaProviderPanel.svelte';

	let {
		provider,
		modelSearch = $bindable(''),
		filteredModels,
		xaiAuthenticated = false,
		onUpdate,
		onFetchModels,
		onToggleModel
	}: {
		provider: ProviderConfig;
		modelSearch?: string;
		filteredModels: string[];
		xaiAuthenticated?: boolean;
		onUpdate: (patch: Partial<ProviderConfig>) => void;
		onFetchModels: () => void;
		onToggleModel: (model: string) => void;
	} = $props();

	function canFetchModels(next: ProviderConfig) {
		if (settingsStore.isFetchingModels || !next.baseURL.trim()) return false;
		return (
			next.method === 'codex' ||
			next.method === 'opencode-go' ||
			next.method === 'ollama' ||
			(next.method === 'xai' ? xaiAuthenticated : next.apiKey.trim().length > 0)
		);
	}
</script>

{#if provider.method === 'ollama'}
	<div class="settings-section model-section">
		<div class="settings-section-heading model-heading">
			<div>
				<h3>Ollama Local</h3>
				<p>No API key. Health checks and model pulls stay on loopback only.</p>
			</div>
		</div>
		<OllamaProviderPanel {provider} {onUpdate} />
	</div>
{:else}
	<div class="settings-section model-section">
		<div class="settings-section-heading model-heading">
			<div>
				<h3>Models</h3>
				{#if provider.method === 'codex'}
					<p>Use Fetch models to refresh models from your ChatGPT browser session.</p>
				{:else if provider.method === 'xai'}
					<p>Use Fetch models to refresh the available Grok models from xAI.</p>
				{:else if provider.method === 'opencode-go'}
					<p>
						Use Fetch models to refresh the latest list from <code>/models</code> at OpenCode
						Go.
					</p>
				{:else}
					<p>
						Use Fetch models to refresh the latest list from <code>/models</code>.
					</p>
				{/if}
			</div>
			<button class="secondary" onclick={onFetchModels} disabled={!canFetchModels(provider)}>
				{#if settingsStore.isFetchingModels}<span class="spin"
						><LoaderCircle size={14} /></span
					>{/if}
				Fetch models
			</button>
		</div>

		<input
			class="model-search"
			bind:value={modelSearch}
			placeholder="Search models..."
			spellcheck="false"
		/>

		<div class="settings-scroll-list model-list scrollbar-none">
			{#each filteredModels as model (model)}
				{@const limits = modelStore.limitFor(provider.id, model)}
				<ModelRow
					{model}
					providerId={provider.id}
					enabled={provider.enabledModels.includes(model)}
					context={limits?.context}
					inputModalities={limits?.inputModalities}
					modalitiesKnown={limits?.visionKnown}
					onclick={() => onToggleModel(model)}
				/>
			{:else}
				<p class="empty-models">
					{provider.models.length === 0
						? 'No models loaded yet.'
						: 'No models match your search.'}
				</p>
			{/each}
		</div>
	</div>
{/if}

<style>
	.model-section {
		margin-top: 4px;
		padding-top: 20px;
		border-top: 1px solid var(--border-soft);
	}

	.model-heading {
		margin-bottom: 0;
	}

	.model-heading h3 {
		margin: 0;
		font-size: 14px;
	}

	.model-heading p {
		margin: 4px 0 0;
		font-size: 12px;
		line-height: 1.45;
		color: var(--text-muted);
	}

	.model-heading code {
		font-size: 11px;
	}

	.model-search {
		width: 100%;
		margin-bottom: 10px;
	}

	.empty-models {
		padding: 12px;
		font-size: 12px;
		color: var(--text-muted);
	}
</style>
