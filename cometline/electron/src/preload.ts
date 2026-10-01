import { contextBridge, ipcRenderer } from 'electron';
import type { ElectronAPI } from './shared/api.js';
import {
	EVENT_CHANNELS,
	INVOKE_CHANNELS,
	SEND_CHANNELS,
	type EventMethod,
	type InvokeMethod,
	type InvokeRequest,
	type InvokeResponse,
	type SendMethod,
	type SendPayload
} from './shared/ipc-channels.js';

const invoke = <K extends InvokeMethod>(
	method: K,
	...args: InvokeRequest<K>
): Promise<InvokeResponse<K>> => ipcRenderer.invoke(INVOKE_CHANNELS[method], ...args);

const send = <K extends SendMethod>(method: K, ...args: SendPayload<K>) =>
	ipcRenderer.send(SEND_CHANNELS[method], ...args);

const subscribe = <T>(method: EventMethod, callback: (payload: T) => void) => {
	const channel = EVENT_CHANNELS[method];
	const handler = (_event: Electron.IpcRendererEvent, payload: T) => callback(payload);
	ipcRenderer.on(channel, handler);
	return () => ipcRenderer.removeListener(channel, handler);
};

const subscribeSignal = (method: EventMethod, callback: () => void) => {
	const channel = EVENT_CHANNELS[method];
	const handler = () => callback();
	ipcRenderer.on(channel, handler);
	return () => ipcRenderer.removeListener(channel, handler);
};

const electronAPI: ElectronAPI = {
	restartCometMind: () => send('restartCometMind'),
	openExternal: (url) => invoke('openExternal', url),
	copyMediaFile: (sessionId, mediaId) => invoke('copyMediaFile', sessionId, mediaId),
	getWorkspacePath: () => invoke('getWorkspacePath'),
	selectWorkspacePath: () => invoke('selectWorkspacePath'),
	browseWorkspacePath: () => invoke('browseWorkspacePath'),
	selectBackupFolder: () => invoke('selectBackupFolder'),
	setWorkspacePath: (workspacePath) => invoke('setWorkspacePath', workspacePath),
	watchWorkspace: (workspacePath) => invoke('watchWorkspace', workspacePath),
	listRecentWorkspaces: () => invoke('listRecentWorkspaces'),
	removeRecentWorkspacePath: (workspacePath) =>
		invoke('removeRecentWorkspacePath', workspacePath),
	filterExistingWorkspacePaths: (paths) => invoke('filterExistingWorkspacePaths', paths),
	pruneWorkspaceStore: () => invoke('pruneWorkspaceStore'),
	createPdfPreview: (request) => invoke('createPdfPreview', request),
	revokePdfPreview: (token) => invoke('revokePdfPreview', token),
	listTerminals: () => invoke('listTerminals'),
	createTerminal: (payload) => invoke('createTerminal', payload),
	writeTerminal: (payload) => invoke('writeTerminal', payload),
	resizeTerminal: (payload) => invoke('resizeTerminal', payload),
	terminateTerminal: (sessionId) => invoke('terminateTerminal', sessionId),
	removeTerminal: (sessionId) => invoke('removeTerminal', sessionId),
	onTerminalData: (callback) => subscribe('onTerminalData', callback),
	onTerminalExit: (callback) => subscribe('onTerminalExit', callback),
	listCustomPersonas: () => invoke('listCustomPersonas'),
	saveCustomPersona: (payload) => invoke('saveCustomPersona', payload),
	deleteCustomPersona: (id) => invoke('deleteCustomPersona', id),
	readPersonaAvatar: (id) => invoke('readPersonaAvatar', id),
	readBuiltinSoul: (personaId) => invoke('readBuiltinSoul', personaId),
	getProviderSettings: () => invoke('getProviderSettings'),
	getCodexAuthStatus: () => invoke('getCodexAuthStatus'),
	startCodexLogin: () => invoke('startCodexLogin'),
	getXaiAuthStatus: () => invoke('getXaiAuthStatus'),
	startXaiLogin: () => invoke('startXaiLogin'),
	readCursorMcpConfig: () => invoke('readCursorMcpConfig'),
	getDiscordGatewayStatus: () => invoke('getDiscordGatewayStatus'),
	setDiscordGatewayEnabled: (enabled) => invoke('setDiscordGatewayEnabled', enabled),
	getOpenAtLogin: () => invoke('getOpenAtLogin'),
	setOpenAtLogin: (enabled) => invoke('setOpenAtLogin', enabled),
	getScreenCaptureAccess: () => invoke('getScreenCaptureAccess'),
	setScreenCapturePreferred: (enabled) => invoke('setScreenCapturePreferred', enabled),
	openScreenCaptureSettings: () => invoke('openScreenCaptureSettings'),
	openSessionInMainWindow: (sessionId) => invoke('openSessionInMainWindow', sessionId),
	openSettingsWindow: () => invoke('openSettingsWindow'),
	replayIntroInMainWindow: () => invoke('replayIntroInMainWindow'),
	runSetupWizardInMainWindow: () => invoke('runSetupWizardInMainWindow'),
	onMiniWindowActivated: (callback) => subscribeSignal('onMiniWindowActivated', callback),
	getMiniWindowState: () => invoke('getMiniWindowState'),
	saveMiniWindowState: (state) => invoke('saveMiniWindowState', state),
	fetchProviderModels: (config) => invoke('fetchProviderModels', config),
	checkOllamaHealth: (baseURL) => invoke('checkOllamaHealth', baseURL),
	listOllamaModels: (baseURL) => invoke('listOllamaModels', baseURL),
	pullOllamaModel: (payload) => invoke('pullOllamaModel', payload),
	cancelOllamaPull: () => invoke('cancelOllamaPull'),
	onOllamaPullProgress: (callback) => subscribe('onOllamaPullProgress', callback),
	saveProviderSettings: (settings, options) => invoke('saveProviderSettings', settings, options),
	setSidebarOpen: (payload) => send('setSidebarOpen', payload),
	getFullScreen: () => invoke('getFullScreen'),
	onFullScreenChange: (callback) =>
		subscribe('onFullScreenChange', (isFullScreen) => callback(Boolean(isFullScreen))),
	onWorkspaceChanged: (callback) => subscribe('onWorkspaceChanged', callback),
	getAppVersion: () => invoke('getAppVersion'),
	getUpdateState: () => invoke('getUpdateState'),
	checkForUpdates: () => invoke('checkForUpdates'),
	installUpdate: () => invoke('installUpdate'),
	onUpdateState: (callback) => subscribe('onUpdateState', callback),
	setShortcutCaptureActive: (active) => send('setShortcutCaptureActive', Boolean(active)),
	setSessionNavigationSuspended: (suspended) =>
		send('setSessionNavigationSuspended', Boolean(suspended)),
	setWorkspacePanelOpen: (open) => send('setWorkspacePanelOpen', Boolean(open)),
	setInboxOpen: (open) => send('setInboxOpen', Boolean(open)),
	confirmCloseWindow: () => send('confirmCloseWindow'),
	onCloseWorkspacePanel: (callback) => subscribeSignal('onCloseWorkspacePanel', callback),
	onCloseInbox: (callback) => subscribeSignal('onCloseInbox', callback),
	onRequestCloseWindow: (callback) => subscribeSignal('onRequestCloseWindow', callback),
	onRequestReload: (callback) => subscribeSignal('onRequestReload', callback),
	onNavigateSession: (callback) =>
		subscribe('onNavigateSession', (direction) => {
			if (direction === 'prev' || direction === 'next') callback(direction);
		}),
	onShortcutAction: (callback) =>
		subscribe('onShortcutAction', (action) => {
			if (typeof action === 'string') callback(action as Parameters<typeof callback>[0]);
		}),
	onProviderSettingsChanged: (callback) => subscribe('onProviderSettingsChanged', callback),
	onPersonaAvatarChanged: (callback) => subscribe('onPersonaAvatarChanged', callback),
	onReplayIntro: (callback) => subscribeSignal('onReplayIntro', callback),
	onRunSetupWizard: (callback) => subscribeSignal('onRunSetupWizard', callback),
	notifyJob: (payload) => send('notifyJob', payload),
	loadComposerHistory: () => invoke('loadComposerHistory'),
	appendComposerHistory: (entry) => invoke('appendComposerHistory', entry)
};

contextBridge.exposeInMainWorld('electronAPI', electronAPI);
