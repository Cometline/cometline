import { onMount } from 'svelte';
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

export function createSettingsMemoryPanel(deps: {
	getProviders: () => ProviderConfig[];
	getSavedEmbedding: () => SavedEmbeddingRef | undefined;
	onEmbeddingSaved?: (embedding: MemorySettings['embedding']) => void | Promise<void>;
}) {
	const s = $state({
		settings: null as MemorySettings | null,
		fullMemories: [] as MemoryResource[],
		memories: [] as MemoryResource[],
		searchQuery: '',
		searching: false,
		newContent: '',
		newKind: 'fact',
		newApplicationPolicy: 'relevant' as 'always' | 'relevant',
		newRetentionPolicy: 'decaying' as 'protected' | 'decaying',
		memoryStatus: '',
		loading: true,
		saving: false,
		compacting: false,
		previewing: false,
		compactionPreview: null as CompactMemoryPreviewResponse | null,
		compactionResult: null as MemoryCompactionResult | null,
		compactionError: '',
		selectedEmbeddingKey: '',
		reembedJob: null as MemoryReembedJob | null,
		reembedStatus: '',
		reembedding: false,
		loadError: '',
		savedSnapshot: ''
	});
	function memorySettingsSnapshot(next: MemorySettings): string {
		return JSON.stringify({
			auto_retrieve: next.auto_retrieve,
			auto_extract: next.auto_extract,
			similarity_threshold: next.similarity_threshold,
			max_retrieved: next.max_retrieved,
			task_outcome_limit: next.task_outcome_limit,
			lifecycle: next.lifecycle,
			embedding: next.embedding
		});
	}

	function markSavedSnapshot(next: MemorySettings) {
		s.savedSnapshot = memorySettingsSnapshot(next);
	}

	const persistedEmbedding = $derived(
		s.settings
			? mergeEmbeddingFields(s.settings.embedding, deps.getSavedEmbedding())
			: undefined
	);
	const retentionLocked = $derived(
		s.newKind === 'preference' && s.newApplicationPolicy === 'always'
	);

	const embeddingDropdownOptions = $derived(
		buildEmbeddingDropdownOptions(
			deps.getProviders(),
			deps.getSavedEmbedding(),
			persistedEmbedding
		)
	);

	function embeddingKeyForSettings(next: MemorySettings | null) {
		if (!next) return '';
		return embeddingKeyForFields(
			deps.getProviders(),
			mergeEmbeddingFields(next.embedding, deps.getSavedEmbedding()),
			deps.getSavedEmbedding()
		);
	}

	const resolvedEmbeddingKey = $derived.by(() => {
		if (
			s.selectedEmbeddingKey &&
			embeddingDropdownOptions.some(
				(opt) => embeddingOptionKey(opt) === s.selectedEmbeddingKey
			)
		) {
			return s.selectedEmbeddingKey;
		}
		return embeddingKeyForSettings(s.settings);
	});

	// Pure: derives the would-be MemorySettings from `base` + the current
	// `s.selectedEmbeddingKey`/`embeddingDropdownOptions` WITHOUT mutating any
	// reactive state. Safe to call from inside a `$derived`/`$effect` (e.g.
	// `isDirty()`). Do NOT add `$state` writes here — see the comment on
	// `isDirty()` for why that breaks the Save button.
	function computeEmbeddingPayload(base: MemorySettings): MemorySettings {
		const option = embeddingDropdownOptions.find(
			(opt) => embeddingOptionKey(opt) === resolvedEmbeddingKey
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

	// Impure: commits the computed payload to `s.settings`. Only call at actual
	// save time (never from a `$derived`/`$effect`).
	function applyEmbeddingSelection(): MemorySettings | null {
		if (!s.settings) return null;
		const next = computeEmbeddingPayload(s.settings);
		s.settings = next;
		return next;
	}

	onMount(() => {
		void reload();
	});

	async function reload() {
		s.loading = true;
		s.loadError = '';
		s.memoryStatus = '';
		try {
			const [loaded, list] = await Promise.all([getMemorySettings(), listMemories()]);
			const mergedEmbedding = mergeEmbeddingFields(
				loaded.embedding,
				deps.getSavedEmbedding()
			);
			let nextSettings: MemorySettings = { ...loaded, embedding: mergedEmbedding };
			if (!loaded.embedding.model.trim() && mergedEmbedding.model.trim()) {
				nextSettings = await putMemorySettings(nextSettings);
			}
			s.settings = nextSettings;
			s.selectedEmbeddingKey = embeddingKeyForSettings(nextSettings);
			markSavedSnapshot(nextSettings);
			s.fullMemories = list.memories ?? [];
			s.memories = s.fullMemories;
			s.searchQuery = '';
		} catch (error) {
			s.loadError = error instanceof Error ? error.message : 'Failed to load memory settings';
			s.settings = defaultMemorySettings();
			s.selectedEmbeddingKey = '';
			markSavedSnapshot(s.settings);
			s.fullMemories = [];
			s.memories = [];
		} finally {
			s.loading = false;
		}
	}

	function isBusy(): boolean {
		return s.loading || s.saving;
	}

	function applySavedMemory(next: MemorySettings) {
		s.settings = next;
		s.selectedEmbeddingKey = embeddingKeyForSettings(next);
		markSavedSnapshot(next);
	}

	// `isDirty()` is read inside a `$derived` (`saveDisabled`). It must stay a pure
	// read: `buildSavePayload()` / `applyEmbeddingSelection()` assign `$state` and
	// break that dependency, so the embedding dropdown no longer enables Save.
	function isDirty(): boolean {
		if (s.loading || !s.settings) return false;
		const candidate = computeEmbeddingPayload(s.settings);
		return memorySettingsSnapshot(candidate) !== s.savedSnapshot;
	}

	function buildSavePayload(): MemorySettings {
		if (s.loading) {
			throw new Error('Memory settings are still loading');
		}
		if (!s.settings) {
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
			s.reembedJob = job;
			if (!job?.status || !['pending', 'running'].includes(job.status)) {
				return job;
			}
			s.reembedStatus = `Re-embedding memories… ${job.completed ?? 0}/${job.total ?? 0}`;
			await new Promise((r) => setTimeout(r, 1000));
		}
		return s.reembedJob;
	}

	async function forceReembed() {
		if (!s.settings || s.reembedding) return;
		const payload = computeEmbeddingPayload(s.settings);
		if (!payload.embedding.model.trim()) {
			s.reembedStatus = 'Select an embedding model first.';
			return;
		}
		if (s.fullMemories.length === 0) {
			s.reembedStatus = 'No memories need re-embedding.';
			return;
		}
		if (
			!window.confirm(
				`Re-embed all ${s.fullMemories.length} memories with “${payload.embedding.model}”?`
			)
		) {
			return;
		}
		s.reembedding = true;
		s.reembedStatus = '';
		try {
			s.reembedJob = await startMemoryReembed(payload.embedding, true);
			const finished = await pollReembedJob();
			if (finished?.status === 'failed') {
				throw new Error(finished.error || 'Re-embed failed');
			}
			if (finished?.status === 'cancelled') {
				throw new Error('Re-embed cancelled');
			}
			s.reembedStatus = 'Re-embed complete. Retrieval now uses the selected model.';
		} catch (error) {
			s.reembedStatus = error instanceof Error ? error.message : 'Re-embed failed';
		} finally {
			s.reembedding = false;
		}
	}

	async function saveMemorySettings(): Promise<void> {
		s.saving = true;
		s.reembedStatus = '';
		try {
			const payload = buildSavePayload();
			try {
				s.settings = await putMemorySettings(payload);
			} catch (error) {
				const conflict =
					error instanceof CometMindApiError
						? error.status === 409
						: error instanceof Error && /re-embed/i.test(error.message);
				if (!conflict) throw error;

				const preview = await previewMemoryReembed(payload.embedding);
				if (!preview.migration_needed) {
					s.settings = await putMemorySettings(payload);
				} else {
					const confirmed = window.confirm(
						`Switching embedding models requires re-embedding ${preview.needs_migration} memories.\n\n` +
							`Retrieval keeps using “${preview.current_model || 'the previous model'}” until the new index is ready.\n\n` +
							`Start background re-embed now?`
					);
					if (!confirmed) {
						throw new Error('Embedding change cancelled — previous model kept.');
					}
					s.reembedJob = await startMemoryReembed(payload.embedding);
					const finished = await pollReembedJob();
					if (finished?.status === 'failed') {
						throw new Error(finished.error || 'Re-embed failed');
					}
					if (finished?.status === 'cancelled') {
						throw new Error('Re-embed cancelled');
					}
					// Job already applied the embedding; refresh s.settings for other fields.
					const refreshed = await getMemorySettings();
					s.settings = {
						...payload,
						embedding: refreshed.embedding.model
							? refreshed.embedding
							: payload.embedding
					};
					try {
						s.settings = await putMemorySettings(s.settings);
					} catch {
						// Embedding already migrated; ignore a second conflict.
						s.settings = await getMemorySettings();
					}
					s.reembedStatus = 'Re-embed complete. Retrieval now uses the new model.';
				}
			}
			const savedFromResponse = savedEmbeddingFromApi(s.settings.embedding);
			s.selectedEmbeddingKey =
				embeddingKeyForFields(
					deps.getProviders(),
					s.settings.embedding,
					savedFromResponse
				) || s.selectedEmbeddingKey;
			markSavedSnapshot(s.settings);
			await deps.onEmbeddingSaved?.(s.settings.embedding);
		} catch (error) {
			throw error instanceof Error ? error : new Error('Failed to save memory settings');
		} finally {
			s.saving = false;
		}
	}

	function syncFields() {
		// Memory s.settings persist via SettingsPanel Save changes.
	}

	async function applyMemorySearch(query: string) {
		if (!query) {
			s.memories = s.fullMemories;
			s.searching = false;
			return;
		}
		s.searching = true;
		try {
			const res = await searchMemories(query, 20);
			if (s.searchQuery.trim() !== query) return;
			s.memories = res.memories;
		} catch (error) {
			if (s.searchQuery.trim() !== query) return;
			s.memoryStatus = error instanceof Error ? error.message : 'Search failed';
		} finally {
			if (s.searchQuery.trim() === query) {
				s.searching = false;
			}
		}
	}

	const searchActive = $derived(Boolean(s.searchQuery.trim()));
	const visibleMemories = $derived(searchActive ? s.memories : s.fullMemories);
	const visibleSearching = $derived(searchActive && s.searching);

	$effect(() => {
		const query = s.searchQuery.trim();
		if (!query) return;
		s.searching = true;
		const timer = setTimeout(() => {
			void applyMemorySearch(query);
		}, 300);
		return () => clearTimeout(timer);
	});

	async function addMemory() {
		if (!s.newContent.trim()) return;
		try {
			const rec = await createMemory({
				content: s.newContent.trim(),
				kind: s.newKind,
				application_policy:
					s.newKind === 'preference' ? s.newApplicationPolicy : 'relevant',
				retention_policy: s.newRetentionPolicy
			});
			s.fullMemories = [rec, ...s.fullMemories];
			if (!s.searchQuery.trim()) {
				s.memories = s.fullMemories;
			}
			s.newContent = '';
			s.memoryStatus = 'Memory added.';
		} catch (error) {
			s.memoryStatus = error instanceof Error ? error.message : 'Failed to add memory';
		}
	}

	function selectNewKind(kind: string) {
		s.newKind = kind;
		if (kind !== 'preference') {
			s.newApplicationPolicy = 'relevant';
		} else if (s.newApplicationPolicy === 'always') {
			s.newRetentionPolicy = 'protected';
		}
	}

	function selectNewApplicationPolicy(policy: 'always' | 'relevant') {
		s.newApplicationPolicy = policy;
		if (policy === 'always') s.newRetentionPolicy = 'protected';
	}

	async function removeMemory(id: string) {
		try {
			await deleteMemory(id);
			s.fullMemories = s.fullMemories.filter((m) => m.id !== id);
			s.memories = s.memories.filter((m) => m.id !== id);
		} catch (error) {
			s.memoryStatus = error instanceof Error ? error.message : 'Failed to delete memory';
		}
	}

	async function runCompact() {
		s.compacting = true;
		s.compactionError = '';
		try {
			const result = await compactMemory();
			await reload();
			s.compactionResult = result;
		} catch (error) {
			s.compactionError = error instanceof Error ? error.message : 'Compaction failed';
		} finally {
			s.compacting = false;
		}
	}

	async function previewCompact() {
		s.previewing = true;
		s.compactionError = '';
		try {
			s.compactionPreview = await compactMemoryPreview();
		} catch (error) {
			s.compactionError = error instanceof Error ? error.message : 'Preview failed';
		} finally {
			s.previewing = false;
		}
	}

	return {
		s,
		get persistedEmbedding() {
			return persistedEmbedding;
		},
		get retentionLocked() {
			return retentionLocked;
		},
		get embeddingDropdownOptions() {
			return embeddingDropdownOptions;
		},
		get resolvedEmbeddingKey() {
			return resolvedEmbeddingKey;
		},
		get searchActive() {
			return searchActive;
		},
		get visibleMemories() {
			return visibleMemories;
		},
		get visibleSearching() {
			return visibleSearching;
		},
		reload,
		isBusy,
		applySavedMemory,
		isDirty,
		buildSavePayload,
		syncFields,
		addMemory,
		selectNewKind,
		selectNewApplicationPolicy,
		removeMemory,
		runCompact,
		previewCompact,
		forceReembed,
		saveMemorySettings
	};
}

export type SettingsMemoryPanel = ReturnType<typeof createSettingsMemoryPanel>;
