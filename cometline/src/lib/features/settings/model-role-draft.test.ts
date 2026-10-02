import { describe, expect, it } from 'vitest';
import { defaultCometMindSettings } from '#lib/cometmind-settings.js';
import type { ProviderConfig, ProviderSettings } from '#lib/types.js';
import { normalizeModelRoleDraft } from './model-role-draft';

function provider(
	partial: Pick<ProviderConfig, 'id' | 'enabled' | 'enabledModels'>
): ProviderConfig {
	return {
		name: partial.id,
		method: 'openai',
		baseURL: '',
		apiKey: '',
		selectedModel: partial.enabledModels[0] ?? '',
		models: partial.enabledModels,
		...partial
	};
}

function draft(
	partial: Partial<ProviderSettings> & Pick<ProviderSettings, 'providers'>
): ProviderSettings {
	return {
		defaultProviderId: '',
		defaultModelId: '',
		appearance: {} as ProviderSettings['appearance'],
		shortcuts: {} as ProviderSettings['shortcuts'],
		app: {} as ProviderSettings['app'],
		cometmind: defaultCometMindSettings(),
		...partial
	};
}

describe('normalizeModelRoleDraft', () => {
	it('keeps a valid selection and returns the same draft', () => {
		const input = draft({
			providers: [provider({ id: 'anthropic', enabled: true, enabledModels: ['claude'] })],
			defaultProviderId: 'anthropic',
			defaultModelId: 'claude'
		});
		expect(normalizeModelRoleDraft(input)).toBe(input);
	});

	it('snaps a missing default to the first chat model and drops dead role pins', () => {
		const input = draft({
			providers: [
				provider({
					id: 'openai',
					enabled: true,
					enabledModels: ['text-embedding-3-small', 'gpt-4.1']
				}),
				provider({ id: 'gone', enabled: false, enabledModels: ['old'] })
			],
			defaultProviderId: 'missing',
			defaultModelId: 'nope',
			cometmind: {
				...defaultCometMindSettings(),
				titleProviderId: 'gone',
				titleModelId: 'old',
				autonomy: {
					...defaultCometMindSettings().autonomy,
					providerId: 'openai',
					modelId: 'gpt-4.1'
				}
			}
		});

		const next = normalizeModelRoleDraft(input);
		expect(next.defaultProviderId).toBe('openai');
		expect(next.defaultModelId).toBe('gpt-4.1');
		expect(next.cometmind.titleProviderId).toBe('');
		expect(next.cometmind.titleModelId).toBe('');
		expect(next.cometmind.autonomy.providerId).toBe('openai');
	});
});
