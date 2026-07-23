import { describe, expect, it } from 'vitest';
import { modelStore, type ModelLimitEntry } from './model.svelte';

describe('modelStore.applyLimits', () => {
	it('merges catalog limits including modalities for disabled models via limitFor', () => {
		modelStore.setProviders(
			[
				{
					id: 'openai',
					name: 'OpenAI',
					method: 'openai',
					enabled: true,
					baseURL: '',
					apiKey: '',
					models: ['o3-mini', 'gpt-4o'],
					enabledModels: ['gpt-4o'],
					selectedModel: 'gpt-4o'
				}
			],
			'openai',
			'gpt-4o'
		);

		const entries: ModelLimitEntry[] = [
			{
				providerId: 'openai',
				modelId: 'o3-mini',
				context: 200_000,
				output: 100_000,
				limitSource: 'catalog',
				vision: false,
				visionKnown: true,
				inputModalities: ['text']
			},
			{
				providerId: 'openai',
				modelId: 'gpt-4o',
				context: 128_000,
				output: 16_384,
				limitSource: 'catalog',
				vision: true,
				visionKnown: true,
				inputModalities: ['text', 'image']
			}
		];
		modelStore.applyLimits(entries);

		const disabled = modelStore.limitFor('openai', 'o3-mini');
		expect(disabled?.context).toBe(200_000);
		expect(disabled?.vision).toBe(false);
		expect(disabled?.visionKnown).toBe(true);
		expect(disabled?.inputModalities).toEqual(['text']);

		expect(modelStore.selected?.modelId).toBe('gpt-4o');
		expect(modelStore.selected?.vision).toBe(true);
		expect(modelStore.selected?.context).toBe(128_000);
		expect(modelStore.selected?.inputModalities).toEqual(['text', 'image']);
	});
});
