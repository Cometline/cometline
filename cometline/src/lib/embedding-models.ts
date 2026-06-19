import type { ProviderConfig, ProviderMethod } from '$lib/types';

export interface EmbeddingModelOption {
	providerId: string;
	providerName: string;
	method: ProviderMethod;
	model: string;
	baseURL: string;
	apiKey: string;
	/** Saved selection that is no longer enabled under Providers. */
	orphan?: boolean;
}

export interface SavedEmbeddingRef {
	providerId: string;
	provider: string;
	model: string;
	baseURL: string;
	apiKey: string;
}

export interface MemoryEmbeddingFields {
	provider_id: string;
	provider: string;
	model: string;
	base_url: string;
	api_key?: string;
}

const EMBEDDING_MODEL_RE = /embed/i;

export function providerSupportsEmbeddings(method: ProviderMethod): boolean {
	return method === 'openai' || method === 'openai-compatible' || method === 'opencode-go';
}

export function isEmbeddingModelName(model: string): boolean {
	return EMBEDDING_MODEL_RE.test(model.trim());
}

export function embeddingProviderForMethod(method: ProviderMethod): string {
	switch (method) {
		case 'openai':
			return 'openai';
		case 'openai-compatible':
		case 'opencode-go':
			return 'openai-compatible';
		default:
			return '';
	}
}

export function embeddingOptionKey(option: EmbeddingModelOption): string {
	return `${option.providerId}:${option.model}`;
}

function trim(value: string | undefined): string {
	return String(value ?? '').trim();
}

export function savedEmbeddingFromLocal(local: SavedEmbeddingRef | undefined): SavedEmbeddingRef {
	return {
		providerId: trim(local?.providerId),
		provider: trim(local?.provider),
		model: trim(local?.model),
		baseURL: trim(local?.baseURL),
		apiKey: trim(local?.apiKey)
	};
}

export function savedEmbeddingFromApi(api: MemoryEmbeddingFields | undefined): SavedEmbeddingRef {
	return {
		providerId: trim(api?.provider_id),
		provider: trim(api?.provider),
		model: trim(api?.model),
		baseURL: trim(api?.base_url),
		apiKey: trim(api?.api_key)
	};
}

/** Merge API embedding with local saved embedding; local fills missing API fields. */
export function mergeEmbeddingFields(
	api: MemoryEmbeddingFields | undefined,
	local: SavedEmbeddingRef | undefined
): MemoryEmbeddingFields {
	const fromApi = savedEmbeddingFromApi(api);
	const fromLocal = savedEmbeddingFromLocal(local);
	return {
		provider_id: fromApi.providerId || fromLocal.providerId,
		provider: fromApi.provider || fromLocal.provider,
		model: fromApi.model || fromLocal.model,
		base_url: fromApi.baseURL || fromLocal.baseURL,
		api_key: fromApi.apiKey || fromLocal.apiKey
	};
}

export function hasSavedEmbedding(ref: SavedEmbeddingRef | MemoryEmbeddingFields): boolean {
	if ('provider_id' in ref) {
		return trim(ref.model) !== '';
	}
	return trim(ref.model) !== '';
}

/** Lists embedding models enabled in the Providers settings. Empty when none qualify. */
export function listEmbeddingModelOptions(providers: ProviderConfig[]): EmbeddingModelOption[] {
	const out: EmbeddingModelOption[] = [];
	for (const provider of providers) {
		if (!provider.enabled || !providerSupportsEmbeddings(provider.method)) {
			continue;
		}
		for (const model of provider.enabledModels) {
			if (!isEmbeddingModelName(model)) {
				continue;
			}
			out.push({
				providerId: provider.id,
				providerName: provider.name,
				method: provider.method,
				model,
				baseURL: provider.baseURL,
				apiKey: provider.apiKey
			});
		}
	}
	return out;
}

function findProviderForSaved(
	providers: ProviderConfig[],
	saved: SavedEmbeddingRef
): ProviderConfig | undefined {
	if (saved.providerId) {
		const byId = providers.find((p) => p.id === saved.providerId);
		if (byId) return byId;
	}
	if (saved.provider) {
		const byMethod = providers.find(
			(p) => p.id === saved.provider || p.method === saved.provider
		);
		if (byMethod) return byMethod;
	}
	if (saved.model) {
		return providers.find(
			(p) => p.enabledModels.includes(saved.model) || p.models.includes(saved.model)
		);
	}
	return undefined;
}

function orphanOptionFromSaved(
	providers: ProviderConfig[],
	saved: SavedEmbeddingRef
): EmbeddingModelOption | undefined {
	if (!trim(saved.model)) return undefined;
	const provider = findProviderForSaved(providers, saved);
	const providerId = saved.providerId || provider?.id || '__saved__';
	const providerName = provider?.name || saved.providerId || saved.provider || 'Saved provider';
	const method = provider?.method ?? 'openai-compatible';
	return {
		providerId,
		providerName,
		method,
		model: saved.model,
		baseURL: saved.baseURL || provider?.baseURL || '',
		apiKey: saved.apiKey || provider?.apiKey || '',
		orphan: true
	};
}

export function resolveEmbeddingSelection(
	providers: ProviderConfig[],
	providerId: string,
	model: string,
	saved?: SavedEmbeddingRef
): EmbeddingModelOption | undefined {
	const trimmedModel = trim(model);
	const trimmedProviderId = trim(providerId);
	const options = listEmbeddingModelOptions(providers);

	if (trimmedProviderId && trimmedModel) {
		const exact = options.find(
			(opt) => opt.providerId === trimmedProviderId && opt.model === trimmedModel
		);
		if (exact) return exact;
	}

	if (trimmedModel) {
		const byModel = options.filter((opt) => opt.model === trimmedModel);
		if (byModel.length === 1) return byModel[0];
		if (trimmedProviderId) {
			const byProviderAndModel = options.filter((opt) => opt.providerId === trimmedProviderId);
			if (byProviderAndModel.length === 1) return byProviderAndModel[0];
		}
	}

	const savedRef = savedEmbeddingFromLocal(saved);
	if (!trim(savedRef.model)) {
		savedRef.providerId = savedRef.providerId || trimmedProviderId;
		savedRef.model = trimmedModel;
	}
	if (!trim(savedRef.model)) return undefined;

	const savedMatch = options.find(
		(opt) =>
			opt.providerId === savedRef.providerId && opt.model === savedRef.model
	);
	if (savedMatch) return savedMatch;

	const orphan = orphanOptionFromSaved(providers, savedRef);
	if (orphan?.model) {
		const alreadyListed = options.some(
			(opt) => embeddingOptionKey(opt) === embeddingOptionKey(orphan)
		);
		if (!alreadyListed) return orphan;
	}

	return undefined;
}

function persistedEmbeddingRef(
	saved?: SavedEmbeddingRef,
	current?: MemoryEmbeddingFields
): SavedEmbeddingRef {
	return savedEmbeddingFromApi(mergeEmbeddingFields(current, saved));
}

/** Dropdown options: enabled models plus an orphan entry for a persisted but disabled selection. */
export function buildEmbeddingDropdownOptions(
	providers: ProviderConfig[],
	saved?: SavedEmbeddingRef,
	current?: MemoryEmbeddingFields
): EmbeddingModelOption[] {
	const options = listEmbeddingModelOptions(providers);
	const savedRef = persistedEmbeddingRef(saved, current);
	if (!trim(savedRef.model)) return options;

	const resolved = resolveEmbeddingSelection(
		providers,
		savedRef.providerId,
		savedRef.model,
		savedRef
	);
	if (!resolved) return options;
	if (!resolved.orphan) return options;
	if (options.some((opt) => embeddingOptionKey(opt) === embeddingOptionKey(resolved))) {
		return options;
	}
	return [...options, resolved];
}

export function embeddingKeyForFields(
	providers: ProviderConfig[],
	embedding: MemoryEmbeddingFields,
	saved?: SavedEmbeddingRef
): string {
	const persisted = persistedEmbeddingRef(saved, embedding);
	const match = resolveEmbeddingSelection(
		providers,
		persisted.providerId,
		persisted.model,
		persisted
	);
	return match ? embeddingOptionKey(match) : '';
}
