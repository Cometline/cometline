const { contextBridge, ipcRenderer } = require('electron');

contextBridge.exposeInMainWorld('electronAPI', {
	restartCometMind: () => ipcRenderer.send('cometmind:restart'),
	openExternal: (url) => ipcRenderer.invoke('cometline:open-external', url),
	getWorkspacePath: () => ipcRenderer.invoke('cometline:get-workspace-path'),
	selectWorkspacePath: () => ipcRenderer.invoke('cometline:select-workspace-path'),
	setWorkspacePath: (workspacePath) => ipcRenderer.invoke('cometline:set-workspace-path', workspacePath),
	getProviderSettings: () => ipcRenderer.invoke('cometline:get-provider-settings'),
	fetchProviderModels: (config) => ipcRenderer.invoke('cometline:fetch-provider-models', config),
	saveProviderSettings: (settings, options) =>
		ipcRenderer.invoke('cometline:save-provider-settings', settings, options),
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
	onNavigateSession: (callback) => {
		const handler = (_event, direction) => {
			if (direction === 'prev' || direction === 'next') callback(direction);
		};
		ipcRenderer.on('cometline:navigate-session', handler);
		return () => ipcRenderer.removeListener('cometline:navigate-session', handler);
	}
});
