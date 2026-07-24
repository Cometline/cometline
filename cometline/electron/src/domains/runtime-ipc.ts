import type { App, IpcMainEvent, IpcMainInvokeEvent, Shell } from 'electron';

import type { ProviderConfig, ProviderSettings } from '../../../src/lib/types.js';
import type { createOllamaService } from '../services/ollama.js';
import type { createAutoUpdater } from './auto-updater.js';
import type { createCometMindLifecycle } from './cometmind-lifecycle.js';
import { registerIpcHandlers, type IpcHandlers } from './ipc.js';
import {
	getScreenCaptureAccess,
	openScreenCaptureSettings,
	requestScreenCaptureAccess
} from './media-permissions.js';
import type { createPersonas } from './personas.js';
import type { createProviderAuth } from './provider-auth.js';
import type { ShellWindowContext } from './runtime-context.js';
import type { createSettingsDomain } from './settings.js';
import type { createShortcutCoordinator } from './shortcuts.js';
import type { createTerminalManager, TerminalCreateInput } from './terminal.js';
import type { createWindowChrome } from './window-chrome.js';
import type { createWindows } from './windows.js';
import { isExternallyOpenableUrl, readWorkspaceFileForPreview } from './workspace-preview.js';

type SettingsDomain = ReturnType<typeof createSettingsDomain>;
type Windows = ReturnType<typeof createWindows>;
type Terminals = ReturnType<typeof createTerminalManager>;
type Personas = ReturnType<typeof createPersonas>;
type ProviderAuth = ReturnType<typeof createProviderAuth>;
type OllamaService = ReturnType<typeof createOllamaService>;
type CometMindLifecycle = ReturnType<typeof createCometMindLifecycle>;
type AutoUpdater = ReturnType<typeof createAutoUpdater>;
type Shortcuts = ReturnType<typeof createShortcutCoordinator>;
type WindowChrome = ReturnType<typeof createWindowChrome>;

interface NotificationService {
	isSupported(): boolean;
	new (options: { title: string; body: string }): { show(): void };
}

export interface RuntimeIpcDependencies {
	app: Pick<App, 'getVersion'>;
	Notification: NotificationService;
	shell: Pick<Shell, 'openExternal'>;
	workspacePreview: Parameters<typeof readWorkspaceFileForPreview>[0];
	selectBackupFolder(): Promise<{ canceled: boolean; path?: string }>;
	context: ShellWindowContext;
	windows: Windows;
	terminals: Terminals;
	settings: SettingsDomain;
	personas: Personas;
	providerAuth: ProviderAuth;
	ollama: OllamaService;
	cometMind: CometMindLifecycle;
	updater: AutoUpdater;
	shortcuts: Pick<Shortcuts, 'refreshGlobalShortcuts'>;
	applicationMenuTray: { configureApplicationMenu(): void };
	windowChrome: Pick<WindowChrome, 'animateWindowButtons'>;
	broadcastProviderSettingsChanged(settings: ProviderSettings): void;
}

type RuntimeAction = 'none' | 'reload' | 'restart' | 'gateway';

function isRecord(value: unknown): value is Record<string, unknown> {
	return Boolean(value) && typeof value === 'object' && !Array.isArray(value);
}

function record(value: unknown): Record<string, unknown> {
	return isRecord(value) ? value : {};
}

function providerModelConfig(
	value: unknown
): Pick<ProviderConfig, 'method' | 'baseURL' | 'apiKey'> {
	const config = record(value);
	return {
		method: String(config.method || '') as ProviderConfig['method'],
		baseURL: String(config.baseURL || ''),
		apiKey: String(config.apiKey || '')
	};
}

function terminalInput(value: unknown): TerminalCreateInput {
	const input = record(value);
	return {
		sessionId: input.sessionId as string,
		workspacePath: input.workspacePath as string,
		cols: input.cols as number | undefined,
		rows: input.rows as number | undefined
	};
}

function runtimeAction(value: unknown): RuntimeAction {
	const options = record(value);
	const action = options.runtimeAction;
	if (action === 'none' || action === 'reload' || action === 'restart' || action === 'gateway') {
		return action;
	}
	return options.restartCometMind === false ? 'none' : 'restart';
}

/** Composes the stable preload IPC contract from typed runtime domains. */
export function registerRuntimeIpcHandlers(dependencies: RuntimeIpcDependencies) {
	const handlers = {
		jobsNotify: (_event: IpcMainEvent, payload: unknown) => {
			const notification = record(payload);
			if (typeof notification.title !== 'string' || !dependencies.Notification.isSupported())
				return;
			new dependencies.Notification({
				title: notification.title,
				body: typeof notification.body === 'string' ? notification.body : ''
			}).show();
		},
		restartCometMind: async () => {
			await dependencies.cometMind.stop();
			dependencies.cometMind.start();
			await dependencies.cometMind.waitForHealth();
		},
		shortcutCaptureActive: (_event: IpcMainEvent, active: unknown) =>
			dependencies.context.setShortcutCaptureActive(Boolean(active)),
		sessionNavigationSuspended: (_event: IpcMainEvent, suspended: unknown) =>
			dependencies.context.setSessionNavigationSuspended(Boolean(suspended)),
		workspacePanelOpen: (_event: IpcMainEvent, open: unknown) =>
			dependencies.context.setWorkspacePanelOpen(Boolean(open)),
		inboxOpen: (_event: IpcMainEvent, open: unknown) =>
			dependencies.context.setInboxOpen(Boolean(open)),
		confirmCloseWindow: () => dependencies.windows.hideMainWindow(),
		setSidebarOpen: (_event: IpcMainEvent, payload: unknown) =>
			dependencies.windowChrome.animateWindowButtons(
				typeof payload === 'boolean'
					? payload
					: { open: record(payload).open, duration: record(payload).duration }
			),
		getFullScreen: () => {
			const mainWindow = dependencies.windows.getMainWindow();
			return Boolean(mainWindow && !mainWindow.isDestroyed() && mainWindow.isFullScreen());
		},
		getWorkspacePath: () => dependencies.settings.getWorkspacePath(),
		openSessionInMainWindow: (_event: IpcMainInvokeEvent, sessionId: unknown) =>
			dependencies.windows.openSessionInMainWindow(sessionId),
		selectWorkspacePath: () => dependencies.settings.selectWorkspacePath(),
		selectBackupFolder: () => dependencies.selectBackupFolder(),
		setWorkspacePath: (_event: IpcMainInvokeEvent, workspacePath: unknown) =>
			dependencies.settings.writeStoredWorkspacePath(String(workspacePath || '')),
		listRecentWorkspaces: () => dependencies.settings.listRecentWorkspacePaths(),
		removeRecentWorkspacePath: (_event: IpcMainInvokeEvent, workspacePath: unknown) =>
			dependencies.settings.removeRecentWorkspacePath(String(workspacePath || '')),
		filterExistingWorkspacePaths: (_event: IpcMainInvokeEvent, paths: unknown) =>
			dependencies.settings.filterExistingWorkspacePaths(Array.isArray(paths) ? paths : []),
		pruneWorkspaceStore: () => dependencies.settings.pruneWorkspaceStore(),
		readWorkspaceFile: (
			_event: IpcMainInvokeEvent,
			workspacePath: unknown,
			relativePath: unknown
		) =>
			readWorkspaceFileForPreview(dependencies.workspacePreview, workspacePath, relativePath),
		terminalList: (event: IpcMainInvokeEvent) =>
			dependencies.terminals.isMainWindowSender(event) ? dependencies.terminals.list() : [],
		terminalCreate: (event: IpcMainInvokeEvent, payload: unknown = {}) => {
			const input = terminalInput(payload);
			dependencies.terminals.requireInput(event, input.sessionId);
			return dependencies.terminals.create(input.sessionId, input.workspacePath, input);
		},
		terminalWrite: (event: IpcMainInvokeEvent, payload: unknown = {}) => {
			const input = record(payload);
			dependencies.terminals.requireInput(event, input.sessionId);
			return dependencies.terminals.write(input.sessionId as string, input.data as string);
		},
		terminalResize: (event: IpcMainInvokeEvent, payload: unknown = {}) => {
			const input = terminalInput(payload);
			dependencies.terminals.requireInput(event, input.sessionId);
			return dependencies.terminals.resize(input.sessionId, input);
		},
		terminalTerminate: (event: IpcMainInvokeEvent, sessionId: unknown) => {
			dependencies.terminals.requireInput(event, sessionId);
			return dependencies.terminals.terminate(sessionId as string);
		},
		terminalRemove: (event: IpcMainInvokeEvent, sessionId: unknown) => {
			dependencies.terminals.requireInput(event, sessionId);
			return dependencies.terminals.terminate(sessionId as string, true);
		},
		listCustomPersonas: () =>
			dependencies.personas.customPersonasFromSettings(
				dependencies.settings.readProviderSettings()
			),
		readPersonaAvatar: (_event: IpcMainInvokeEvent, id: unknown) =>
			dependencies.personas.readPersonaAvatar(id),
		readBuiltinSoul: (_event: IpcMainInvokeEvent, personaId: unknown) =>
			dependencies.personas.readPersonaSoul(personaId),
		saveCustomPersona: (_event: IpcMainInvokeEvent, payload: unknown) =>
			dependencies.personas.saveCustomPersona(record(payload)),
		deleteCustomPersona: (_event: IpcMainInvokeEvent, id: unknown) =>
			dependencies.personas.deleteCustomPersona(id),
		getProviderSettings: () => dependencies.settings.readProviderSettings(),
		getCodexAuthStatus: () => dependencies.providerAuth.getCodexAuthStatus(),
		startCodexLogin: () => dependencies.providerAuth.startCodexLogin(),
		getXaiAuthStatus: () => dependencies.providerAuth.getXaiAuthStatus(),
		startXaiLogin: () => dependencies.providerAuth.startXaiLogin(),
		getMcpOAuthStatus: (_event: IpcMainInvokeEvent, serverId: unknown) =>
			dependencies.providerAuth.getMcpOAuthStatus(serverId),
		startMcpOAuth: (_event: IpcMainInvokeEvent, payload: unknown) =>
			dependencies.providerAuth.startMcpOAuth(record(payload)),
		readCursorMcpConfig: () => dependencies.providerAuth.readCursorMcpConfig(),
		fetchProviderModels: (_event: IpcMainInvokeEvent, config: unknown) =>
			dependencies.providerAuth.fetchProviderModels(providerModelConfig(config)),
		ollamaHealth: (_event: IpcMainInvokeEvent, baseURL: unknown) =>
			dependencies.ollama.checkHealth(baseURL),
		ollamaModels: (_event: IpcMainInvokeEvent, baseURL: unknown) =>
			dependencies.ollama.listModels(baseURL),
		ollamaDiagnostics: (_event: IpcMainInvokeEvent, baseURL: unknown) =>
			dependencies.ollama.getDiagnostics(baseURL),
		ollamaPull: (_event: IpcMainInvokeEvent, payload: unknown = {}) => {
			const input = record(payload);
			return dependencies.ollama.pullModel({
				baseURL: input.baseURL,
				catalogId: input.catalogId,
				modelName: input.modelName
			});
		},
		ollamaCancelPull: () => dependencies.ollama.cancelPull(),
		saveProviderSettings: async (
			_event: IpcMainInvokeEvent,
			settings: unknown,
			options: unknown = {}
		) => {
			const previous = dependencies.settings.readProviderSettings();
			const saved = dependencies.settings.writeProviderSettings(
				record(settings) as Partial<ProviderSettings>
			);
			const personaIdChanged =
				(previous.app?.personaId ?? 'minako') !== (saved.app?.personaId ?? 'minako');
			let action = runtimeAction(options);
			if (
				action === 'none' &&
				(previous.cometmind?.systemPromptPath ?? '') !==
					(saved.cometmind?.systemPromptPath ?? '')
			) {
				action = 'reload';
			}
			dependencies.shortcuts.refreshGlobalShortcuts();
			dependencies.applicationMenuTray.configureApplicationMenu();
			let reload = null;
			if (action === 'restart') {
				await dependencies.cometMind.stop();
				dependencies.cometMind.start();
				reload = {
					action: 'restart',
					healthy: await dependencies.cometMind.waitForHealth()
				};
			} else if (action === 'reload') {
				reload = await dependencies.cometMind.reload();
			} else if (action === 'gateway') {
				reload = {
					action: 'gateway',
					healthy: await dependencies.cometMind.waitForHealth()
				};
			}
			await dependencies.cometMind.syncDiscordGateway(saved);
			dependencies.windows.applyOpenAtLoginSetting(saved.app?.openAtLogin);
			if (personaIdChanged) dependencies.personas.applyPersona(saved.app?.personaId, saved);
			dependencies.broadcastProviderSettingsChanged(saved);
			return { settings: saved, reload };
		},
		openSettingsWindow: async () => {
			await dependencies.windows.showSettingsWindow();
			return true;
		},
		replayIntro: () =>
			dependencies.windows.triggerMainWindowOnboarding('cometline:replay-intro'),
		runSetupWizard: () =>
			dependencies.windows.triggerMainWindowOnboarding('cometline:run-setup-wizard'),
		getMiniWindowState: () => dependencies.settings.readMiniWindowState(),
		saveMiniWindowState: (_event: IpcMainInvokeEvent, state: unknown) =>
			dependencies.settings.writeMiniWindowState(record(state)),
		getDiscordGatewayStatus: () => ({
			running: dependencies.cometMind.isGatewayRunning(),
			enabled: Boolean(
				dependencies.settings.readProviderSettings().cometmind?.gateway?.discord?.enabled
			)
		}),
		setDiscordGatewayEnabled: async (_event: IpcMainInvokeEvent, enabled: unknown) => {
			const settings = dependencies.settings.readProviderSettings();
			settings.cometmind.gateway.discord.enabled = Boolean(enabled);
			const saved = dependencies.settings.writeProviderSettings(settings);
			await dependencies.cometMind.syncDiscordGateway(saved);
			return {
				running: dependencies.cometMind.isGatewayRunning(),
				enabled: Boolean(saved.cometmind?.gateway?.discord?.enabled)
			};
		},
		loadComposerHistory: () => dependencies.settings.loadComposerHistoryEntries(),
		appendComposerHistory: (_event: IpcMainInvokeEvent, entry: unknown) =>
			dependencies.settings.appendComposerHistoryEntry(entry),
		getOpenAtLogin: () => {
			const settings = dependencies.settings.readProviderSettings();
			try {
				return dependencies.windows.readLoginItemState();
			} catch {
				return { openAtLogin: Boolean(settings.app?.openAtLogin), status: 'unknown' };
			}
		},
		setOpenAtLogin: (_event: IpcMainInvokeEvent, openAtLogin: unknown) => {
			const settings = dependencies.settings.readProviderSettings();
			settings.app = { ...settings.app, openAtLogin: Boolean(openAtLogin) };
			const saved = dependencies.settings.writeProviderSettings(settings);
			const result = dependencies.windows.applyOpenAtLoginSetting(saved.app.openAtLogin);
			return {
				openAtLogin: result.openAtLogin ?? saved.app.openAtLogin,
				status: result.status ?? 'unknown',
				needsApproval: Boolean(result.needsApproval),
				openedSettings: Boolean(result.openedSettings),
				isDev: Boolean(result.isDev),
				message: result.message
			};
		},
		getScreenCaptureAccess: () => {
			const settings = dependencies.settings.readProviderSettings();
			return getScreenCaptureAccess(Boolean(settings.app?.screenCapturePreferred));
		},
		setScreenCapturePreferred: async (_event: IpcMainInvokeEvent, preferred: unknown) => {
			const settings = dependencies.settings.readProviderSettings();
			settings.app = { ...settings.app, screenCapturePreferred: Boolean(preferred) };
			const saved = dependencies.settings.writeProviderSettings(settings);
			return requestScreenCaptureAccess(Boolean(saved.app?.screenCapturePreferred));
		},
		openScreenCaptureSettings: async () => openScreenCaptureSettings(),
		openExternal: async (_event: IpcMainInvokeEvent, rawUrl: unknown) => {
			if (!isExternallyOpenableUrl(rawUrl)) return false;
			await dependencies.shell.openExternal(String(rawUrl));
			return true;
		},
		getAppVersion: () => dependencies.app.getVersion(),
		getUpdateState: () => dependencies.updater.getState(),
		checkForUpdates: () => dependencies.updater.check(),
		installUpdate: () => dependencies.updater.install()
	} satisfies IpcHandlers;

	registerIpcHandlers(handlers);
}
