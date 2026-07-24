import { contextBridge, ipcRenderer } from 'electron';
import type { ElectronAPI } from './shared/api.js';

const subscribe = <T>(channel: string, callback: (payload: T) => void) => {
	const handler = (_event: Electron.IpcRendererEvent, payload: T) => callback(payload);
	ipcRenderer.on(channel, handler);
	return () => ipcRenderer.removeListener(channel, handler);
};

const subscribeSignal = (channel: string, callback: () => void) => {
	const handler = () => callback();
	ipcRenderer.on(channel, handler);
	return () => ipcRenderer.removeListener(channel, handler);
};

const electronAPI: ElectronAPI = {
	restartCometMind: () => ipcRenderer.send('cometmind:restart'),
	openExternal: (url) => ipcRenderer.invoke('cometline:open-external', url),
	getWorkspacePath: () => ipcRenderer.invoke('cometline:get-workspace-path'),
	selectWorkspacePath: () => ipcRenderer.invoke('cometline:select-workspace-path'),
	selectBackupFolder: () => ipcRenderer.invoke('cometline:select-backup-folder'),
	setWorkspacePath: (workspacePath) =>
		ipcRenderer.invoke('cometline:set-workspace-path', workspacePath),
	listRecentWorkspaces: () => ipcRenderer.invoke('cometline:list-recent-workspaces'),
	removeRecentWorkspacePath: (workspacePath) =>
		ipcRenderer.invoke('cometline:remove-recent-workspace-path', workspacePath),
	filterExistingWorkspacePaths: (paths) =>
		ipcRenderer.invoke('cometline:filter-existing-workspace-paths', paths),
	pruneWorkspaceStore: () => ipcRenderer.invoke('cometline:prune-workspace-store'),
	readWorkspaceFile: (workspacePath, relativePath) =>
		ipcRenderer.invoke('cometline:read-workspace-file', workspacePath, relativePath),
	listTerminals: () => ipcRenderer.invoke('cometline:terminal-list'),
	createTerminal: (payload) => ipcRenderer.invoke('cometline:terminal-create', payload),
	writeTerminal: (payload) => ipcRenderer.invoke('cometline:terminal-write', payload),
	resizeTerminal: (payload) => ipcRenderer.invoke('cometline:terminal-resize', payload),
	terminateTerminal: (sessionId) => ipcRenderer.invoke('cometline:terminal-terminate', sessionId),
	removeTerminal: (sessionId) => ipcRenderer.invoke('cometline:terminal-remove', sessionId),
	onTerminalData: (callback) => subscribe('cometline:terminal-data', callback),
	onTerminalExit: (callback) => subscribe('cometline:terminal-exit', callback),
	listCustomPersonas: () => ipcRenderer.invoke('cometline:list-custom-personas'),
	saveCustomPersona: (payload) => ipcRenderer.invoke('cometline:save-custom-persona', payload),
	deleteCustomPersona: (id) => ipcRenderer.invoke('cometline:delete-custom-persona', id),
	readPersonaAvatar: (id) => ipcRenderer.invoke('cometline:read-persona-avatar', id),
	readBuiltinSoul: (personaId) => ipcRenderer.invoke('cometline:read-builtin-soul', personaId),
	getProviderSettings: () => ipcRenderer.invoke('cometline:get-provider-settings'),
	getCodexAuthStatus: () => ipcRenderer.invoke('cometline:get-codex-auth-status'),
	startCodexLogin: () => ipcRenderer.invoke('cometline:start-codex-login'),
	getXaiAuthStatus: () => ipcRenderer.invoke('cometline:get-xai-auth-status'),
	startXaiLogin: () => ipcRenderer.invoke('cometline:start-xai-login'),
	getMcpOAuthStatus: (serverId) => ipcRenderer.invoke('cometline:get-mcp-oauth-status', serverId),
	startMcpOAuth: (payload) => ipcRenderer.invoke('cometline:start-mcp-oauth', payload),
	readCursorMcpConfig: () => ipcRenderer.invoke('cometline:read-cursor-mcp-config'),
	getDiscordGatewayStatus: () => ipcRenderer.invoke('cometline:get-discord-gateway-status'),
	setDiscordGatewayEnabled: (enabled) =>
		ipcRenderer.invoke('cometline:set-discord-gateway-enabled', enabled),
	getOpenAtLogin: () => ipcRenderer.invoke('cometline:get-open-at-login'),
	setOpenAtLogin: (enabled) => ipcRenderer.invoke('cometline:set-open-at-login', enabled),
	getScreenCaptureAccess: () => ipcRenderer.invoke('cometline:get-screen-capture-access'),
	setScreenCapturePreferred: (enabled) =>
		ipcRenderer.invoke('cometline:set-screen-capture-preferred', enabled),
	openScreenCaptureSettings: () => ipcRenderer.invoke('cometline:open-screen-capture-settings'),
	openSessionInMainWindow: (sessionId) =>
		ipcRenderer.invoke('cometline:open-session-in-main-window', sessionId),
	openSettingsWindow: () => ipcRenderer.invoke('cometline:open-settings-window'),
	replayIntroInMainWindow: () => ipcRenderer.invoke('cometline:replay-intro'),
	runSetupWizardInMainWindow: () => ipcRenderer.invoke('cometline:run-setup-wizard'),
	getMiniWindowState: () => ipcRenderer.invoke('cometline:get-mini-window-state'),
	saveMiniWindowState: (state) => ipcRenderer.invoke('cometline:save-mini-window-state', state),
	fetchProviderModels: (config) => ipcRenderer.invoke('cometline:fetch-provider-models', config),
	checkOllamaHealth: (baseURL) => ipcRenderer.invoke('cometline:ollama-health', baseURL),
	listOllamaModels: (baseURL) => ipcRenderer.invoke('cometline:ollama-models', baseURL),
	getOllamaDiagnostics: (baseURL) => ipcRenderer.invoke('cometline:ollama-diagnostics', baseURL),
	pullOllamaModel: (payload) => ipcRenderer.invoke('cometline:ollama-pull', payload),
	cancelOllamaPull: () => ipcRenderer.invoke('cometline:ollama-cancel-pull'),
	onOllamaPullProgress: (callback) => subscribe('cometline:ollama-pull-progress', callback),
	saveProviderSettings: (settings, options) =>
		ipcRenderer.invoke('cometline:save-provider-settings', settings, options),
	setSidebarOpen: (payload) => ipcRenderer.send('cometline:set-sidebar-open', payload),
	getFullScreen: () => ipcRenderer.invoke('cometline:get-fullscreen'),
	onFullScreenChange: (callback) =>
		subscribe('cometline:fullscreen-changed', (isFullScreen) =>
			callback(Boolean(isFullScreen))
		),
	getAppVersion: () => ipcRenderer.invoke('cometline:get-app-version'),
	getUpdateState: () => ipcRenderer.invoke('cometline:get-update-state'),
	checkForUpdates: () => ipcRenderer.invoke('cometline:check-for-updates'),
	installUpdate: () => ipcRenderer.invoke('cometline:install-update'),
	onUpdateState: (callback) => subscribe('cometline:update-state', callback),
	setShortcutCaptureActive: (active) =>
		ipcRenderer.send('cometline:shortcut-capture-active', Boolean(active)),
	setSessionNavigationSuspended: (suspended) =>
		ipcRenderer.send('cometline:session-navigation-suspended', Boolean(suspended)),
	setWorkspacePanelOpen: (open) => ipcRenderer.send('cometline:workspace-panel-open', Boolean(open)),
	setInboxOpen: (open) => ipcRenderer.send('cometline:inbox-open', Boolean(open)),
	confirmCloseWindow: () => ipcRenderer.send('cometline:confirm-close-window'),
	onCloseWorkspacePanel: (callback) => subscribeSignal('cometline:close-workspace-panel', callback),
	onCloseInbox: (callback) => subscribeSignal('cometline:close-inbox', callback),
	onRequestCloseWindow: (callback) => subscribeSignal('cometline:request-close-window', callback),
	onRequestReload: (callback) => subscribeSignal('cometline:request-reload', callback),
	onToggleWorkspacePanel: (callback) => subscribeSignal('cometline:toggle-workspace-panel', callback),
	onOpenWebSearch: (callback) => subscribeSignal('cometline:open-web-search', callback),
	onNavigateSession: (callback) =>
		subscribe('cometline:navigate-session', (direction) => {
			if (direction === 'prev' || direction === 'next') callback(direction);
		}),
	onShortcutAction: (callback) =>
		subscribe('cometline:shortcut-action', (action) => {
			if (typeof action === 'string') callback(action as Parameters<typeof callback>[0]);
		}),
	onProviderSettingsChanged: (callback) =>
		subscribe('cometline:provider-settings-changed', callback),
	onPersonaAvatarChanged: (callback) => subscribe('cometline:persona-avatar-changed', callback),
	onReplayIntro: (callback) => subscribeSignal('cometline:replay-intro', callback),
	onRunSetupWizard: (callback) => subscribeSignal('cometline:run-setup-wizard', callback),
	notifyJob: (payload) => ipcRenderer.send('jobs:notify', payload),
	loadComposerHistory: () => ipcRenderer.invoke('cometline:load-composer-history'),
	appendComposerHistory: (entry) => ipcRenderer.invoke('cometline:append-composer-history', entry)
};

contextBridge.exposeInMainWorld('electronAPI', electronAPI);
