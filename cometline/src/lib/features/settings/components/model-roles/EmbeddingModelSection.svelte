<script lang="ts">
	import { LoaderCircle } from '@lucide/svelte';
	import { cancelMemoryReembed } from '#lib/client/cometmind.js';
	import { embeddingOptionKey } from '#lib/embedding-models.js';
	import type { SettingsMemoryPanel } from '#lib/features/settings/settings-memory-panel.svelte.js';

	let { panel }: { panel: SettingsMemoryPanel } = $props();
</script>

<div class="settings-section">
	<div class="settings-section-heading">
		<h3>Embedding model</h3>
		<p>The model used to store and search semantic memories.</p>
	</div>
	{#if panel.s.loading}
		<p class="settings-field-hint">Loading memory settings…</p>
	{:else if !panel.s.settings}
		<p class="settings-field-hint">
			{panel.s.loadError || 'Could not load memory settings.'}
		</p>
	{:else}
		<div class="embedding-row">
			<div class="embedding-control-row">
				<label>
					<span>Embedding model</span>
					{#if panel.embeddingDropdownOptions.length === 0}
						<p class="empty-embedding">
							No embedding models enabled. Enable an embedding model under Models
							(Ollama Local recommended for private memory).
						</p>
					{:else}
						<select
							value={panel.resolvedEmbeddingKey}
							onchange={(event) => {
								panel.s.selectedEmbeddingKey = event.currentTarget.value;
							}}
						>
							<option value="">Select embedding model…</option>
							{#each panel.embeddingDropdownOptions as option (embeddingOptionKey(option))}
								<option value={embeddingOptionKey(option)}>
									{option.method === 'ollama'
										? 'Local · '
										: ''}{option.providerName}
									· {option.model}{option.orphan ? ' (enable in Models)' : ''}
								</option>
							{/each}
						</select>
					{/if}
				</label>
				<button
					type="button"
					class="secondary reembed-button"
					onclick={() => void panel.forceReembed()}
					disabled={panel.s.reembedding ||
						panel.s.saving ||
						panel.s.loading ||
						panel.s.fullMemories.length === 0 ||
						panel.s.reembedJob?.status === 'running'}
				>
					{#if panel.s.reembedding}<span class="spin"><LoaderCircle size={14} /></span
						>{/if}
					Re-embed
				</button>
			</div>
			{#if panel.s.reembedStatus || (panel.s.reembedJob?.status && ['pending', 'running'].includes(panel.s.reembedJob.status))}
				<div class="reembed-status">
					<p>
						{panel.s.reembedStatus ||
							`Re-embedding… ${panel.s.reembedJob?.completed ?? 0}/${panel.s.reembedJob?.total ?? 0}`}
					</p>
					{#if panel.s.reembedJob?.status === 'running'}
						<button
							type="button"
							class="secondary"
							onclick={() => void cancelMemoryReembed()}
						>
							Cancel re-embed
						</button>
					{/if}
				</div>
			{/if}
		</div>
	{/if}
</div>

<style>
	.empty-embedding {
		margin: 0;
		padding: 10px 11px;
		border: 1px dashed var(--border-soft);
		border-radius: 11px;
		background: rgba(255, 255, 255, 0.5);
		font-size: 12px;
		font-weight: 500;
		color: var(--text-muted);
	}

	.embedding-row {
		display: grid;
		gap: 12px;
	}

	.embedding-control-row {
		display: flex;
		align-items: flex-end;
		gap: 8px;
	}

	.embedding-control-row label {
		display: grid;
		min-width: 0;
		flex: 1 1 auto;
		gap: 6px;
		font-size: 12px;
		font-weight: 600;
		color: var(--text-muted);
	}

	.reembed-button {
		flex: 0 0 auto;
		height: 38px;
		padding: 5px 9px;
		font-size: 11px;
		line-height: 1.2;
		white-space: nowrap;
	}

	.reembed-status {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: 10px;
	}

	.reembed-status p {
		margin: 0;
		font-size: 12px;
		line-height: 1.45;
		color: var(--text-muted);
	}

	.spin {
		display: inline-flex;
		margin-right: 4px;
		animation: spin 0.8s linear infinite;
	}

	@keyframes spin {
		to {
			transform: rotate(360deg);
		}
	}
</style>
