import {
	normalizeProviders,
	normalizeSettings,
	parseAndNormalizeSettings,
	validateSettings
} from '../../../src/lib/settings/schema.js';
import type { ProviderSettings } from '../../../src/lib/types.js';
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
	writeJsonFileAtomic,
	type MiniWindowState,
	type WorkspaceStore
} from './settings-domain.js';

type JsonRecord = Record<string, unknown>;
type SettingsFileSystem = Pick<
	typeof import('node:fs'),
	| 'appendFileSync'
	| 'chmodSync'
	| 'existsSync'
	| 'mkdirSync'
	| 'readFileSync'
	| 'renameSync'
	| 'statSync'
	| 'writeFileSync'
>;
type PathService = Pick<typeof import('node:path'), 'join' | 'resolve'>;

interface WorkspaceDialogResult {
	canceled: boolean;
	filePaths: string[];
}

interface WorkspaceDialogOptions {
	properties: ['openDirectory', 'createDirectory'];
	buttonLabel: string;
	title: string;
}

export interface ComposerHistoryEntry {
	display: string;
	timestamp: number;
	workspacePath: string;
	sessionId: string;
}

export interface SettingsDomainDependencies {
	fs: SettingsFileSystem;
	path: PathService;
	homedir: () => string;
	environment: Record<string, string | undefined>;
	processId: number;
	now: () => number;
	readSavedPersonaId: (saved: unknown) => string;
	resolveNextPersonaId: (settings: Partial<ProviderSettings>, current: ProviderSettings) => string;
	resolveSystemPromptPath: (personaId: string, settings?: unknown) => string;
	getFocusedWindow: () => unknown;
	showOpenDialog: (
		window: unknown,
		options: WorkspaceDialogOptions
	) => Promise<WorkspaceDialogResult>;
}

const COMPOSER_HISTORY_MAX_ENTRIES = 2000;

/** Owns persisted settings, workspace selection/storage, mini state, and composer history. */
export function createSettingsDomain(dependencies: SettingsDomainDependencies) {
	const { fs, path } = dependencies;
	const cometMindDirectory = () => {
		const directory = path.join(dependencies.homedir(), '.cometmind');
		if (!fs.existsSync(directory)) fs.mkdirSync(directory, { recursive: true });
		return directory;
	};
	const settingsPath = () => path.join(cometMindDirectory(), 'cometline-settings.json');
	const desktopSettingsPath = () => path.join(cometMindDirectory(), 'cometline-desktop.json');
	const composerHistoryPath = () => path.join(cometMindDirectory(), 'composer-history.jsonl');
	const workspaceStoragePath = () => path.join(cometMindDirectory(), 'cometline-workspace.json');

	function readJsonFileIfExists(filePath: string): JsonRecord | null {
		if (!fs.existsSync(filePath)) return null;
		try {
			return JSON.parse(fs.readFileSync(filePath, 'utf8')) as JsonRecord;
		} catch {
			return null;
		}
	}

	function workspacePathExists(candidate: string) {
		const clean = String(candidate || '').trim();
		if (!clean) return false;
		try {
			return fs.existsSync(clean) && fs.statSync(clean).isDirectory();
		} catch {
			return false;
		}
	}

	function writeWorkspaceStore(store: WorkspaceStore) {
		fs.writeFileSync(workspaceStoragePath(), JSON.stringify(store, null, 2));
		fs.chmodSync(workspaceStoragePath(), 0o600);
	}

	function readWorkspaceStore(): WorkspaceStore {
		try {
			const parsed = JSON.parse(fs.readFileSync(workspaceStoragePath(), 'utf8')) as JsonRecord;
			return {
				workspacePath: String(parsed?.workspacePath || '').trim(),
				recentPaths: Array.isArray(parsed?.recentPaths)
					? parsed.recentPaths.map((item) => String(item || '').trim()).filter(Boolean)
					: []
			};
		} catch {
			return { workspacePath: '', recentPaths: [] };
		}
	}

	function pruneWorkspaceStore() {
		const result = pruneWorkspaceStoreValues(readWorkspaceStore(), workspacePathExists);
		if (result.changed) writeWorkspaceStore(result.store);
		return { removedRecent: result.removedRecent, clearedCurrent: result.clearedCurrent };
	}

	function readStoredWorkspacePath() {
		pruneWorkspaceStore();
		const { workspacePath } = readWorkspaceStore();
		if (workspacePath && workspacePathExists(workspacePath)) return path.resolve(workspacePath);
		return '';
	}

	function rememberWorkspacePath(workspacePath: string) {
		const next = rememberWorkspacePathValue(readWorkspaceStore(), workspacePath, path.resolve);
		writeWorkspaceStore(next);
		return next.workspacePath;
	}

	function writeStoredWorkspacePath(workspacePath: string) {
		return rememberWorkspacePath(workspacePath);
	}

	function listRecentWorkspacePaths() {
		pruneWorkspaceStore();
		return listRecentWorkspacePathValues(readWorkspaceStore(), workspacePathExists, path.resolve);
	}

	function removeRecentWorkspacePath(workspacePath: string) {
		const clean = String(workspacePath || '').trim();
		if (!clean) return { removed: false };
		const target = path.resolve(clean);
		const store = readWorkspaceStore();
		const recentPaths = store.recentPaths.filter((item) => path.resolve(item) !== target);
		if (recentPaths.length === store.recentPaths.length) return { removed: false };
		writeWorkspaceStore({ workspacePath: store.workspacePath, recentPaths });
		return { removed: true };
	}

	function filterExistingWorkspacePaths(paths: unknown[]) {
		const seen = new Set<string>();
		const out: string[] = [];
		for (const candidate of paths) {
			const clean = String(candidate || '').trim();
			if (!clean || !workspacePathExists(clean)) continue;
			const resolved = path.resolve(clean);
			if (seen.has(resolved)) continue;
			seen.add(resolved);
			out.push(resolved);
		}
		return out;
	}

	function defaultWorkspacePath() {
		const defaultPath = path.join(dependencies.homedir(), 'Cometline');
		if (!fs.existsSync(defaultPath)) fs.mkdirSync(defaultPath, { recursive: true });
		return defaultPath;
	}

	function getWorkspacePath() {
		if (dependencies.environment.COMETMIND_WORKSPACE_PATH) {
			return path.resolve(dependencies.environment.COMETMIND_WORKSPACE_PATH);
		}
		return readStoredWorkspacePath() || defaultWorkspacePath();
	}

	async function chooseWorkspacePath() {
		const result = await dependencies.showOpenDialog(dependencies.getFocusedWindow(), {
			properties: ['openDirectory', 'createDirectory'],
			buttonLabel: 'Select workspace',
			title: 'Choose a workspace folder'
		});
		if (result.canceled || result.filePaths.length === 0) return null;
		return path.resolve(result.filePaths[0]);
	}

	async function selectWorkspacePath() {
		const selected = await chooseWorkspacePath();
		return selected ? writeStoredWorkspacePath(selected) : null;
	}

	async function browseWorkspacePath() {
		return chooseWorkspacePath();
	}

	function normalizeOptions(personaId = 'minako', settings: unknown = undefined) {
		return {
			fallbackWorkspacePath: readStoredWorkspacePath() || defaultWorkspacePath(),
			systemPromptPath: dependencies.resolveSystemPromptPath(personaId, settings)
		};
	}

	function migrateSplitSettingsIfNeeded() {
		const runtimePath = settingsPath();
		const desktopPath = desktopSettingsPath();
		const saved = readJsonFileIfExists(runtimePath) ?? {};
		const hasDesktopKeys = ['appearance', 'shortcuts', 'app'].some(
			(key) => saved[key] !== undefined
		);
		const prompt = (saved.cometmind as JsonRecord | null | undefined)?.systemPromptPath;
		if (!hasDesktopKeys) {
			if (prompt !== undefined) {
				const desktop = readJsonFileIfExists(desktopPath) ?? {};
				if (desktop.systemPromptPath === undefined) {
					writeJsonFileAtomic(
						fs,
						desktopPath,
						{ ...desktop, systemPromptPath: prompt },
						0o600,
						dependencies.processId
					);
				}
			}
			return;
		}
		const desktop = readJsonFileIfExists(desktopPath) ?? {};
		const split = splitSettingsDocument(saved);
		writeJsonFileAtomic(fs, desktopPath, { ...desktop, ...split.desktop }, 0o600, dependencies.processId);
		writeJsonFileAtomic(fs, runtimePath, split.settings, 0o600, dependencies.processId);
	}

	function readSavedProviderSettings() {
		migrateSplitSettingsIfNeeded();
		const saved = mergeSettingsDocuments(
			readJsonFileIfExists(settingsPath()) ?? {},
			readJsonFileIfExists(desktopSettingsPath()) ?? {}
		);
		const personaId = dependencies.readSavedPersonaId(saved);
		return parseAndNormalizeSettings(saved, normalizeOptions(personaId, saved));
	}

	function readProviderSettings() {
		return applyProviderEnvironmentOverrides(
			readSavedProviderSettings(),
			providerEnvironmentOverrides(dependencies.environment)
		);
	}

	function writeProviderSettings(settings: Partial<ProviderSettings>) {
		const current = readSavedProviderSettings();
		const providers = Array.isArray(settings.providers)
			? normalizeProviders(settings.providers)
			: current.providers;
		const preferredProvider = String(settings.defaultProviderId ?? current.defaultProviderId ?? '').trim();
		const preferredModel = String(settings.defaultModelId ?? current.defaultModelId ?? '').trim();
		const { defaultProviderId, defaultModelId } = selectDefaultProviderAndModel(
			providers,
			current,
			preferredProvider,
			preferredModel
		);
		const app = { ...current.app, ...settings.app };
		const personaId = dependencies.resolveNextPersonaId(settings, current);
		app.personaId = personaId;
		const next = validateSettings(
			normalizeSettings(
				{
					providers,
					defaultProviderId,
					defaultModelId,
					appearance: settings.appearance ?? current.appearance,
					shortcuts: settings.shortcuts ?? current.shortcuts,
					cometmind: {
						...(settings.cometmind ?? current.cometmind),
						systemPromptPath: dependencies.resolveSystemPromptPath(personaId, { app })
					},
					app
				},
				normalizeOptions(personaId, { app })
			)
		);
		const split = splitSettingsDocument(next as unknown as JsonRecord);
		writeJsonFileAtomic(fs, settingsPath(), split.settings, 0o600, dependencies.processId);
		writeJsonFileAtomic(fs, desktopSettingsPath(), split.desktop, 0o600, dependencies.processId);
		return next;
	}

	function readMiniWindowState(): MiniWindowState {
		return miniWindowStateFromSettings(readProviderSettings());
	}

	function writeMiniWindowState(partial: {
		sessionId?: unknown;
		lastActiveAt?: unknown;
	}): MiniWindowState {
		return miniWindowStateFromSettings(
			writeProviderSettings(withMiniWindowState(readProviderSettings(), partial))
		);
	}

	function parseComposerHistoryEntry(value: unknown): ComposerHistoryEntry | null {
		if (!value || typeof value !== 'object' || Array.isArray(value)) return null;
		const entry = value as JsonRecord;
		const display = typeof entry.display === 'string' ? entry.display : '';
		if (!display.trim()) return null;
		return {
			display,
			timestamp:
				typeof entry.timestamp === 'number' && Number.isFinite(entry.timestamp)
					? entry.timestamp
					: dependencies.now(),
			workspacePath:
				typeof entry.workspacePath === 'string'
					? entry.workspacePath
					: typeof entry.project === 'string'
						? entry.project
						: '',
			sessionId: typeof entry.sessionId === 'string' ? entry.sessionId : ''
		};
	}

	function serializeComposerHistoryEntry(entry: ComposerHistoryEntry) {
		return JSON.stringify({
			display: entry.display,
			timestamp: entry.timestamp,
			workspacePath: entry.workspacePath,
			sessionId: entry.sessionId
		});
	}

	function loadComposerHistoryEntries(): ComposerHistoryEntry[] {
		const filePath = composerHistoryPath();
		if (!fs.existsSync(filePath)) return [];
		try {
			const entries: ComposerHistoryEntry[] = [];
			for (const line of fs.readFileSync(filePath, 'utf8').split('\n')) {
				if (!line.trim()) continue;
				try {
					const entry = parseComposerHistoryEntry(JSON.parse(line));
					if (entry) entries.push(entry);
				} catch {
					/* skip corrupt lines */
				}
			}
			return entries;
		} catch {
			return [];
		}
	}

	function writeComposerHistoryEntries(entries: ComposerHistoryEntry[]) {
		const body = entries.length ? `${entries.map(serializeComposerHistoryEntry).join('\n')}\n` : '';
		const filePath = composerHistoryPath();
		const tempPath = `${filePath}.${dependencies.processId}.tmp`;
		fs.writeFileSync(tempPath, body, { mode: 0o600 });
		try {
			fs.chmodSync(tempPath, 0o600);
		} catch {
			/* ignore */
		}
		fs.renameSync(tempPath, filePath);
	}

	function appendComposerHistoryEntry(rawEntry: unknown) {
		const entry = parseComposerHistoryEntry(rawEntry);
		if (!entry) return { ok: false, error: 'Invalid history entry', entries: loadComposerHistoryEntries() };
		let entries = loadComposerHistoryEntries();
		entries.push(entry);
		if (entries.length > COMPOSER_HISTORY_MAX_ENTRIES) {
			entries = entries.slice(entries.length - COMPOSER_HISTORY_MAX_ENTRIES);
			writeComposerHistoryEntries(entries);
		} else {
			const filePath = composerHistoryPath();
			fs.appendFileSync(filePath, `${serializeComposerHistoryEntry(entry)}\n`, { mode: 0o600 });
			try {
				fs.chmodSync(filePath, 0o600);
			} catch {
				/* ignore */
			}
		}
		return { ok: true, entries };
	}

	return {
		appendComposerHistoryEntry,
		filterExistingWorkspacePaths,
		getWorkspacePath,
		listRecentWorkspacePaths,
		loadComposerHistoryEntries,
		pruneWorkspaceStore,
		readMiniWindowState,
		readProviderSettings,
		readSavedProviderSettings,
		removeRecentWorkspacePath,
		browseWorkspacePath,
		selectWorkspacePath,
		writeMiniWindowState,
		writeProviderSettings,
		writeStoredWorkspacePath
	};
}
