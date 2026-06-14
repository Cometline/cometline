const { contextBridge, ipcRenderer } = require('electron');

contextBridge.exposeInMainWorld('electronAPI', {
	restartCometMind: () => ipcRenderer.send('cometmind:restart'),
	getWorkspacePath: () => ipcRenderer.invoke('cometline:get-workspace-path'),
	getProviderSettings: () => ipcRenderer.invoke('cometline:get-provider-settings'),
	fetchProviderModels: (config) => ipcRenderer.invoke('cometline:fetch-provider-models', config),
	saveProviderSettings: (settings) =>
		ipcRenderer.invoke('cometline:save-provider-settings', settings)
});
