import { describe, expect, it } from 'vitest';
import { listEmbeddingModelOptions } from './embedding-models';
import type { ProviderConfig } from './types';

const baseProvider = (patch: Partial<ProviderConfig>): ProviderConfig => ({
	id: 'openai',
	name: 'OpenAI',
	method: 'openai',
	enabled: true,
	baseURL: 'https://api.openai.com/v1',
	apiKey: 'sk-test',
	selectedModel: 'gpt-4o',
	models: ['gpt-4o', 'text-embedding-3-small'],
	enabledModels: ['gpt-4o'],
	...patch
});

describe('listEmbeddingModelOptions', () => {
	it('returns empty when no embedding models are enabled', () => {
		expect(listEmbeddingModelOptions([baseProvider({})])).toEqual([]);
	});

	it('lists enabled embedding models from enabled providers', () => {
		const options = listEmbeddingModelOptions([
			baseProvider({ enabledModels: ['gpt-4o', 'text-embedding-3-small'] })
		]);
		expect(options).toHaveLength(1);
		expect(options[0]?.model).toBe('text-embedding-3-small');
	});

	it('skips anthropic providers', () => {
		const options = listEmbeddingModelOptions([
			baseProvider({
				id: 'anthropic',
				method: 'anthropic',
				enabledModels: ['text-embedding-3-small']
			})
		]);
		expect(options).toEqual([]);
	});
});
