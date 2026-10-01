import { describe, expect, it } from 'vitest';
import type { ProviderConfig } from '$lib/types';
import {
	countEnabledModels,
	countEnabledProviders,
	filterProviderModels,
	modelsSectionWarningText
} from './settings-panel-models';

function provider(
	partial: Pick<ProviderConfig, 'id' | 'enabled' | 'models' | 'enabledModels'>
): ProviderConfig {
	return {
		name: partial.id,
		method: 'openai',
		baseURL: '',
		apiKey: '',
		selectedModel: partial.enabledModels[0] ?? '',
		...partial
	};
}

describe('filterProviderModels', () => {
	const openai = provider({
		id: 'openai',
		enabled: true,
		models: ['gpt-5', 'GPT-4o', 'o3'],
		enabledModels: []
	});

	it('returns no models without a provider', () => {
		expect(filterProviderModels(undefined, 'gpt')).toEqual([]);
	});

	it('returns the provider model list as-is for a blank search', () => {
		expect(filterProviderModels(openai, '   ')).toBe(openai.models);
	});

	it('matches case-insensitively on trimmed search text', () => {
		expect(filterProviderModels(openai, ' gpt ')).toEqual(['gpt-5', 'GPT-4o']);
	});
});

describe('enabled counts', () => {
	const providers = [
		provider({ id: 'a', enabled: true, models: ['m1', 'm2'], enabledModels: ['m1', 'm2'] }),
		provider({ id: 'b', enabled: false, models: ['m3'], enabledModels: ['m3'] }),
		provider({ id: 'c', enabled: true, models: ['m4'], enabledModels: [] })
	];

	it('counts enabled providers', () => {
		expect(countEnabledProviders(providers)).toBe(2);
	});

	it('counts enabled models only on enabled providers', () => {
		expect(countEnabledModels(providers)).toBe(2);
	});
});

describe('modelsSectionWarningText', () => {
	it('warns on the models tab with pending changes and no enabled models', () => {
		expect(modelsSectionWarningText('models', true, 0)).toBe(
			'Enable at least one model to send messages.'
		);
	});

	it('stays silent otherwise', () => {
		expect(modelsSectionWarningText('models', false, 0)).toBe('');
		expect(modelsSectionWarningText('models', true, 1)).toBe('');
		expect(modelsSectionWarningText('app', true, 0)).toBe('');
	});
});
