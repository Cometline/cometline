import { getActiveSessionId } from '$lib/active-session';

export type WebPanelMode = 'url' | 'file';

export type SessionWebPanel =
	| { mode: 'url'; url: string; visible: boolean }
	| { mode: 'file'; filePath: string; visible: boolean };

export type FocusedPane = 'chat' | 'web';

function createShellStore() {
	let sidebarOpen = $state(true);
	let settingsOpen = $state(false);
	let introOpen = $state(false);
	let composerPhase = $state<'centered' | 'docked'>('centered');
	let workspacePath = $state('/');
	let bootMessage = $state('');
	let fullscreen = $state(false);
	let webPanelsBySession = $state<Record<string, SessionWebPanel>>({});
	let focusedPane = $state<FocusedPane>('chat');
	let addressBarFocusRequestId = $state(0);

	function activeSessionId(): string | null {
		return getActiveSessionId();
	}

	function panelForActiveSession(): SessionWebPanel | null {
		const id = activeSessionId();
		if (!id) return null;
		return webPanelsBySession[id] ?? null;
	}

	function syncWebPanelOpen(open: boolean) {
		window.electronAPI?.setWebPanelOpen?.(open);
	}

	function syncWebPanelOpenForActiveSession() {
		const panel = panelForActiveSession();
		syncWebPanelOpen(Boolean(panel?.visible));
	}

	return {
		get sidebarOpen() {
			return sidebarOpen;
		},
		get fullscreen() {
			return fullscreen;
		},
		get settingsOpen() {
			return settingsOpen;
		},
		get introOpen() {
			return introOpen;
		},
		get composerPhase() {
			return composerPhase;
		},
		get workspacePath() {
			return workspacePath;
		},
		get bootMessage() {
			return bootMessage;
		},
		get focusedPane() {
			return focusedPane;
		},
		get webPanelOpen() {
			const panel = panelForActiveSession();
			return Boolean(panel?.visible);
		},
		get webPanelMode(): WebPanelMode | null {
			return panelForActiveSession()?.mode ?? null;
		},
		get webPanelUrl() {
			const panel = panelForActiveSession();
			return panel?.mode === 'url' ? panel.url : null;
		},
		get webPanelFilePath() {
			const panel = panelForActiveSession();
			return panel?.mode === 'file' ? panel.filePath : null;
		},
		get hasWebPanelForSession() {
			return panelForActiveSession() !== null;
		},
		get addressBarFocusRequestId() {
			return addressBarFocusRequestId;
		},
		setWorkspacePath(path: string) {
			workspacePath = path;
		},
		setBootMessage(message: string) {
			bootMessage = message;
		},
		setFullscreen(value: boolean) {
			fullscreen = value;
		},
		toggleSidebar() {
			sidebarOpen = !sidebarOpen;
		},
		openSidebar() {
			sidebarOpen = true;
		},
		closeSidebar() {
			sidebarOpen = false;
		},
		openSettings() {
			settingsOpen = true;
		},
		closeSettings() {
			settingsOpen = false;
		},
		openIntro() {
			introOpen = true;
		},
		closeIntro() {
			introOpen = false;
		},
		dockComposer() {
			composerPhase = 'docked';
		},
		centerComposer() {
			composerPhase = 'centered';
		},
		setFocusedPane(pane: FocusedPane) {
			focusedPane = pane;
		},
		onActiveSessionChange() {
			focusedPane = 'chat';
			syncWebPanelOpenForActiveSession();
		},
		openWebPanel(url: string, sessionId: string) {
			webPanelsBySession = {
				...webPanelsBySession,
				[sessionId]: { mode: 'url', url, visible: true }
			};
			focusedPane = 'web';
			syncWebPanelOpen(true);
		},
		openFilePreview(filePath: string, sessionId: string) {
			webPanelsBySession = {
				...webPanelsBySession,
				[sessionId]: { mode: 'file', filePath, visible: true }
			};
			focusedPane = 'web';
			syncWebPanelOpen(true);
		},
		openWebPanelEmpty() {
			const sessionId = activeSessionId();
			if (!sessionId) return;
			webPanelsBySession = {
				...webPanelsBySession,
				[sessionId]: { mode: 'url', url: '', visible: true }
			};
			focusedPane = 'web';
			syncWebPanelOpen(true);
			addressBarFocusRequestId += 1;
		},
		navigateWebPanel(url: string) {
			const sessionId = activeSessionId();
			if (!sessionId) return;
			webPanelsBySession = {
				...webPanelsBySession,
				[sessionId]: { mode: 'url', url, visible: true }
			};
			focusedPane = 'web';
			syncWebPanelOpen(true);
		},
		requestAddressBarFocus() {
			const sessionId = activeSessionId();
			if (!sessionId) return;
			const panel = webPanelsBySession[sessionId];
			if (!panel) return;
			if (!panel.visible) {
				webPanelsBySession = {
					...webPanelsBySession,
					[sessionId]: { ...panel, visible: true }
				};
				syncWebPanelOpen(true);
				focusedPane = 'web';
			}
			addressBarFocusRequestId += 1;
		},
		openWebPanelFromShortcut() {
			const sessionId = activeSessionId();
			if (!sessionId) return;
			if (panelForActiveSession()) {
				this.requestAddressBarFocus();
				return;
			}
			this.openWebPanelEmpty();
		},
		toggleWebPanel() {
			const sessionId = activeSessionId();
			if (!sessionId) return;
			const panel = webPanelsBySession[sessionId];
			if (!panel) return;
			const visible = !panel.visible;
			webPanelsBySession = {
				...webPanelsBySession,
				[sessionId]: { ...panel, visible }
			};
			focusedPane = visible ? 'web' : 'chat';
			syncWebPanelOpen(visible);
		},
		closeWebPanel() {
			const sessionId = activeSessionId();
			if (!sessionId || !webPanelsBySession[sessionId]) return;
			const next = { ...webPanelsBySession };
			delete next[sessionId];
			webPanelsBySession = next;
			focusedPane = 'chat';
			syncWebPanelOpen(false);
		},
		clearWebPanelForSession(sessionId: string) {
			if (!webPanelsBySession[sessionId]) return;
			const next = { ...webPanelsBySession };
			delete next[sessionId];
			webPanelsBySession = next;
			if (activeSessionId() === sessionId) {
				focusedPane = 'chat';
				syncWebPanelOpen(false);
			}
		}
	};
}

export const shellStore = createShellStore();
