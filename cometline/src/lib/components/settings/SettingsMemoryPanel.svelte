<script lang="ts">
	import { LoaderCircle, Trash2 } from '@lucide/svelte';
	import SettingsToggle from './SettingsToggle.svelte';
	import {
		compactMemory,
		compactMemoryPreview,
		createMemory,
		defaultMemorySettings,
		deleteMemory,
		getMemorySettings,
		listMemories,
		putMemorySettings,
		previewMemoryReembed,
		startMemoryReembed,
		getMemoryReembedJob,
		cancelMemoryReembed,
		searchMemories,
		CometMindApiError,
		type MemoryResource,
		type CompactMemoryPreviewResponse,
		type MemoryCompactionResult,
		type MemoryReembedJob,
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
	let fullMemories = $state<MemoryResource[]>([]);
	let memories = $state<MemoryResource[]>([]);
	let searchQuery = $state('');
	let searching = $state(false);
	let newContent = $state('');
	let memoryStatus = $state('');
	let loading = $state(true);
	let saving = $state(false);
	let compacting = $state(false);
	let previewing = $state(false);
	let compactionPreview = $state<CompactMemoryPreviewResponse | null>(null);
	let compactionResult = $state<MemoryCompactionResult | null>(null);
	let compactionError = $state('');
	let selectedEmbeddingKey = $state('');
	let reembedJob = $state<MemoryReembedJob | null>(null);
	let reembedStatus = $state('');
	let reembedding = $state(false);

	let loadError = $state('');
	let savedSnapshot = $state('');

	function memorySettingsSnapshot(next: MemorySettings): string {
		return JSON.stringify({
			auto_retrieve: next.auto_retrieve,
			auto_extract: next.auto_extract,
			similarity_threshold: next.similarity_threshold,
			max_retrieved: next.max_retrieved,
			lifecycle: next.lifecycle,
			embedding: next.embedding
		});
	}

	function markSavedSnapshot(next: MemorySettings) {
		savedSnapshot = memorySettingsSnapshot(next);
	}

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

	// Pure: derives the would-be MemorySettings from `base` + the current
	// `selectedEmbeddingKey`/`embeddingDropdownOptions` WITHOUT mutating any
	// reactive state. Safe to call from inside a `$derived`/`$effect` (e.g.
	// `isDirty()`). Do NOT add `$state` writes here — see the comment on
	// `isDirty()` for why that breaks the Save button.
	function computeEmbeddingPayload(base: MemorySettings): MemorySettings {
		const option = embeddingDropdownOptions.find(
			(opt) => embeddingOptionKey(opt) === selectedEmbeddingKey
		);
		if (!option) {
			return {
				...base,
				embedding: {
					...base.embedding,
					provider_id: '',
					provider: '',
					model: '',
					base_url: '',
					api_key: ''
				}
			};
		}
		return {
			...base,
			embedding: {
				...base.embedding,
				provider_id: option.providerId,
				provider: embeddingProviderForMethod(option.method),
				model: option.model,
				base_url: option.baseURL,
				api_key: option.apiKey
			}
		};
	}

	// Impure: commits the computed payload to `settings`. Only call at actual
	// save time (never from a `$derived`/`$effect`).
	function applyEmbeddingSelection(): MemorySettings | null {
		if (!settings) return null;
		const next = computeEmbeddingPayload(settings);
		settings = next;
		return next;
	}

	onMount(() => {
		void reload();
	});

	async function reload() {
		loading = true;
		loadError = '';
		memoryStatus = '';
		try {
			const [s, list] = await Promise.all([getMemorySettings(), listMemories()]);
			const mergedEmbedding = mergeEmbeddingFields(s.embedding, savedEmbedding);
			let nextSettings: MemorySettings = { ...s, embedding: mergedEmbedding };
			if (!s.embedding.model.trim() && mergedEmbedding.model.trim()) {
				nextSettings = await putMemorySettings(nextSettings);
			}
			settings = nextSettings;
			selectedEmbeddingKey = embeddingKeyForSettings(nextSettings);
			markSavedSnapshot(nextSettings);
			fullMemories = list.memories ?? [];
			memories = fullMemories;
			searchQuery = '';
		} catch (error) {
			loadError = error instanceof Error ? error.message : 'Failed to load memory settings';
			settings = defaultMemorySettings();
			selectedEmbeddingKey = '';
			markSavedSnapshot(settings);
			fullMemories = [];
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
		markSavedSnapshot(next);
	}

	// POSTMORTEM GUARDRAIL: `isDirty()` is consumed inside a `$derived`
	// (`saveDisabled` in settings-controller.svelte.ts via `getMemoryPanelDirty`).
	// It and every helper it calls MUST be a pure read of reactive state. Never
	// call `buildSavePayload()`/`applyEmbeddingSelection()` here — those assign to
	// the `settings` `$state`, and mutating reactive state during a `$derived`
	// evaluation breaks Svelte 5 dependency tracking, so changing the embedding
	// dropdown no longer re-triggers `saveDisabled` and the "Save changes" button
	// stays disabled. Use the pure `computeEmbeddingPayload()` instead.
	export function isDirty(): boolean {
		if (loading || !settings) return false;
		const candidate = computeEmbeddingPayload(settings);
		return memorySettingsSnapshot(candidate) !== savedSnapshot;
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

	async function pollReembedJob() {
		for (let i = 0; i < 600; i++) {
			const job = await getMemoryReembedJob();
			reembedJob = job;
			if (!job?.status || !['pending', 'running'].includes(job.status)) {
				return job;
			}
			reembedStatus = `Re-embedding memories… ${job.completed ?? 0}/${job.total ?? 0}`;
			await new Promise((r) => setTimeout(r, 1000));
		}
		return reembedJob;
	}

	async function forceReembed() {
		if (!settings || reembedding) return;
		const payload = computeEmbeddingPayload(settings);
		if (!payload.embedding.model.trim()) {
			reembedStatus = 'Select an embedding model first.';
			return;
		}
		if (fullMemories.length === 0) {
			reembedStatus = 'No memories need re-embedding.';
			return;
		}
		if (
			!window.confirm(
				`Re-embed all ${fullMemories.length} memories with “${payload.embedding.model}”?`
			)
		) {
			return;
		}
		reembedding = true;
		reembedStatus = '';
		try {
			reembedJob = await startMemoryReembed(payload.embedding, true);
			const finished = await pollReembedJob();
			if (finished?.status === 'failed') {
				throw new Error(finished.error || 'Re-embed failed');
			}
			if (finished?.status === 'cancelled') {
				throw new Error('Re-embed cancelled');
			}
			reembedStatus = 'Re-embed complete. Retrieval now uses the selected model.';
		} catch (error) {
			reembedStatus = error instanceof Error ? error.message : 'Re-embed failed';
		} finally {
			reembedding = false;
		}
	}

	export async function saveMemorySettings(): Promise<void> {
		saving = true;
		reembedStatus = '';
		try {
			const payload = buildSavePayload();
			try {
				settings = await putMemorySettings(payload);
			} catch (error) {
				const conflict =
					error instanceof CometMindApiError
						? error.status === 409
						: error instanceof Error && /re-embed/i.test(error.message);
				if (!conflict) throw error;

				const preview = await previewMemoryReembed(payload.embedding);
				if (!preview.migration_needed) {
					settings = await putMemorySettings(payload);
				} else {
					const confirmed = window.confirm(
						`Switching embedding models requires re-embedding ${preview.needs_migration} memories.\n\n` +
							`Retrieval keeps using “${preview.current_model || 'the previous model'}” until the new index is ready.\n\n` +
							`Start background re-embed now?`
					);
					if (!confirmed) {
						throw new Error('Embedding change cancelled — previous model kept.');
					}
					reembedJob = await startMemoryReembed(payload.embedding);
					const finished = await pollReembedJob();
					if (finished?.status === 'failed') {
						throw new Error(finished.error || 'Re-embed failed');
					}
					if (finished?.status === 'cancelled') {
						throw new Error('Re-embed cancelled');
					}
					// Job already applied the embedding; refresh settings for other fields.
					const refreshed = await getMemorySettings();
					settings = {
						...payload,
						embedding: refreshed.embedding.model
							? refreshed.embedding
							: payload.embedding
					};
					try {
						settings = await putMemorySettings(settings);
					} catch {
						// Embedding already migrated; ignore a second conflict.
						settings = await getMemorySettings();
					}
					reembedStatus = 'Re-embed complete. Retrieval now uses the new model.';
				}
			}
			const savedFromResponse = savedEmbeddingFromApi(settings.embedding);
			selectedEmbeddingKey =
				embeddingKeyForFields(providers, settings.embedding, savedFromResponse) ||
				selectedEmbeddingKey;
			markSavedSnapshot(settings);
			await onEmbeddingSaved?.(settings.embedding);
		} catch (error) {
			throw error instanceof Error ? error : new Error('Failed to save memory settings');
		} finally {
			saving = false;
		}
	}

	export function syncFields() {
		// Memory settings persist via SettingsPanel Save changes.
	}

	async function applyMemorySearch(query: string) {
		if (!query) {
			memories = fullMemories;
			searching = false;
			return;
		}
		searching = true;
		try {
			const res = await searchMemories(query, 20);
			if (searchQuery.trim() !== query) return;
			memories = res.memories;
		} catch (error) {
			if (searchQuery.trim() !== query) return;
			memoryStatus = error instanceof Error ? error.message : 'Search failed';
		} finally {
			if (searchQuery.trim() === query) {
				searching = false;
			}
		}
	}

	$effect(() => {
		const query = searchQuery.trim();
		if (!query) {
			memories = fullMemories;
			searching = false;
			return;
		}

		searching = true;
		const timer = setTimeout(() => {
			void applyMemorySearch(query);
		}, 300);

		return () => clearTimeout(timer);
	});

	async function addMemory() {
		if (!newContent.trim()) return;
		try {
			const rec = await createMemory({
				content: newContent.trim(),
				kind: 'fact',
				pinned: false
			});
			fullMemories = [rec, ...fullMemories];
			if (!searchQuery.trim()) {
				memories = fullMemories;
			}
			newContent = '';
			memoryStatus = 'Memory added.';
		} catch (error) {
			memoryStatus = error instanceof Error ? error.message : 'Failed to add memory';
		}
	}

	async function removeMemory(id: string) {
		try {
			await deleteMemory(id);
			fullMemories = fullMemories.filter((m) => m.id !== id);
			memories = memories.filter((m) => m.id !== id);
		} catch (error) {
			memoryStatus = error instanceof Error ? error.message : 'Failed to delete memory';
		}
	}

	async function runCompact() {
		compacting = true;
		compactionError = '';
		try {
			const result = await compactMemory();
			await reload();
			compactionResult = result;
		} catch (error) {
			compactionError = error instanceof Error ? error.message : 'Compaction failed';
		} finally {
			compacting = false;
		}
	}

	async function previewCompact() {
		previewing = true;
		compactionError = '';
		try {
			compactionPreview = await compactMemoryPreview();
		} catch (error) {
			compactionError = error instanceof Error ? error.message : 'Preview failed';
		} finally {
			previewing = false;
		}
	}
</script>

{#if loading}
	<p class="muted">Loading memory settings…</p>
{:else if settings}
	<section class="memory-panel settings-panel-frame">
		<div class="settings-panel-body">
			{#if loadError}
				<p class="load-error">
					{loadError}. Showing defaults — reload CometMind (Save in Settings → Providers)
					or run
					<code>make build-cometmind</code> if endpoints are missing.
					<button class="link-button" type="button" onclick={reload}>Retry</button>
				</p>
			{/if}

			<div class="settings-section">
				<div class="settings-section-heading">
					<div>
						<h3>Retrieval & lifecycle</h3>
						<p>Control when memories are retrieved, extracted, and aged out.</p>
					</div>
				</div>

				<div class="settings-grid">
					<div class="toggles">
						<SettingsToggle
							label="Auto retrieve"
							bind:checked={settings.auto_retrieve}
						/>
						<SettingsToggle
							label="Auto summarize"
							bind:checked={settings.auto_extract}
						/>
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
							<input
								type="range"
								min="1"
								max="20"
								bind:value={settings.max_retrieved}
							/>
						</label>
						<label>
							<span>Task outcomes in prompt ({settings.task_outcome_limit})</span>
							<input
								type="range"
								min="1"
								max="10"
								bind:value={settings.task_outcome_limit}
							/>
							<p class="field-hint">
								Recent completed-job outcomes injected separately from semantic
								memory retrieval.
							</p>
						</label>
						<label>
							<span
								>Decay half-life (days): {settings.lifecycle
									.decay_half_life_days}</span
							>
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
						<div class="embedding-control-row">
							<label>
								<span>Embedding model</span>
								{#if embeddingDropdownOptions.length === 0}
									<p class="empty-embedding">
										No embedding models enabled. Enable an embedding model under
										Settings → Providers (Ollama Local recommended for private
										memory).
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
												{option.method === 'ollama' ? 'Local · ' : ''}{option.providerName}
												· {option.model}{option.orphan
													? ' (enable in Providers)'
													: ''}
											</option>
										{/each}
									</select>
								{/if}
							</label>
							<button
								type="button"
								class="secondary reembed-button"
								onclick={() => void forceReembed()}
								disabled={
									reembedding ||
									saving ||
									loading ||
									fullMemories.length === 0 ||
									reembedJob?.status === 'running'
								}
							>
								{#if reembedding}<span class="spin"><LoaderCircle size={14} /></span>{/if}
								Re-embed
							</button>
						</div>
						{#if reembedStatus || (reembedJob?.status && ['pending', 'running'].includes(reembedJob.status))}
							<div class="reembed-status">
								<p>{reembedStatus || `Re-embedding… ${reembedJob?.completed ?? 0}/${reembedJob?.total ?? 0}`}</p>
								{#if reembedJob?.status === 'running'}
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
				</div>
			</div>

			<div class="settings-section">
				<div class="settings-section-heading">
					<div>
						<h3>Compaction</h3>
						<p>Preview or run memory compaction to merge and prune stored memories.</p>
					</div>
					<div class="memory-total" aria-label={`${fullMemories.length} total memories`}>
						<strong>{fullMemories.length}</strong>
						<span>total memories</span>
					</div>
				</div>

				<div class="actions">
					<button
						class="secondary"
						onclick={previewCompact}
						disabled={previewing || compacting}
					>
						{#if previewing}<span class="spin"><LoaderCircle size={14} /></span>{/if}
						Preview compaction
					</button>
					<button class="secondary" onclick={runCompact} disabled={compacting}>
						{#if compacting}<span class="spin"><LoaderCircle size={14} /></span>{/if}
						Run compaction
					</button>
				</div>

				{#if compactionPreview}
					<div class="compaction-feedback" data-testid="compaction-preview">
						<strong>Preview</strong>
						<span>
							{compactionPreview.to_forget.length} to forget · {compactionPreview
								.to_merge.length}
							merge {compactionPreview.to_merge.length === 1 ? 'cluster' : 'clusters'} ·
							{compactionPreview.active} of {compactionPreview.max_memories} active
						</span>
					</div>
				{/if}

				{#if compactionResult}
					<div class="compaction-feedback success" data-testid="compaction-result">
						<strong>Compaction complete</strong>
						<span>
							{compactionResult.before} → {compactionResult.after} memories ·
							{Math.max(0, compactionResult.before - compactionResult.after)} removed
						</span>
					</div>
				{/if}

				{#if compactionError}
					<p class="compaction-error">{compactionError}</p>
				{/if}
			</div>

			<div class="settings-section">
				<div class="settings-section-heading">
					<div>
						<h3>Memories</h3>
						<p>Search, add, or remove individual memories stored for this workspace.</p>
					</div>
				</div>

				<div class="search-row">
					<input
						type="search"
						bind:value={searchQuery}
						placeholder="Search memories…"
						spellcheck="false"
						aria-busy={searching}
					/>
				</div>

				<div class="add-row">
					<div class="add-row-header">
						<span>Add memory</span>
						<button type="button" class="secondary" onclick={addMemory}>Add</button>
					</div>
					<textarea
						bind:value={newContent}
						rows="3"
						placeholder="Something the agent should remember…"
						aria-label="Memory content"
					></textarea>
				</div>

				<div class="memory-list scrollbar-none">
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

				{#if memoryStatus}
					<p class="status">{memoryStatus}</p>
				{/if}
			</div>
		</div>
	</section>
{:else}
	<p class="load-error">
		{loadError || 'Could not load memory settings.'}
		<button class="link-button" type="button" onclick={reload}>Retry</button>
	</p>
{/if}

<style>
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
		color: var(--status-error);
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

	.reembed-status {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: 10px;
		margin-top: 8px;
	}

	.reembed-status p {
		margin: 0;
		font-size: 12px;
		line-height: 1.45;
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
		min-width: 0;
		flex: 1 1 auto;
	}

	.reembed-button {
		flex: 0 0 auto;
		height: 38px;
		padding: 5px 9px;
		font-size: 11px;
		line-height: 1.2;
		white-space: nowrap;
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

	input[type='range'] {
		width: 100%;
	}

	.actions,
	.search-row {
		display: flex;
		gap: 8px;
		flex-wrap: wrap;
	}

	.memory-total {
		display: flex;
		flex: 0 0 auto;
		align-items: baseline;
		gap: 5px;
		color: var(--text-muted);
	}

	.memory-total strong {
		font-size: 18px;
		color: var(--text-main);
	}

	.memory-total span {
		font-size: 11px;
	}

	.compaction-feedback {
		display: grid;
		gap: 3px;
		padding: 10px 12px;
		border: 1px solid var(--border-soft);
		border-radius: 11px;
		background: rgba(251, 251, 250, 0.72);
		font-size: 12px;
		color: var(--text-muted);
	}

	.compaction-feedback strong {
		color: var(--text-main);
	}

	.compaction-feedback.success {
		border-color: color-mix(in srgb, var(--status-success) 25%, var(--border-soft));
	}

	.compaction-error {
		margin: 0;
		font-size: 12px;
		color: var(--status-error);
	}

	.search-row input {
		width: 100%;
		min-width: 0;
	}

	.add-row {
		display: grid;
		gap: 6px;
	}

	.add-row-header {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 12px;
	}

	.add-row-header span {
		font-size: 12px;
		font-weight: 600;
		color: var(--text-muted);
	}

	.add-row textarea {
		width: 100%;
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

	.icon {
		border: none;
		background: transparent;
		color: var(--text-muted);
	}

	.icon.danger:hover {
		color: var(--status-error);
	}

	@media (max-width: 780px) {
		.toggles,
		.sliders {
			grid-template-columns: 1fr;
		}
	}
</style>
