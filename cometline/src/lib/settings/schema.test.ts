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
	it('orders built-in providers for the settings sidebar', () => {
		const settings = defaultSettings();
		expect(settings.providers).toHaveLength(5);
		expect(settings.providers.map((provider) => provider.id)).toEqual([
			'codex',
			'openai',
			'anthropic',
			'opencode-go',
			'openai-compatible'
		]);
		expect(settings.providers.find((p) => p.id === 'codex')?.apiKey).toBe('');
		expect(settings.activeProviderId).toBe('codex');
		expect(settings.app.personaId).toBe('minako');
		expect(settings.app.hasCompletedSetup).toBe(false);
		expect(settings.app.hasDismissedSetupWizard).toBe(false);
		expect(settings.cometmind.systemPromptPath).toBe('');
		expect(settings.cometmind.maxTokens).toBe(2048);
		expect(settings.cometmind.contextWindowLimit).toBe(128_000);
		expect(settings.cometmind.storage.retentionDays).toBe(90);
		expect(settings.cometmind.storage.maxSessionsPerWorkspace).toBe(0);
	});

	it('round-trips hasDismissedSetupWizard through normalizeSettings', () => {
		const settings = normalizeSettings({
			...defaultSettings(),
			app: { ...defaultSettings().app, hasDismissedSetupWizard: true }
		});
		expect(settings.app.hasDismissedSetupWizard).toBe(true);
	});

	it('defaults webPanelWidth to 0 (use CSS default)', () => {
		expect(defaultSettings().app.webPanelWidth).toBe(0);
	});

	it('normalizes webPanelWidth: floors, clamps negatives, falls back on invalid', () => {
		const base = defaultSettings();
		expect(
			normalizeSettings({ ...base, app: { ...base.app, webPanelWidth: 642.9 } }).app
				.webPanelWidth
		).toBe(642);
		expect(
			normalizeSettings({ ...base, app: { ...base.app, webPanelWidth: -10 } }).app
				.webPanelWidth
		).toBe(0);
		expect(
			normalizeSettings({
				...base,
				app: { ...base.app, webPanelWidth: 'oops' as unknown as number }
			}).app.webPanelWidth
		).toBe(0);
	});

	it('appends custom providers after built-ins', () => {
		const settings = normalizeSettings({
			...defaultSettings(),
			providers: [
				...defaultSettings().providers,
				{
					id: 'custom-local',
					name: 'Local Ollama',
					method: 'openai-compatible',
					enabled: false,
					baseURL: 'http://localhost:11434/v1',
					apiKey: '',
					selectedModel: '',
					models: [],
					enabledModels: []
				}
			]
		});

		expect(settings.providers.map((provider) => provider.id)).toEqual([
			'codex',
			'openai',
			'anthropic',
			'opencode-go',
			'openai-compatible',
			'custom-local'
		]);
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
		expect(codex?.models).toEqual(['gpt-test']);
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

	it('normalizes context window limit to 128k or 256k', () => {
		const settings = normalizeSettings({
			...defaultSettings(),
			cometmind: {
				...defaultSettings().cometmind,
				contextWindowLimit: 256_000
			}
		});
		expect(settings.cometmind.contextWindowLimit).toBe(256_000);

		const invalid = normalizeSettings({
			...defaultSettings(),
			cometmind: {
				...defaultSettings().cometmind,
				contextWindowLimit: 200_000 as 128_000
			}
		});
		expect(invalid.cometmind.contextWindowLimit).toBe(128_000);
	});
});
