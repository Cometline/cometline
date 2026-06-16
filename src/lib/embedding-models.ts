import type { ProviderConfig, ProviderMethod } from '$lib/types';

export interface EmbeddingModelOption {
	providerId: string;
	providerName: string;
	method: ProviderMethod;
	model: string;
	baseURL: string;
	apiKey: string;
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

export function resolveEmbeddingSelection(
	providers: ProviderConfig[],
	providerId: string,
	model: string
): EmbeddingModelOption | undefined {
	const key = `${providerId}:${model}`;
	return listEmbeddingModelOptions(providers).find((opt) => embeddingOptionKey(opt) === key);
}
