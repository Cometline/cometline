import { describe, expect, it } from 'vitest';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import type { ProviderSettings } from '$lib/types';
import {
	defaultSettings,
	defaultCometMindSettings,
	normalizeCometMindSettings,
	migrateSingleProvider,
	normalizeSettings,
	parseAndNormalizeSettings,
	runtimeSlice,
	validateSettings
} from './schema';

describe('settings schema', () => {
	it('normalizes the runtime settings contract fixture consumed by CometMind', () => {
		const fixture = JSON.parse(
			readFileSync(
				resolve(
					process.cwd(),
					'../cometmind/internal/config/testdata/cometline-settings.json'
				),
				'utf8'
			)
		) as Partial<ProviderSettings>;
		const settings = normalizeSettings(fixture);

		expect(settings.activeProviderId).toBe('local-llm');
		expect(settings.cometmind.systemPromptPath).toBe('/tmp/SOUL.md');
		expect(settings.cometmind.storage).toMatchObject({
			retentionDays: 90,
			maxSessionsPerWorkspace: 0,
			archivedMemoryPurgeDays: 90,
			vacuumAfterPurge: true,
			backup: {
				enabled: false,
				destinationDir: '',
				intervalHours: 24,
				maxBackups: 7
			}
		});
		expect(runtimeSlice(settings)).toMatchObject({
			provider: 'local-llm',
			model: 'qwen2.5',
			maxTokens: 2048,
			systemPromptPath: '/tmp/SOUL.md'
		});
	});

	it('orders built-in providers for the settings sidebar', () => {
		const settings = defaultSettings();
		expect(settings.providers).toHaveLength(7);
		expect(settings.providers.map((provider) => provider.id)).toEqual([
			'codex',
			'xai',
			'openai',
			'anthropic',
			'opencode-go',
			'ollama',
			'openai-compatible'
		]);
		expect(settings.providers.find((p) => p.id === 'ollama')).toMatchObject({
			method: 'ollama',
			baseURL: 'http://127.0.0.1:11434',
			apiKey: '',
			enabled: false
		});
		expect(settings.providers.find((p) => p.id === 'openai-compatible')?.name).toBe(
			'Advanced / Custom endpoint'
		);
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
		expect(settings.cometmind.acp.enabled).toBe(false);
		expect(settings.cometmind.acp.defaultHarness).toBe('opencode');
	});

	it('normalizes legacy ACP settings with the new harness defaults', () => {
		const defaults = defaultCometMindSettings();
		const normalized = normalizeCometMindSettings({
			...defaults,
			acp: {
				enabled: false,
				defaultHarness: 'codex',
				command: 'custom-agent',
				args: ['--user-controlled'],
				timeout: '1m'
			} as typeof defaults.acp
		});

		expect(normalized.acp).toEqual({ enabled: false, defaultHarness: 'codex' });
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
			'xai',
			'openai',
			'anthropic',
			'opencode-go',
			'ollama',
			'openai-compatible',
			'custom-local'
		]);
	});

	it('normalizes ollama base URLs to native form without /v1', () => {
		const settings = normalizeSettings({
			...defaultSettings(),
			providers: defaultSettings().providers.map((provider) =>
				provider.id === 'ollama'
					? { ...provider, baseURL: 'http://127.0.0.1:11434/v1', apiKey: 'ignored' }
					: provider
			)
		});
		expect(settings.providers.find((p) => p.id === 'ollama')).toMatchObject({
			baseURL: 'http://127.0.0.1:11434',
			apiKey: ''
		});
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

	it('restores fixed provider names and methods while preserving custom configuration', () => {
		const settings = normalizeSettings({
			...defaultSettings(),
			providers: defaultSettings().providers.map((provider) =>
				provider.id === 'openai'
					? { ...provider, name: 'Openai', method: 'openai-compatible' }
					: provider
			)
		});

		expect(settings.providers.find((p) => p.id === 'openai')).toMatchObject({
			name: 'OpenAI',
			method: 'openai'
		});
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

	it('preserves CometMind runtime settings through normalization and validation', () => {
		const base = defaultSettings();
		const settings = normalizeSettings({
			...base,
			cometmind: {
				...base.cometmind,
				skills: {
					...base.cometmind.skills,
					synthesisEnabled: true,
					synthesisProviderId: 'codex',
					synthesisModel: 'gpt-5.1-codex'
				},
				memory: {
					...base.cometmind.memory,
					enabled: true,
					autoExtract: false,
					autoRetrieve: false,
					maxRetrieved: 9,
					taskOutcomeLimit: 4,
					similarityThreshold: 0.72,
					extractionProviderId: 'codex',
					extractionModel: 'gpt-5.1-codex',
					lifecycle: {
						decayHalfLifeDays: 45,
						forgetThreshold: 0.22,
						usageBoostFactor: 0.33,
						maxUsageBoost: 3.5,
						maxMemories: 777,
						compactionTargetRatio: 0.66,
						compactionOnExtract: false
					},
					embedding: {
						providerId: 'openai',
						provider: 'openai',
						model: 'text-embedding-3-small',
						baseURL: 'https://api.openai.com/v1',
						apiKey: 'env'
					}
				},
				jobs: {
					...base.cometmind.jobs,
					doneArchiveDays: 5,
					archivedPurgeDays: 12,
					staleReviewMinutes: 31,
					maxConsecutiveFailures: 4,
					retryCooldownMinutes: 6,
					maxRetryCooldownMinutes: 66,
					notifications: {
						...base.cometmind.jobs.notifications,
						onBlocked: false
					}
				},
				autonomy: {
					...base.cometmind.autonomy,
					enabled: true,
					providerId: 'codex',
					modelId: 'gpt-5.1-codex'
				},
				scheduler: { enabled: true, pollIntervalSeconds: 45 }
			}
		});

		expect(() => validateSettings(settings)).not.toThrow();
		expect(settings.cometmind.skills.synthesisEnabled).toBe(true);
		expect(settings.cometmind.memory.autoRetrieve).toBe(false);
		expect(settings.cometmind.memory.maxRetrieved).toBe(9);
		expect(settings.cometmind.memory.taskOutcomeLimit).toBe(4);
		expect(settings.cometmind.memory.lifecycle.maxMemories).toBe(777);
		expect(settings.cometmind.jobs.doneArchiveDays).toBe(5);
		expect(settings.cometmind.jobs.archivedPurgeDays).toBe(12);
		expect(settings.cometmind.jobs.notifications.onBlocked).toBe(false);
		expect(settings.cometmind.autonomy.providerId).toBe('codex');
		expect(settings.cometmind.scheduler.enabled).toBe(true);
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
