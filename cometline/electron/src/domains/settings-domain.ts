import type { ProviderConfig, ProviderSettings } from '../../../src/lib/types.js';

type JsonRecord = Record<string, unknown>;

export const DESKTOP_TOP_LEVEL_KEYS = ['appearance', 'shortcuts', 'app'];

export interface ProviderEnvironmentOverrides {
	providerId: string | undefined;
	baseURL: string | undefined;
	apiKey: string | undefined;
	selectedModel: string | undefined;
}

export interface MiniWindowState {
	sessionId: string;
	lastActiveAt: number;
	inactivityTimeoutMinutes: number;
}

export interface WorkspaceStore {
	workspacePath: string;
	recentPaths: string[];
}

type JsonFileSystem = Pick<
	typeof import('node:fs'),
	'writeFileSync' | 'chmodSync' | 'renameSync'
>;

function cloneJson<T>(value: T | null | undefined): T | null {
	return JSON.parse(JSON.stringify(value ?? null)) as T | null;
}

export function providerEnvironmentOverrides(
	environment: Record<string, string | undefined>
): ProviderEnvironmentOverrides {
	return {
		providerId: environment.COMETMIND_PROVIDER,
		baseURL: environment.COMETMIND_BASE_URL,
		apiKey:
			environment.COMETMIND_API_KEY ||
			environment.OPENAI_API_KEY ||
			environment.ANTHROPIC_API_KEY,
		selectedModel: environment.COMETMIND_MODEL
	};
}

export function applyProviderEnvironmentOverrides(
	settings: ProviderSettings,
	fromEnv: ProviderEnvironmentOverrides
): ProviderSettings {
	if (fromEnv.providerId) {
		const matched = settings.providers.find((provider) => provider.id === fromEnv.providerId?.trim());
		if (matched) {
			settings.defaultProviderId = matched.id;
			if (!settings.defaultModelId || !matched.enabledModels.includes(settings.defaultModelId)) {
				settings.defaultModelId =
					matched.enabledModels[0] || matched.selectedModel || matched.models[0] || '';
			}
		}
	}
	const active =
		settings.providers.find((provider) => provider.id === settings.defaultProviderId) ??
		settings.providers[0];
	if (fromEnv.baseURL) active.baseURL = fromEnv.baseURL.trim();
	if (fromEnv.apiKey) active.apiKey = fromEnv.apiKey.trim();
	if (fromEnv.selectedModel) {
		const model = fromEnv.selectedModel.trim();
		active.selectedModel = model;
		active.enabled = true;
		if (model && !active.models.includes(model)) active.models = [...active.models, model];
		if (model && !active.enabledModels.includes(model)) active.enabledModels = [model];
		settings.defaultModelId = model;
	}
	return settings;
}

export function selectDefaultProviderAndModel(
	nextProviders: ProviderConfig[],
	current: ProviderSettings,
	preferredDefaultProvider: string,
	preferredDefaultModel: string
) {
	const runtimeProviders = nextProviders.filter(
		(provider) => provider.enabled && provider.enabledModels.length > 0
	);
	const defaultProvider =
		runtimeProviders.find((provider) => provider.id === preferredDefaultProvider) ??
		runtimeProviders.find((provider) => provider.id === current.defaultProviderId) ??
		runtimeProviders[0] ??
		nextProviders[0];
	return {
		defaultProviderId: defaultProvider?.id ?? '',
		defaultModelId:
			defaultProvider &&
			preferredDefaultModel &&
			defaultProvider.enabledModels.includes(preferredDefaultModel)
				? preferredDefaultModel
				: defaultProvider?.enabledModels?.[0] || defaultProvider?.selectedModel || ''
	};
}

export function splitSettingsDocument(merged: JsonRecord | null | undefined) {
	const settings = (cloneJson(merged ?? {}) ?? {}) as JsonRecord;
	const desktop: JsonRecord = {};
	for (const key of DESKTOP_TOP_LEVEL_KEYS) {
		if (settings[key] !== undefined) {
			desktop[key] = settings[key];
			delete settings[key];
		}
	}
	const prompt = (settings.cometmind as JsonRecord | null | undefined)?.systemPromptPath;
	if (prompt !== undefined) desktop.systemPromptPath = prompt;
	return { settings, desktop };
}

export function mergeSettingsDocuments(
	settingsDoc: JsonRecord | null | undefined,
	desktopDoc: JsonRecord | null | undefined
) {
	const out = (cloneJson(settingsDoc ?? {}) ?? {}) as JsonRecord;
	const desktop = desktopDoc ?? {};
	for (const key of DESKTOP_TOP_LEVEL_KEYS) {
		if (desktop[key] !== undefined) out[key] = cloneJson(desktop[key]);
	}
	if (desktop.systemPromptPath !== undefined) {
		out.cometmind = {
			...((out.cometmind as JsonRecord | null | undefined) ?? {}),
			systemPromptPath: desktop.systemPromptPath
		};
	}
	return out;
}

export function writeJsonFileAtomic(
	fs: JsonFileSystem,
	targetPath: string,
	data: unknown,
	mode: number,
	processId: number
) {
	// Renaming a same-directory temporary file prevents readers seeing partial JSON.
	const tmpPath = `${targetPath}.${processId}.tmp`;
	fs.writeFileSync(tmpPath, JSON.stringify(data, null, 2), { mode });
	try {
		fs.chmodSync(tmpPath, mode);
	} catch {
		/* ignore */
	}
	fs.renameSync(tmpPath, targetPath);
}

export function miniWindowStateFromSettings(settings: {
	app?: Partial<ProviderSettings['app']>;
}): MiniWindowState {
	return {
		sessionId: String(settings.app?.miniWindowSessionId || ''),
		lastActiveAt: Number(settings.app?.miniWindowLastActiveAt || 0),
		inactivityTimeoutMinutes: Number(settings.app?.miniWindowInactivityTimeoutMinutes || 30)
	};
}

export function withMiniWindowState(
	settings: ProviderSettings,
	partial: { sessionId?: unknown; lastActiveAt?: unknown } | null | undefined
): ProviderSettings {
	return {
		...settings,
		app: {
			...settings.app,
			...(typeof partial?.sessionId === 'string'
				? { miniWindowSessionId: partial.sessionId }
				: {}),
			...(Number.isFinite(partial?.lastActiveAt)
				? { miniWindowLastActiveAt: Math.max(0, Math.floor(Number(partial?.lastActiveAt))) }
				: {})
		}
	};
}

export function pruneWorkspaceStoreValues(
	store: WorkspaceStore,
	workspacePathExists: (candidate: string) => boolean
) {
	let workspacePath = store.workspacePath;
	if (workspacePath && !workspacePathExists(workspacePath)) workspacePath = '';
	const recentPaths = store.recentPaths.filter((item) => workspacePathExists(item));
	const removedRecent = store.recentPaths.length - recentPaths.length;
	const clearedCurrent = Boolean(store.workspacePath && !workspacePath);
	return {
		store: { workspacePath, recentPaths },
		removedRecent,
		clearedCurrent,
		changed: clearedCurrent || removedRecent > 0
	};
}

export function rememberWorkspacePathValue(
	store: WorkspaceStore,
	workspacePath: string,
	resolvePath: (candidate: string) => string
) {
	const clean = resolvePath(workspacePath);
	return {
		workspacePath: clean,
		recentPaths: [
			clean,
			...store.recentPaths.filter((item) => resolvePath(item) !== clean)
		].slice(0, 20)
	};
}

export function listRecentWorkspacePathValues(
	store: WorkspaceStore,
	workspacePathExists: (candidate: string) => boolean,
	resolvePath: (candidate: string) => string
) {
	const seen = new Set<string>();
	const out: string[] = [];
	const add = (candidate: string) => {
		const clean = String(candidate || '').trim();
		if (!clean || !workspacePathExists(clean)) return;
		const resolved = resolvePath(clean);
		if (seen.has(resolved)) return;
		seen.add(resolved);
		out.push(resolved);
	};
	add(store.workspacePath);
	for (const item of store.recentPaths) add(item);
	return out;
}
