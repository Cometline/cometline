import { describe, expect, it } from 'vitest';
import {
	buildEmbeddingDropdownOptions,
	embeddingKeyForFields,
	listEmbeddingModelOptions,
	mergeEmbeddingFields,
	resolveEmbeddingSelection
} from './embedding-models';
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

describe('resolveEmbeddingSelection', () => {
	const providers = [
		baseProvider({ enabledModels: ['gpt-4o', 'text-embedding-3-small'] })
	];

	it('matches by provider id and model', () => {
		const match = resolveEmbeddingSelection(providers, 'openai', 'text-embedding-3-small');
		expect(match?.model).toBe('text-embedding-3-small');
		expect(match?.orphan).toBeUndefined();
	});

	it('matches by model alone when provider id is empty', () => {
		const match = resolveEmbeddingSelection(providers, '', 'text-embedding-3-small');
		expect(match?.providerId).toBe('openai');
		expect(match?.model).toBe('text-embedding-3-small');
	});

	it('returns orphan option when saved model is not enabled', () => {
		const match = resolveEmbeddingSelection(
			[baseProvider({ enabledModels: ['gpt-4o'] })],
			'openai',
			'text-embedding-3-small',
			{
				providerId: 'openai',
				provider: 'openai',
				model: 'text-embedding-3-small',
				baseURL: 'https://api.openai.com/v1',
				apiKey: 'sk-test'
			}
		);
		expect(match?.orphan).toBe(true);
		expect(match?.model).toBe('text-embedding-3-small');
	});

	it('returns orphan when provider id is missing but model is saved', () => {
		const match = resolveEmbeddingSelection(
			[baseProvider({ enabledModels: ['gpt-4o'] })],
			'',
			'text-embedding-3-small',
			{
				providerId: '',
				provider: '',
				model: 'text-embedding-3-small',
				baseURL: '',
				apiKey: ''
			}
		);
		expect(match?.orphan).toBe(true);
		expect(match?.providerId).toBe('openai');
	});
});

describe('mergeEmbeddingFields', () => {
	it('fills missing API fields from local saved embedding', () => {
		const merged = mergeEmbeddingFields(
			{ provider_id: '', provider: '', model: '', base_url: '', api_key: '' },
			{
				providerId: 'openai',
				provider: 'openai',
				model: 'text-embedding-3-small',
				baseURL: 'https://api.openai.com/v1',
				apiKey: 'sk-test'
			}
		);
		expect(merged.provider_id).toBe('openai');
		expect(merged.model).toBe('text-embedding-3-small');
	});

	it('prefers API values when present', () => {
		const merged = mergeEmbeddingFields(
			{
				provider_id: 'custom',
				provider: 'openai-compatible',
				model: 'custom-embed',
				base_url: 'http://localhost/v1',
				api_key: 'api-key'
			},
			{
				providerId: 'openai',
				provider: 'openai',
				model: 'text-embedding-3-small',
				baseURL: 'https://api.openai.com/v1',
				apiKey: 'sk-test'
			}
		);
		expect(merged.provider_id).toBe('custom');
		expect(merged.model).toBe('custom-embed');
	});
});

describe('buildEmbeddingDropdownOptions', () => {
	it('includes orphan saved model when not in enabled models', () => {
		const providers = [baseProvider({ enabledModels: ['gpt-4o'] })];
		const options = buildEmbeddingDropdownOptions(providers, {
			providerId: 'openai',
			provider: 'openai',
			model: 'text-embedding-3-small',
			baseURL: 'https://api.openai.com/v1',
			apiKey: 'sk-test'
		});
		expect(options).toHaveLength(1);
		expect(options[0]?.orphan).toBe(true);
	});

	it('includes orphan from API current fields when local saved is empty', () => {
		const providers = [baseProvider({ enabledModels: ['gpt-4o'] })];
		const options = buildEmbeddingDropdownOptions(
			providers,
			undefined,
			{
				provider_id: 'openai',
				provider: 'openai',
				model: 'text-embedding-3-small',
				base_url: 'https://api.openai.com/v1'
			}
		);
		expect(options).toHaveLength(1);
		expect(options[0]?.orphan).toBe(true);
		expect(options[0]?.model).toBe('text-embedding-3-small');
	});
});

describe('embeddingKeyForFields', () => {
	it('returns key for merged embedding fields', () => {
		const providers = [
			baseProvider({ enabledModels: ['gpt-4o', 'text-embedding-3-small'] })
		];
		const key = embeddingKeyForFields(providers, {
			provider_id: 'openai',
			provider: 'openai',
			model: 'text-embedding-3-small',
			base_url: 'https://api.openai.com/v1'
		});
		expect(key).toBe('openai:text-embedding-3-small');
	});
});
