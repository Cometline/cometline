<script lang="ts">
	import { LoaderCircle, Search, Trash2 } from '@lucide/svelte';
	import SettingsToggle from '$lib/components/SettingsToggle.svelte';
	import {
		compactMemory,
		compactMemoryPreview,
		createMemory,
		defaultMemorySettings,
		deleteMemory,
		getMemorySettings,
		listMemories,
		putMemorySettings,
		searchMemories,
		type MemoryResource,
		type MemorySettings
	} from '$lib/client/cometmind';
	import {
		buildEmbeddingDropdownOptions,
		embeddingKeyForFields,
		embeddingOptionKey,
		embeddingProviderForMethod,
		mergeEmbeddingFields,
		savedEmbeddingFromApi,
		type SavedEmbeddingRef
	} from '$lib/embedding-models';
	import type { ProviderConfig } from '$lib/types';
	import { onMount } from 'svelte';

	interface Props {
		providers?: ProviderConfig[];
		savedEmbedding?: SavedEmbeddingRef;
		onEmbeddingSaved?: (embedding: MemorySettings['embedding']) => void | Promise<void>;
	}

	let { providers = [], savedEmbedding, onEmbeddingSaved }: Props = $props();

	let settings = $state<MemorySettings | null>(null);
	let memories = $state<MemoryResource[]>([]);
	let searchQuery = $state('');
	let newContent = $state('');
	let status = $state('');
	let loading = $state(true);
	let saving = $state(false);
	let compacting = $state(false);
	let selectedEmbeddingKey = $state('');

	let loadError = $state('');

	const persistedEmbedding = $derived(
		settings ? mergeEmbeddingFields(settings.embedding, savedEmbedding) : undefined
	);

	const embeddingDropdownOptions = $derived(
		buildEmbeddingDropdownOptions(providers, savedEmbedding, persistedEmbedding)
	);

	function embeddingKeyForSettings(next: MemorySettings | null) {
		if (!next) return '';
		return embeddingKeyForFields(
			providers,
			mergeEmbeddingFields(next.embedding, savedEmbedding),
			savedEmbedding
		);
	}

	$effect(() => {
		if (!settings) return;
		if (
			selectedEmbeddingKey &&
			embeddingDropdownOptions.some((opt) => embeddingOptionKey(opt) === selectedEmbeddingKey)
		) {
			return;
		}
		selectedEmbeddingKey = embeddingKeyForSettings(settings);
	});

	function applyEmbeddingSelection(): MemorySettings | null {
		if (!settings) return null;
		const option = embeddingDropdownOptions.find(
			(opt) => embeddingOptionKey(opt) === selectedEmbeddingKey
		);
		if (!option) {
			const next = {
				...settings,
				embedding: {
					...settings.embedding,
					provider_id: '',
					provider: '',
					model: '',
					base_url: '',
					api_key: ''
				}
			};
			settings = next;
			return next;
		}
		const next = {
			...settings,
			embedding: {
				...settings.embedding,
				provider_id: option.providerId,
				provider: embeddingProviderForMethod(option.method),
				model: option.model,
				base_url: option.baseURL,
				api_key: option.apiKey
			}
		};
		settings = next;
		return next;
	}

	onMount(() => {
		void reload();
	});

	async function reload() {
		loading = true;
		loadError = '';
		status = '';
		try {
			const [s, list] = await Promise.all([getMemorySettings(), listMemories()]);
			const mergedEmbedding = mergeEmbeddingFields(s.embedding, savedEmbedding);
			let nextSettings: MemorySettings = { ...s, embedding: mergedEmbedding };
			if (!s.embedding.model.trim() && mergedEmbedding.model.trim()) {
				nextSettings = await putMemorySettings(nextSettings);
			}
			settings = nextSettings;
			selectedEmbeddingKey = embeddingKeyForSettings(nextSettings);
			memories = list.memories ?? [];
		} catch (error) {
			loadError = error instanceof Error ? error.message : 'Failed to load memory settings';
			settings = defaultMemorySettings();
			selectedEmbeddingKey = '';
			memories = [];
		} finally {
			loading = false;
		}
	}

	export function isBusy(): boolean {
		return loading || saving;
	}

	export function applySavedMemory(next: MemorySettings) {
		settings = next;
		selectedEmbeddingKey = embeddingKeyForSettings(next);
	}

	export function buildSavePayload(): MemorySettings {
		if (loading) {
			throw new Error('Memory settings are still loading');
		}
		if (!settings) {
			throw new Error('Memory settings are not available');
		}
		const payload = applyEmbeddingSelection();
		if (!payload) {
			throw new Error('Memory settings are not available');
		}
		return payload;
	}

	export async function saveMemorySettings(): Promise<void> {
		saving = true;
		try {
			settings = await putMemorySettings(buildSavePayload());
			const savedFromResponse = savedEmbeddingFromApi(settings.embedding);
			selectedEmbeddingKey =
				embeddingKeyForFields(providers, settings.embedding, savedFromResponse) ||
				selectedEmbeddingKey;
			await onEmbeddingSaved?.(settings.embedding);
		} catch (error) {
			throw error instanceof Error ? error : new Error('Failed to save memory settings');
		} finally {
			saving = false;
		}
	}

	export function syncFields() {
		// Memory settings persist via SettingsPanel footer Save.
	}

	async function addMemory() {
		if (!newContent.trim()) return;
		try {
			const rec = await createMemory({
				content: newContent.trim(),
				kind: 'fact',
				pinned: false
			});
			memories = [rec, ...memories];
			newContent = '';
			status = 'Memory added.';
		} catch (error) {
			status = error instanceof Error ? error.message : 'Failed to add memory';
		}
	}

	async function removeMemory(id: string) {
		try {
			await deleteMemory(id);
			memories = memories.filter((m) => m.id !== id);
		} catch (error) {
			status = error instanceof Error ? error.message : 'Failed to delete memory';
		}
	}

	async function runSearch() {
		if (!searchQuery.trim()) {
			await reload();
			return;
		}
		try {
			const res = await searchMemories(searchQuery.trim(), 20);
			memories = res.memories;
		} catch (error) {
			status = error instanceof Error ? error.message : 'Search failed';
		}
	}

	async function runCompact() {
		compacting = true;
		try {
			await compactMemory();
			await reload();
			status = 'Compaction complete.';
		} catch (error) {
			status = error instanceof Error ? error.message : 'Compaction failed';
		} finally {
			compacting = false;
		}
	}

	async function previewCompact() {
		try {
			const preview = await compactMemoryPreview();
			status = `Preview: ${preview.to_forget.length} to forget, ${preview.to_merge.length} merge clusters (${preview.active}/${preview.max_memories} active).`;
		} catch (error) {
			status = error instanceof Error ? error.message : 'Preview failed';
		}
	}
</script>

{#if loading}
	<p class="muted">Loading memory settings…</p>
{:else if settings}
	<section class="memory-panel">
		{#if loadError}
			<p class="load-error">
				{loadError}. Showing defaults — restart CometMind (Save in Settings → Providers) or
				run
				<code>make build-cometmind</code> if endpoints are missing.
				<button class="link-button" type="button" onclick={reload}>Retry</button>
			</p>
		{/if}

		<div class="settings-grid">
			<div class="toggles">
				<SettingsToggle label="Auto retrieve" bind:checked={settings.auto_retrieve} />
				<SettingsToggle label="Auto summarize" bind:checked={settings.auto_extract} />
			</div>

			<div class="sliders">
				<label>
					<span
						>Similarity threshold ({Math.round(
							settings.similarity_threshold * 100
						)}%)</span
					>
					<input
						type="range"
						min="0"
						max="1"
						step="0.05"
						bind:value={settings.similarity_threshold}
					/>
				</label>
				<label>
					<span>Max retrieved ({settings.max_retrieved})</span>
					<input type="range" min="1" max="20" bind:value={settings.max_retrieved} />
				</label>
				<label>
					<span>Decay half-life (days): {settings.lifecycle.decay_half_life_days}</span>
					<input
						type="range"
						min="7"
						max="90"
						bind:value={settings.lifecycle.decay_half_life_days}
					/>
				</label>
				<label>
					<span>Max memories: {settings.lifecycle.max_memories}</span>
					<input
						type="range"
						min="100"
						max="2000"
						step="50"
						bind:value={settings.lifecycle.max_memories}
					/>
				</label>
			</div>

			<div class="embedding-row">
				<label>
					<span>Embedding model</span>
					{#if embeddingDropdownOptions.length === 0}
						<p class="empty-embedding">
							No embedding models enabled. Enable an embedding model under Settings →
							Providers.
						</p>
					{:else}
						<select
							bind:value={selectedEmbeddingKey}
							onchange={(event) => {
								selectedEmbeddingKey = event.currentTarget.value;
							}}
						>
							<option value="">Select embedding model…</option>
							{#each embeddingDropdownOptions as option (embeddingOptionKey(option))}
								<option value={embeddingOptionKey(option)}>
									{option.providerName} · {option.model}{option.orphan
										? ' (enable in Providers)'
										: ''}
								</option>
							{/each}
						</select>
					{/if}
				</label>
			</div>
		</div>

		<div class="actions">
			<button class="secondary" onclick={previewCompact}>Preview compaction</button>
			<button class="secondary" onclick={runCompact} disabled={compacting}>
				{#if compacting}<span class="spin"><LoaderCircle size={14} /></span>{/if}
				Run compaction
			</button>
		</div>

		<div class="search-row">
			<input bind:value={searchQuery} placeholder="Search memories…" spellcheck="false" />
			<button class="secondary" onclick={runSearch}><Search size={14} /> Search</button>
		</div>

		<label class="add-row">
			<span>Add memory</span>
			<textarea
				bind:value={newContent}
				rows="3"
				placeholder="Something the agent should remember…"
			></textarea>
			<button class="secondary" onclick={addMemory}>Add</button>
		</label>

		<div class="memory-list">
			{#each memories as memory (memory.id)}
				<article class="memory-card">
					<div>
						<strong>{memory.kind}</strong>
						<p>{memory.content}</p>
						<small>
							weight {memory.effective_weight.toFixed(2)} · accessed {memory.access_count}
							times
						</small>
					</div>
					<button
						class="icon danger"
						aria-label="Delete memory"
						onclick={() => removeMemory(memory.id)}
					>
						<Trash2 size={14} />
					</button>
				</article>
			{:else}
				<p class="muted">No memories yet.</p>
			{/each}
		</div>

		{#if status}
			<p class="status">{status}</p>
		{/if}
	</section>
{:else}
	<p class="load-error">
		{loadError || 'Could not load memory settings.'}
		<button class="link-button" type="button" onclick={reload}>Retry</button>
	</p>
{/if}

<style>
	.memory-panel {
		display: grid;
		gap: 14px;
	}

	.muted,
	.status,
	.load-error,
	.empty-embedding {
		font-size: 12px;
		color: var(--text-muted);
	}

	.empty-embedding {
		margin: 0;
		padding: 10px 11px;
		border: 1px dashed var(--border-soft);
		border-radius: 11px;
		background: rgba(255, 255, 255, 0.5);
	}

	.load-error {
		padding: 12px;
		border: 1px solid rgba(180, 35, 24, 0.25);
		border-radius: 12px;
		background: rgba(180, 35, 24, 0.06);
		color: #b42318;
	}

	.link-button {
		border: none;
		background: none;
		padding: 0;
		margin-left: 6px;
		font: inherit;
		font-size: inherit;
		color: var(--accent);
		cursor: pointer;
		text-decoration: underline;
	}

	.settings-grid {
		display: grid;
		gap: 14px;
	}

	.toggles {
		display: grid;
		grid-template-columns: repeat(2, minmax(0, 1fr));
		gap: 12px;
	}

	.sliders {
		display: grid;
		grid-template-columns: repeat(2, minmax(0, 1fr));
		gap: 12px;
	}

	.embedding-row label {
		display: grid;
		gap: 6px;
	}

	label {
		display: grid;
		gap: 6px;
		font-size: 12px;
		font-weight: 600;
		color: var(--text-muted);
	}

	input,
	select,
	textarea {
		width: 100%;
		border: 1px solid var(--border-soft);
		border-radius: 11px;
		background: rgba(255, 255, 255, 0.76);
		padding: 10px 11px;
		font: inherit;
		font-size: 13px;
		color: var(--text-main);
	}

	.actions,
	.search-row {
		display: flex;
		gap: 8px;
		flex-wrap: wrap;
	}

	.search-row input {
		flex: 1;
		min-width: 180px;
	}

	.add-row textarea {
		resize: vertical;
	}

	.memory-list {
		display: grid;
		gap: 8px;
		max-height: 280px;
		overflow: auto;
	}

	.memory-card {
		display: flex;
		justify-content: space-between;
		gap: 12px;
		padding: 12px;
		border: 1px solid var(--border-soft);
		border-radius: 14px;
		background: rgba(251, 251, 250, 0.72);
	}

	.memory-card p {
		margin: 6px 0;
		font-size: 13px;
		color: var(--text-main);
	}

	.memory-card small {
		font-size: 11px;
		color: var(--text-soft);
	}

	.secondary {
		border: none;
		border-radius: 10px;
		padding: 8px 11px;
		font: inherit;
		font-size: 12px;
		font-weight: 600;
		display: inline-flex;
		align-items: center;
		gap: 7px;
		background: rgba(15, 23, 42, 0.04);
		color: var(--text-main);
	}

	.icon {
		border: none;
		background: transparent;
		color: var(--text-muted);
	}

	.icon.danger:hover {
		color: #b42318;
	}

	.spin {
		display: inline-grid;
		animation: spin 0.9s linear infinite;
	}

	@keyframes spin {
		to {
			transform: rotate(360deg);
		}
	}

	@media (max-width: 780px) {
		.toggles,
		.sliders {
			grid-template-columns: 1fr;
		}
	}
</style>
