import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { ProviderSettings } from '$lib/types';

const mocks = vi.hoisted(() => ({
	lookupModelCatalog: vi.fn()
}));

vi.mock('$lib/client/cometmind', () => ({
	lookupModelCatalog: mocks.lookupModelCatalog
}));

import { settingsStore } from './settings.svelte';
import { modelStore } from './model.svelte';
import { defaultSettings } from '$lib/settings/schema';

describe('settingsStore.refreshModelLimits', () => {
	beforeEach(() => {
		mocks.lookupModelCatalog.mockReset();
	});

	it('restores model capabilities after an early sidecar lookup fails', async () => {
		mocks.lookupModelCatalog
			.mockRejectedValueOnce(new Error('sidecar is starting'))
			.mockResolvedValueOnce([
				{
					model_id: 'o3',
					context: 200_000,
					output: 100_000,
					limit_source: 'catalog',
					vision: false,
					vision_known: true,
					input_modalities: ['text'],
					reasoning_effort_options: ['low', 'medium', 'high']
				}
			]);

		const settings: ProviderSettings = {
			...defaultSettings(),
			providers: [
				{
					id: 'openai',
					name: 'OpenAI',
					method: 'openai',
					enabled: true,
					baseURL: '',
					apiKey: '',
					models: ['o3'],
					enabledModels: ['o3'],
					selectedModel: 'o3'
				}
			],
			defaultProviderId: 'openai',
			defaultModelId: 'o3'
		};

		settingsStore.apply(settings);
		await settingsStore.refreshModelLimits();

		expect(mocks.lookupModelCatalog).toHaveBeenCalledTimes(2);
		expect(modelStore.selected?.reasoningEffortOptions).toEqual(['low', 'medium', 'high']);
	});
});
