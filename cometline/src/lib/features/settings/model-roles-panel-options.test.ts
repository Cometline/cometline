import { describe, expect, it } from 'vitest';
import type { ProviderConfig } from '#lib/types.js';
import {
	buildModelOptions,
	filterModelOptions,
	groupModelOptions,
	labelForModel,
	selectedModelLabel
} from './model-roles-panel-options';

function provider(patch: Partial<ProviderConfig>): ProviderConfig {
	return {
		id: 'p',
		name: 'Provider',
		method: 'openai',
		baseURL: '',
		apiKey: '',
		enabled: true,
		models: [],
		enabledModels: [],
		selectedModel: '',
		...patch
	} as ProviderConfig;
}

const providers = [
	provider({ id: 'a', name: 'Alpha', enabledModels: ['gpt-x', 'text-embedding-3-small'] }),
	provider({ id: 'b', name: '', enabledModels: ['claude/sonnet'] }),
	provider({ id: 'c', name: 'Off', enabled: false, enabledModels: ['ignored'] })
];

describe('model roles panel options', () => {
	it('labels model ids by upper-casing path segments', () => {
		expect(labelForModel('claude/sonnet_4')).toBe('CLAUDE SONNET 4');
	});

	it('builds options from enabled chat models only', () => {
		const options = buildModelOptions(providers);
		expect(options.map((option) => option.id)).toEqual(['a:gpt-x', 'b:claude/sonnet']);
		expect(options[1].providerName).toBe('b');
	});

	it('filters by label, model id, or provider name and groups by provider', () => {
		const options = buildModelOptions(providers);
		expect(filterModelOptions(options, '  ')).toBe(options);
		expect(filterModelOptions(options, 'alpha').map((o) => o.modelId)).toEqual(['gpt-x']);
		const groups = groupModelOptions(options);
		expect(groups.map((group) => [group.providerId, group.options.length])).toEqual([
			['a', 1],
			['b', 1]
		]);
	});

	it('describes the selected default model', () => {
		const options = buildModelOptions(providers);
		expect(selectedModelLabel(options, 'a', 'gpt-x')).toBe('Alpha · gpt-x');
		expect(selectedModelLabel(options, 'a', 'missing')).toBe('No model selected');
	});
});
