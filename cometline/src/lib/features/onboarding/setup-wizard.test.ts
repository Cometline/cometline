import { describe, expect, it } from 'vitest';
import { defaultMemorySettings } from '#lib/client/cometmind.js';
import type { EmbeddingModelOption } from '#lib/embedding-models.js';
import type { ProviderConfig, ProviderSettings } from '#lib/types.js';
import {
	canAdvanceStep,
	completedSetupSettings,
	embeddingReviewLabel,
	embeddingSelectionPayload,
	filterModels,
	initialDefaultProviderId,
	initialSelectedProviderIds,
	PRIVATE_MEMORY,
	PRIVATE_MEMORY_KEY,
	providerLabel,
	toggleEnabledModel
} from './setup-wizard';

function provider(overrides: Partial<ProviderConfig>): ProviderConfig {
	return {
		id: 'openai',
		name: '',
		method: 'openai',
		baseURL: 'https://api.openai.com/v1',
		apiKey: '',
		enabled: false,
		models: [],
		enabledModels: [],
		selectedModel: '',
		...overrides
	} as ProviderConfig;
}

const option: EmbeddingModelOption = {
	providerId: 'openai',
	providerName: 'OpenAI',
	method: 'openai',
	model: 'text-embedding-3-small',
	baseURL: 'https://api.openai.com/v1',
	apiKey: 'sk-test'
} as EmbeddingModelOption;

describe('setup wizard helpers', () => {
	it('labels providers by name, falling back to the method label', () => {
		expect(providerLabel(provider({ name: 'Work' }))).toBe('Work');
		expect(providerLabel(provider({ method: 'xai' }))).toBe('xAI Grok Subscription');
	});

	it('selects enabled providers, else openai, else the first provider', () => {
		expect(
			initialSelectedProviderIds([
				provider({ id: 'a' }),
				provider({ id: 'b', enabled: true })
			])
		).toEqual(['b']);
		expect(
			initialSelectedProviderIds([provider({ id: 'a' }), provider({ id: 'openai' })])
		).toEqual(['openai']);
		expect(initialSelectedProviderIds([provider({ id: 'a' })])).toEqual(['a']);
		expect(initialSelectedProviderIds([])).toEqual([]);
	});

	it('keeps the saved default only when it is selected', () => {
		expect(initialDefaultProviderId('b', ['a', 'b'])).toBe('b');
		expect(initialDefaultProviderId('c', ['a', 'b'])).toBe('a');
		expect(initialDefaultProviderId('c', [])).toBe('');
	});

	it('filters models case-insensitively', () => {
		expect(filterModels(['GPT-5', 'claude'], ' gpt ')).toEqual(['GPT-5']);
		expect(filterModels(['GPT-5', 'claude'], '')).toEqual(['GPT-5', 'claude']);
	});

	it('toggles enabled models and keeps the selected model valid', () => {
		const p = provider({ enabledModels: ['a', 'b'], selectedModel: 'a' });
		expect(toggleEnabledModel(p, 'a')).toEqual({ enabledModels: ['b'], selectedModel: 'b' });
		expect(toggleEnabledModel(p, 'c')).toEqual({
			enabledModels: ['a', 'b', 'c'],
			selectedModel: 'a'
		});
	});

	it('gates advancing on provider, connection, and model steps', () => {
		const connected = provider({ apiKey: 'k', enabledModels: ['m'] });
		const isConnected = (p: ProviderConfig) => p.apiKey.length > 0;
		expect(canAdvanceStep('provider', [], [], isConnected)).toBe(false);
		expect(canAdvanceStep('apikey', ['openai'], [connected], isConnected)).toBe(true);
		expect(canAdvanceStep('apikey', ['openai'], [provider({})], isConnected)).toBe(false);
		expect(canAdvanceStep('model', ['openai'], [provider({})], isConnected)).toBe(false);
		expect(canAdvanceStep('embedding', [], [], isConnected)).toBe(true);
	});

	it('builds the memory payload for the selected embedding', () => {
		const memory = defaultMemorySettings();
		expect(embeddingSelectionPayload(null, '', [], '')).toBeNull();
		expect(embeddingSelectionPayload(memory, '', [], '')?.embedding.model).toBe('');
		expect(
			embeddingSelectionPayload(memory, PRIVATE_MEMORY_KEY, [], 'http://ollama')?.embedding
		).toMatchObject({
			provider: 'ollama',
			model: PRIVATE_MEMORY.pullName,
			base_url: 'http://ollama'
		});
		expect(
			embeddingSelectionPayload(memory, 'openai:text-embedding-3-small', [option], '')
				?.embedding
		).toMatchObject({
			provider_id: 'openai',
			model: 'text-embedding-3-small',
			api_key: 'sk-test'
		});
		expect(embeddingSelectionPayload(memory, 'missing:model', [option], '')).toBe(memory);
	});

	it('describes the embedding choice for the review step', () => {
		expect(embeddingReviewLabel(PRIVATE_MEMORY_KEY, [])).toBe(
			`Local · ${PRIVATE_MEMORY.pullName}`
		);
		expect(embeddingReviewLabel('', [])).toBe('Skipped');
		expect(embeddingReviewLabel('openai:text-embedding-3-small', [option])).toBe(
			'text-embedding-3-small'
		);
		expect(embeddingReviewLabel('missing:model', [option])).toBe('—');
	});

	it('enables exactly the selected providers in the completed settings', () => {
		const draft = {
			providers: [provider({ id: 'a', enabled: true }), provider({ id: 'b' })],
			defaultProviderId: 'a',
			defaultModelId: ''
		} as unknown as ProviderSettings;
		const defaultProvider = provider({ id: 'b', enabledModels: ['m1', 'm2'] });
		const result = completedSetupSettings(draft, ['b'], defaultProvider);
		expect(result.providers.map((p) => p.enabled)).toEqual([false, true]);
		expect(result.defaultProviderId).toBe('b');
		expect(result.defaultModelId).toBe('m1');
	});
});
