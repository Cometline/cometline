import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { afterEach, describe, expect, it } from 'vitest';

import { defaultSettings } from '../../../src/lib/settings/schema.js';
import type { ProviderConfig, ProviderSettings } from '../../../src/lib/types.js';
import {
	applyProviderEnvironmentOverrides,
	listRecentWorkspacePathValues,
	mergeSettingsDocuments,
	miniWindowStateFromSettings,
	providerEnvironmentOverrides,
	pruneWorkspaceStoreValues,
	rememberWorkspacePathValue,
	selectDefaultProviderAndModel,
	splitSettingsDocument,
	withMiniWindowState,
	writeJsonFileAtomic
} from './settings-domain.js';
import { createSettingsDomain } from './settings.js';

const temporaryDirectories: string[] = [];

afterEach(() => {
	for (const directory of temporaryDirectories.splice(0)) {
		fs.rmSync(directory, { force: true, recursive: true });
	}
});

function provider(id: string, options: Partial<ProviderConfig> = {}): ProviderConfig {
	return {
		id,
		name: id,
		method: 'openai',
		enabled: true,
		baseURL: `https://${id}.example.test/v1`,
		apiKey: `${id}-saved-key`,
		selectedModel: `${id}-selected`,
		models: [`${id}-saved`],
		enabledModels: [`${id}-saved`],
		...options
	};
}

function settingsWithProviders(providers: ProviderConfig[]): ProviderSettings {
	const settings = defaultSettings();
	settings.providers = providers;
	settings.defaultProviderId = providers[0]?.id ?? '';
	settings.defaultModelId = providers[0]?.enabledModels[0] ?? '';
	return settings;
}

function createTestDomain(
	homeDirectory: string,
	environment: Record<string, string | undefined> = {},
	showOpenDialog: () => Promise<{ canceled: boolean; filePaths: string[] }> = async () => ({
		canceled: true,
		filePaths: []
	})
) {
	return createSettingsDomain({
		fs,
		path,
		homedir: () => homeDirectory,
		environment,
		processId: 9876,
		now: () => 1_700_000_000_000,
		readSavedPersonaId: () => 'minako',
		resolveNextPersonaId: (settings, current) => settings.app?.personaId ?? current.app.personaId,
		resolveSystemPromptPath: (personaId) => `/souls/${personaId}.md`,
		getFocusedWindow: () => null,
		showOpenDialog
	});
}

describe('provider environment overrides', () => {
	it('applies provider, base URL, API key, and model overrides to the selected provider', () => {
		const settings = settingsWithProviders([provider('openai'), provider('anthropic')]);
		const result = applyProviderEnvironmentOverrides(
			settings,
			providerEnvironmentOverrides({
				COMETMIND_PROVIDER: ' anthropic ',
				COMETMIND_BASE_URL: ' https://override.example.test/v1 ',
				COMETMIND_API_KEY: ' cometmind-key ',
				OPENAI_API_KEY: ' openai-key ',
				ANTHROPIC_API_KEY: ' anthropic-key ',
				COMETMIND_MODEL: ' claude-characterization '
			})
		);
		expect(result.defaultProviderId).toBe('anthropic');
		expect(result.defaultModelId).toBe('claude-characterization');
		expect(result.providers[1]).toMatchObject({
			baseURL: 'https://override.example.test/v1',
			apiKey: 'cometmind-key',
			selectedModel: 'claude-characterization',
			enabledModels: ['claude-characterization']
		});
		expect(
			providerEnvironmentOverrides({ OPENAI_API_KEY: 'openai', ANTHROPIC_API_KEY: 'anthropic' })
		).toMatchObject({ apiKey: 'openai' });
	});
});

describe('split settings documents', () => {
	it('moves desktop settings out and gives desktop fields precedence on merge', () => {
		const document = {
			providers: [{ id: 'openai' }],
			appearance: { theme: 'night' },
			shortcuts: { openSettings: { key: ',' } },
			app: { miniWindowSessionId: 'session-1' },
			cometmind: { maxTokens: 4096, systemPromptPath: '/saved/SOUL.md' }
		};
		const split = splitSettingsDocument(document);
		expect(split.settings).toEqual({
			providers: [{ id: 'openai' }],
			cometmind: { maxTokens: 4096, systemPromptPath: '/saved/SOUL.md' }
		});
		expect(split.desktop).toMatchObject({ app: { miniWindowSessionId: 'session-1' } });
		expect(
			mergeSettingsDocuments(split.settings, { ...split.desktop, systemPromptPath: '/desktop/SOUL.md' })
		).toMatchObject({ cometmind: { systemPromptPath: '/desktop/SOUL.md' } });
	});
});

describe('atomic settings writes', () => {
	it('uses an indented 0600 temporary replacement file', () => {
		const directory = fs.mkdtempSync(path.join(os.tmpdir(), 'cometline-settings-'));
		temporaryDirectories.push(directory);
		const target = path.join(directory, 'cometline-settings.json');
		writeJsonFileAtomic(fs, target, { after: true }, 0o600, 4321);
		expect(fs.readFileSync(target, 'utf8')).toBe('{\n  "after": true\n}');
		expect(fs.statSync(target).mode & 0o777).toBe(0o600);
		expect(fs.existsSync(`${target}.4321.tmp`)).toBe(false);
	});
});

describe('default provider and model selection', () => {
	it('falls back from invalid preferences to the active runtime provider then first provider', () => {
		const current = settingsWithProviders([provider('current')]);
		const providers = [
			provider('disabled', { enabled: false, enabledModels: ['disabled-model'] }),
			provider('current', { enabledModels: ['current-first'] }),
			provider('other', { enabledModels: ['other-first'] })
		];
		expect(selectDefaultProviderAndModel(providers, current, 'other', 'other-first')).toEqual({
			defaultProviderId: 'other',
			defaultModelId: 'other-first'
		});
		expect(selectDefaultProviderAndModel(providers, current, 'disabled', 'disabled-model')).toEqual({
			defaultProviderId: 'current',
			defaultModelId: 'current-first'
		});
	});
});

describe('mini window settings', () => {
	it('persists valid state and retains the inactivity timeout', () => {
		const settings = defaultSettings();
		settings.app.miniWindowInactivityTimeoutMinutes = 45;
		const updated = withMiniWindowState(settings, { sessionId: 'session-42', lastActiveAt: -3.8 });
		expect(miniWindowStateFromSettings(updated)).toEqual({
			sessionId: 'session-42',
			lastActiveAt: 0,
			inactivityTimeoutMinutes: 45
		});
	});
});

describe('recent workspace paths', () => {
	it('prunes unavailable paths and deduplicates resolved paths', () => {
		expect(
			pruneWorkspaceStoreValues(
				{ workspacePath: '/missing', recentPaths: ['/one', '/missing', '/two'] },
				(candidate) => candidate !== '/missing'
			)
		).toMatchObject({
			store: { workspacePath: '', recentPaths: ['/one', '/two'] },
			removedRecent: 1,
			clearedCurrent: true
		});
		expect(
			listRecentWorkspacePathValues(
				{ workspacePath: '/current', recentPaths: ['/current', '/alias/project', '/project'] },
				() => true,
				(candidate) => candidate.replace('/alias/project', '/project')
			)
		).toEqual(['/current', '/project']);
		expect(
			rememberWorkspacePathValue(
				{ workspacePath: '/old', recentPaths: ['/alias/project', '/other'] },
				'/project',
				(candidate) => candidate.replace('/alias/project', '/project')
			)
		).toEqual({ workspacePath: '/project', recentPaths: ['/project', '/other'] });
	});
});

describe('settings domain factory', () => {
	it('browses for a workspace without changing the stored default', async () => {
		const homeDirectory = fs.mkdtempSync(path.join(os.tmpdir(), 'cometline-domain-'));
		temporaryDirectories.push(homeDirectory);
		const storedWorkspace = path.join(homeDirectory, 'stored-workspace');
		const browsedWorkspace = path.join(homeDirectory, 'browsed-workspace');
		fs.mkdirSync(storedWorkspace);
		fs.mkdirSync(browsedWorkspace);
		const domain = createTestDomain(homeDirectory, {}, async () => ({
			canceled: false,
			filePaths: [browsedWorkspace]
		}));

		domain.writeStoredWorkspacePath(storedWorkspace);

		expect(await domain.browseWorkspacePath()).toBe(browsedWorkspace);
		expect(domain.getWorkspacePath()).toBe(storedWorkspace);
		expect(domain.listRecentWorkspacePaths()).toEqual([storedWorkspace]);
	});

	it('persists split settings, mini state, workspace store, and composer history without Electron lifecycle', () => {
		const homeDirectory = fs.mkdtempSync(path.join(os.tmpdir(), 'cometline-domain-'));
		temporaryDirectories.push(homeDirectory);
		const workspace = path.join(homeDirectory, 'workspace');
		fs.mkdirSync(workspace);
		const domain = createTestDomain(homeDirectory);
		const settings = defaultSettings();
		settings.app.miniWindowSessionId = 'persisted-mini-session';
		domain.writeProviderSettings(settings);
		expect(domain.readMiniWindowState().sessionId).toBe('persisted-mini-session');
		expect(domain.writeStoredWorkspacePath(workspace)).toBe(workspace);
		expect(domain.listRecentWorkspacePaths()).toEqual([workspace]);
		expect(domain.appendComposerHistoryEntry({ display: 'run tests', project: workspace })).toMatchObject({
			ok: true,
			entries: [{ workspacePath: workspace, timestamp: 1_700_000_000_000 }]
		});
		expect(domain.loadComposerHistoryEntries()).toHaveLength(1);
		const directory = path.join(homeDirectory, '.cometmind');
		expect(fs.statSync(path.join(directory, 'cometline-settings.json')).mode & 0o777).toBe(0o600);
		expect(fs.statSync(path.join(directory, 'cometline-desktop.json')).mode & 0o777).toBe(0o600);
	});
});
