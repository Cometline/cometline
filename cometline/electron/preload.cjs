const { contextBridge, ipcRenderer } = require('electron');

contextBridge.exposeInMainWorld('electronAPI', {
	restartCometMind: () => ipcRenderer.send('cometmind:restart'),
	openExternal: (url) => ipcRenderer.invoke('cometline:open-external', url),
	getWorkspacePath: () => ipcRenderer.invoke('cometline:get-workspace-path'),
	selectWorkspacePath: () => ipcRenderer.invoke('cometline:select-workspace-path'),
	setWorkspacePath: (workspacePath) => ipcRenderer.invoke('cometline:set-workspace-path', workspacePath),
	listRecentWorkspaces: () => ipcRenderer.invoke('cometline:list-recent-workspaces'),
	filterExistingWorkspacePaths: (paths) =>
		ipcRenderer.invoke('cometline:filter-existing-workspace-paths', paths),
	pruneWorkspaceStore: () => ipcRenderer.invoke('cometline:prune-workspace-store'),
	readWorkspaceFile: (workspacePath, relativePath) =>
		ipcRenderer.invoke('cometline:read-workspace-file', workspacePath, relativePath),
	getProviderSettings: () => ipcRenderer.invoke('cometline:get-provider-settings'),
	getCodexAuthStatus: () => ipcRenderer.invoke('cometline:get-codex-auth-status'),
	startCodexLogin: () => ipcRenderer.invoke('cometline:start-codex-login'),
	getDiscordGatewayStatus: () => ipcRenderer.invoke('cometline:get-discord-gateway-status'),
	setDiscordGatewayEnabled: (enabled) =>
		ipcRenderer.invoke('cometline:set-discord-gateway-enabled', enabled),
	getOpenAtLogin: () => ipcRenderer.invoke('cometline:get-open-at-login'),
	setOpenAtLogin: (enabled) => ipcRenderer.invoke('cometline:set-open-at-login', enabled),
	fetchProviderModels: (config) => ipcRenderer.invoke('cometline:fetch-provider-models', config),
	saveProviderSettings: (settings, options) =>
		ipcRenderer.invoke('cometline:save-provider-settings', settings, options),
	exportProviderSettings: () => ipcRenderer.invoke('cometline:export-provider-settings'),
	importProviderSettings: () => ipcRenderer.invoke('cometline:import-provider-settings'),
	setSidebarOpen: (payload) => ipcRenderer.send('cometline:set-sidebar-open', payload),
	getFullScreen: () => ipcRenderer.invoke('cometline:get-fullscreen'),
	onFullScreenChange: (callback) => {
		const handler = (_event, isFullScreen) => callback(Boolean(isFullScreen));
		ipcRenderer.on('cometline:fullscreen-changed', handler);
		return () => ipcRenderer.removeListener('cometline:fullscreen-changed', handler);
	},
	getAppVersion: () => ipcRenderer.invoke('cometline:get-app-version'),
	getUpdateState: () => ipcRenderer.invoke('cometline:get-update-state'),
	checkForUpdates: () => ipcRenderer.invoke('cometline:check-for-updates'),
	installUpdate: () => ipcRenderer.invoke('cometline:install-update'),
	onUpdateState: (callback) => {
		const handler = (_event, state) => callback(state);
		ipcRenderer.on('cometline:update-state', handler);
		return () => ipcRenderer.removeListener('cometline:update-state', handler);
	},
	setShortcutCaptureActive: (active) =>
		ipcRenderer.send('cometline:shortcut-capture-active', Boolean(active)),
	setSessionNavigationSuspended: (suspended) =>
		ipcRenderer.send('cometline:session-navigation-suspended', Boolean(suspended)),
	setWebPanelOpen: (open) => ipcRenderer.send('cometline:web-panel-open', Boolean(open)),
	onCloseWebPanel: (callback) => {
		const handler = () => callback();
		ipcRenderer.on('cometline:close-web-panel', handler);
		return () => ipcRenderer.removeListener('cometline:close-web-panel', handler);
	},
	onToggleWebPanel: (callback) => {
		const handler = () => callback();
		ipcRenderer.on('cometline:toggle-web-panel', handler);
		return () => ipcRenderer.removeListener('cometline:toggle-web-panel', handler);
	},
	onOpenWebPanel: (callback) => {
		const handler = () => callback();
		ipcRenderer.on('cometline:open-web-panel', handler);
		return () => ipcRenderer.removeListener('cometline:open-web-panel', handler);
	},
	onNavigateSession: (callback) => {
		const handler = (_event, direction) => {
			if (direction === 'prev' || direction === 'next') callback(direction);
		};
		ipcRenderer.on('cometline:navigate-session', handler);
		return () => ipcRenderer.removeListener('cometline:navigate-session', handler);
	}
});
