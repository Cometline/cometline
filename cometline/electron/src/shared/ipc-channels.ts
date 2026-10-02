import type { ElectronAPI } from './api.js';

/**
 * Single registry of Electron IPC channel names, keyed by the `ElectronAPI`
 * method that uses them. Preload, main-process handler registration, and
 * main-process event senders all read channel strings from here.
 */

/** Renderer → main request/response (`ipcRenderer.invoke` ↔ `ipcMain.handle`). */
export const INVOKE_CHANNELS = {
	openExternal: 'cometline:open-external',
	copyMediaFile: 'cometline:copy-media-file',
	getWorkspacePath: 'cometline:get-workspace-path',
	selectWorkspacePath: 'cometline:select-workspace-path',
	browseWorkspacePath: 'cometline:browse-workspace-path',
	selectBackupFolder: 'cometline:select-backup-folder',
	setWorkspacePath: 'cometline:set-workspace-path',
	watchWorkspace: 'cometline:watch-workspace',
	listRecentWorkspaces: 'cometline:list-recent-workspaces',
	removeRecentWorkspacePath: 'cometline:remove-recent-workspace-path',
	filterExistingWorkspacePaths: 'cometline:filter-existing-workspace-paths',
	pruneWorkspaceStore: 'cometline:prune-workspace-store',
	createPdfPreview: 'cometline:create-pdf-preview',
	revokePdfPreview: 'cometline:revoke-pdf-preview',
	listTerminals: 'cometline:terminal-list',
	createTerminal: 'cometline:terminal-create',
	writeTerminal: 'cometline:terminal-write',
	resizeTerminal: 'cometline:terminal-resize',
	terminateTerminal: 'cometline:terminal-terminate',
	removeTerminal: 'cometline:terminal-remove',
	listCustomPersonas: 'cometline:list-custom-personas',
	saveCustomPersona: 'cometline:save-custom-persona',
	deleteCustomPersona: 'cometline:delete-custom-persona',
	readPersonaAvatar: 'cometline:read-persona-avatar',
	readBuiltinSoul: 'cometline:read-builtin-soul',
	getProviderSettings: 'cometline:get-provider-settings',
	saveProviderSettings: 'cometline:save-provider-settings',
	getCodexAuthStatus: 'cometline:get-codex-auth-status',
	startCodexLogin: 'cometline:start-codex-login',
	getXaiAuthStatus: 'cometline:get-xai-auth-status',
	startXaiLogin: 'cometline:start-xai-login',
	readCursorMcpConfig: 'cometline:read-cursor-mcp-config',
	getDiscordGatewayStatus: 'cometline:get-discord-gateway-status',
	setDiscordGatewayEnabled: 'cometline:set-discord-gateway-enabled',
	getOpenAtLogin: 'cometline:get-open-at-login',
	setOpenAtLogin: 'cometline:set-open-at-login',
	getScreenCaptureAccess: 'cometline:get-screen-capture-access',
	setScreenCapturePreferred: 'cometline:set-screen-capture-preferred',
	openScreenCaptureSettings: 'cometline:open-screen-capture-settings',
	openSessionInMainWindow: 'cometline:open-session-in-main-window',
	openSettingsWindow: 'cometline:open-settings-window',
	replayIntroInMainWindow: 'cometline:replay-intro',
	runSetupWizardInMainWindow: 'cometline:run-setup-wizard',
	getMiniWindowState: 'cometline:get-mini-window-state',
	saveMiniWindowState: 'cometline:save-mini-window-state',
	fetchProviderModels: 'cometline:fetch-provider-models',
	checkOllamaHealth: 'cometline:ollama-health',
	listOllamaModels: 'cometline:ollama-models',
	pullOllamaModel: 'cometline:ollama-pull',
	cancelOllamaPull: 'cometline:ollama-cancel-pull',
	getFullScreen: 'cometline:get-fullscreen',
	getAppVersion: 'cometline:get-app-version',
	getUpdateState: 'cometline:get-update-state',
	checkForUpdates: 'cometline:check-for-updates',
	installUpdate: 'cometline:install-update',
	loadComposerHistory: 'cometline:load-composer-history',
	appendComposerHistory: 'cometline:append-composer-history'
} as const satisfies Partial<Record<keyof ElectronAPI, string>>;

/** Renderer → main fire-and-forget (`ipcRenderer.send` ↔ `ipcMain.on`). */
export const SEND_CHANNELS = {
	restartCometMind: 'cometmind:restart',
	setSidebarOpen: 'cometline:set-sidebar-open',
	setShortcutCaptureActive: 'cometline:shortcut-capture-active',
	setSessionNavigationSuspended: 'cometline:session-navigation-suspended',
	setWorkspacePanelOpen: 'cometline:workspace-panel-open',
	setInboxOpen: 'cometline:inbox-open',
	confirmCloseWindow: 'cometline:confirm-close-window',
	notifyJob: 'jobs:notify'
} as const satisfies Partial<Record<keyof ElectronAPI, string>>;

/** Main → renderer push (`webContents.send` ↔ `ipcRenderer.on`). */
export const EVENT_CHANNELS = {
	onTerminalData: 'cometline:terminal-data',
	onTerminalExit: 'cometline:terminal-exit',
	onMiniWindowActivated: 'cometline:activate-mini-window',
	onOllamaPullProgress: 'cometline:ollama-pull-progress',
	onFullScreenChange: 'cometline:fullscreen-changed',
	onWorkspaceChanged: 'cometline:workspace-changed',
	onUpdateState: 'cometline:update-state',
	onCloseWorkspacePanel: 'cometline:close-workspace-panel',
	onCloseInbox: 'cometline:close-inbox',
	onRequestCloseWindow: 'cometline:request-close-window',
	onRequestReload: 'cometline:request-reload',
	onNavigateSession: 'cometline:navigate-session',
	onShortcutAction: 'cometline:shortcut-action',
	onCommandEnter: 'cometline:command-enter',
	onProviderSettingsChanged: 'cometline:provider-settings-changed',
	onPersonaAvatarChanged: 'cometline:persona-avatar-changed',
	onReplayIntro: 'cometline:replay-intro',
	onRunSetupWizard: 'cometline:run-setup-wizard'
} as const satisfies Partial<Record<keyof ElectronAPI, string>>;

export type InvokeMethod = keyof typeof INVOKE_CHANNELS;
export type SendMethod = keyof typeof SEND_CHANNELS;
export type EventMethod = keyof typeof EVENT_CHANNELS;
export type EventChannel = (typeof EVENT_CHANNELS)[EventMethod];

export type InvokeRequest<K extends InvokeMethod> = Parameters<ElectronAPI[K]>;
export type InvokeResponse<K extends InvokeMethod> = Awaited<ReturnType<ElectronAPI[K]>>;
export type SendPayload<K extends SendMethod> = Parameters<ElectronAPI[K]>;
