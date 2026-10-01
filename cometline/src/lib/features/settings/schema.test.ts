import { describe, expect, it } from 'vitest';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import type { ProviderSettings } from '$lib/types';
import {
	defaultSettings,
	defaultCometMindSettings,
	normalizeCometMindSettings,
	normalizeSettings,
	parseAndNormalizeSettings,
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

		expect(settings.defaultProviderId).toBe('local-llm');
		expect(settings.defaultModelId).toBe('qwen2.5');
		expect(settings).not.toHaveProperty('activeProviderId');
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
		expect(settings.defaultProviderId).toBe('codex');
		expect(settings).not.toHaveProperty('activeProviderId');
		expect(settings.app.personaId).toBe('minako');
		expect(settings.app.hasCompletedSetup).toBe(false);
		expect(settings.app.hasDismissedSetupWizard).toBe(false);
		expect(settings.app.screenCapturePreferred).toBe(false);
		expect(settings.app.confirmBeforeDeletingMedia).toBe(true);
		expect(settings.cometmind.systemPromptPath).toBe('');
		expect(settings.cometmind.storage.retentionDays).toBe(90);
		expect(settings.cometmind.storage.detachedMediaRetentionDays).toBe(30);
		expect(settings.cometmind.storage.maxSessionsPerWorkspace).toBe(0);
		expect(settings.cometmind.acp.enabled).toBe(false);
		expect(settings.cometmind.acp.defaultHarness).toBe('opencode');
	});

	it('defaults missing Gallery delete confirmation settings to enabled', () => {
		const base = defaultSettings();
		const { confirmBeforeDeletingMedia: _omitted, ...legacyApp } = base.app;
		const settings = normalizeSettings({ ...base, app: legacyApp as typeof base.app });

		expect(settings.app.confirmBeforeDeletingMedia).toBe(true);
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

	it('defaults workspacePanelWidth to 0 (use CSS default)', () => {
		expect(defaultSettings().app.workspacePanelWidth).toBe(0);
		expect(defaultSettings().app.workspacePanelRatio).toBe(0);
	});

	it('defaults fileSearchSource to wiki and normalizes invalid values', () => {
		expect(defaultSettings().app.fileSearchSource).toBe('wiki');
		const base = defaultSettings();
		expect(
			normalizeSettings({
				...base,
				app: { ...base.app, fileSearchSource: 'workspace' }
			}).app.fileSearchSource
		).toBe('workspace');
		expect(
			normalizeSettings({
				...base,
				app: { ...base.app, fileSearchSource: 'changes' as never }
			}).app.fileSearchSource
		).toBe('wiki');
	});

	it('normalizes terminal appearance for settings created before terminal preferences', () => {
		const settings = normalizeSettings({
			...defaultSettings(),
			appearance: {
				...defaultSettings().appearance,
				terminal: undefined as never
			}
		});

		expect(settings.appearance.terminal).toMatchObject({
			fontSize: 12,
			theme: 'cometline-dark'
		});
	});

	it('normalizes response complete sound for settings created before that preference', () => {
		const settings = normalizeSettings({
			...defaultSettings(),
			appearance: {
				...defaultSettings().appearance,
				responseCompleteSound: undefined as never
			}
		});

		expect(settings.appearance.responseCompleteSound).toEqual({
			enabled: true,
			volume: 0.7
		});
	});

	it('clamps response complete sound volume to 0–1', () => {
		const settings = normalizeSettings({
			...defaultSettings(),
			appearance: {
				...defaultSettings().appearance,
				responseCompleteSound: { enabled: false, volume: 1.8 }
			}
		});

		expect(settings.appearance.responseCompleteSound).toEqual({
			enabled: false,
			volume: 1
		});
	});

	it('normalizes workspacePanelWidth: floors, clamps negatives, falls back on invalid', () => {
		const base = defaultSettings();
		expect(
			normalizeSettings({ ...base, app: { ...base.app, workspacePanelWidth: 642.9 } }).app
				.workspacePanelWidth
		).toBe(642);
		expect(
			normalizeSettings({ ...base, app: { ...base.app, workspacePanelWidth: -10 } }).app
				.workspacePanelWidth
		).toBe(0);
		expect(
			normalizeSettings({
				...base,
				app: { ...base.app, workspacePanelWidth: 'oops' as unknown as number }
			}).app.workspacePanelWidth
		).toBe(0);
	});

	it('normalizes workspacePanelRatio: caps at 2/3, falls back on invalid', () => {
		const base = defaultSettings();
		expect(
			normalizeSettings({ ...base, app: { ...base.app, workspacePanelRatio: 0.5 } }).app
				.workspacePanelRatio
		).toBe(0.5);
		expect(
			normalizeSettings({ ...base, app: { ...base.app, workspacePanelRatio: 0.9 } }).app
				.workspacePanelRatio
		).toBeCloseTo(2 / 3);
		expect(
			normalizeSettings({
				...base,
				app: { ...base.app, workspacePanelRatio: 'oops' as unknown as number }
			}).app.workspacePanelRatio
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

	it('defaults detached Gallery media retention and allows disabling it', () => {
		expect(
			normalizeCometMindSettings({ storage: {} as never }).storage.detachedMediaRetentionDays
		).toBe(30);
		expect(
			normalizeCometMindSettings({
				storage: { detachedMediaRetentionDays: 0 } as never
			}).storage.detachedMediaRetentionDays
		).toBe(0);
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

	it('normalizeSettings falls back to the first enabled provider when default is empty', () => {
		const settings = normalizeSettings({
			...defaultSettings(),
			providers: defaultSettings().providers.map((p) =>
				p.id === 'codex'
					? {
							...p,
							enabled: true,
							enabledModels: ['gpt-5.4'],
							models: ['gpt-5.4']
						}
					: { ...p, enabled: false, enabledModels: [] }
			),
			defaultProviderId: '',
			defaultModelId: ''
		});
		expect(settings.defaultProviderId).toBe('codex');
		expect(settings.defaultModelId).toBe('gpt-5.4');
	});

	it('normalizeSettings keeps an explicit default provider', () => {
		const settings = normalizeSettings({
			...defaultSettings(),
			providers: defaultSettings().providers.map((p) => {
				if (p.id === 'codex') {
					return {
						...p,
						enabled: true,
						enabledModels: ['gpt-5.4'],
						models: ['gpt-5.4']
					};
				}
				if (p.id === 'opencode-go') {
					return {
						...p,
						enabled: true,
						enabledModels: ['deepseek-v4-flash'],
						models: ['deepseek-v4-flash']
					};
				}
				return { ...p, enabled: false, enabledModels: [] };
			}),
			defaultProviderId: 'opencode-go',
			defaultModelId: 'deepseek-v4-flash'
		});
		expect(settings.defaultProviderId).toBe('opencode-go');
		expect(settings.defaultModelId).toBe('deepseek-v4-flash');
	});

	it('validateSettings rejects empty providers list', () => {
		const settings = defaultSettings();
		settings.providers = [];
		expect(() => validateSettings(settings)).toThrow();
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
});
