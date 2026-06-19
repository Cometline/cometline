import { describe, expect, it } from 'vitest';
import {
	defaultSettings,
	migrateSingleProvider,
	normalizeSettings,
	parseAndNormalizeSettings,
	runtimeSlice,
	validateSettings
} from './schema';

describe('settings schema', () => {
	it('normalizes default providers', () => {
		const settings = defaultSettings();
		expect(settings.providers).toHaveLength(5);
		expect(settings.providers.find((p) => p.id === 'codex')?.apiKey).toBe('');
		expect(settings.app.iconVariant).toBe('default');
		expect(settings.cometmind.systemPromptPath).toBe('');
		expect(settings.cometmind.maxTokens).toBe(2048);
		expect(settings.cometmind.storage.retentionDays).toBe(90);
		expect(settings.cometmind.storage.maxSessionsPerWorkspace).toBe(0);
	});

	it('normalizes Codex without an API key', () => {
		const settings = normalizeSettings({
			...defaultSettings(),
			providers: defaultSettings().providers.map((provider) =>
				provider.id === 'codex'
					? { ...provider, apiKey: 'should-not-persist', models: ['gpt-test'] }
					: provider
			)
		});

		const codex = settings.providers.find((p) => p.id === 'codex');
		expect(codex?.apiKey).toBe('');
		expect(codex?.models).toContain('gpt-5.4');
		expect(codex?.models).toContain('gpt-test');
	});

	it('allows disabling session retention with zero days', () => {
		const settings = normalizeSettings({
			...defaultSettings(),
			cometmind: {
				...defaultSettings().cometmind,
				storage: {
					...defaultSettings().cometmind.storage,
					retentionDays: 0
				}
			}
		});
		expect(settings.cometmind.storage.retentionDays).toBe(0);
		expect(settings.cometmind.storage.archivedMemoryPurgeDays).toBe(90);
	});

	it('migrates legacy single-provider format', () => {
		const migrated = migrateSingleProvider({
			provider: 'openai',
			baseURL: 'https://api.example.com/v1',
			apiKey: 'key',
			selectedModel: 'gpt-4'
		});
		expect(migrated?.providers).toHaveLength(1);
		expect(migrated?.activeProviderId).toBe('openai');
	});

	it('preserves renamed built-in provider names', () => {
		const settings = normalizeSettings({
			...defaultSettings(),
			providers: defaultSettings().providers.map((provider) =>
				provider.id === 'openai-compatible'
					? { ...provider, name: 'Local Ollama' }
					: provider
			)
		});

		expect(settings.providers.find((p) => p.id === 'openai-compatible')?.name).toBe(
			'Local Ollama'
		);
	});

	it('parseAndNormalizeSettings applies systemPromptPath option', () => {
		const settings = parseAndNormalizeSettings({}, { systemPromptPath: '/tmp/SOUL.md' });
		expect(settings.cometmind.systemPromptPath).toBe('/tmp/SOUL.md');
	});

	it('runtimeSlice projects active provider', () => {
		const settings = normalizeSettings({
			...defaultSettings(),
			providers: defaultSettings().providers.map((p) =>
				p.id === 'openai'
					? {
							...p,
							enabled: true,
							enabledModels: ['gpt-4o'],
							models: ['gpt-4o']
						}
					: { ...p, enabled: false, enabledModels: [] }
			),
			activeProviderId: 'openai',
			cometmind: {
				...defaultSettings().cometmind,
				systemPromptPath: '/tmp/SOUL.md'
			}
		});
		const slice = runtimeSlice(settings);
		expect(slice?.provider).toBe('openai');
		expect(slice?.model).toBe('gpt-4o');
		expect(slice?.maxTokens).toBe(2048);
		expect(slice?.systemPromptPath).toBe('/tmp/SOUL.md');
		expect(slice?.providers).toHaveLength(1);
	});

	it('validateSettings rejects empty providers list', () => {
		const settings = defaultSettings();
		settings.providers = [];
		expect(() => validateSettings(settings)).toThrow();
	});

	it('persists custom CometMind max tokens into runtime slice', () => {
		const settings = normalizeSettings({
			...defaultSettings(),
			providers: defaultSettings().providers.map((p) =>
				p.id === 'openai'
					? {
							...p,
							enabled: true,
							enabledModels: ['gpt-4o'],
							models: ['gpt-4o']
						}
					: { ...p, enabled: false, enabledModels: [] }
			),
			activeProviderId: 'openai',
			cometmind: {
				...defaultSettings().cometmind,
				maxTokens: 3072
			}
		});

		expect(settings.cometmind.maxTokens).toBe(3072);
		expect(runtimeSlice(settings)?.maxTokens).toBe(3072);
	});
});
