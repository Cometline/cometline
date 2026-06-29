import { getActiveSessionId } from '$lib/active-session';
import { readHasSeenIntroSync } from '$lib/stores/settings.svelte';

export type WebPanelMode = 'url' | 'file';

export type SessionWebPanel =
	| { mode: 'url'; url: string; visible: boolean }
	| { mode: 'file'; filePath: string; visible: boolean };

export type FocusedPane = 'chat' | 'web';

/**
 * Sentinel key for the web panel state of a not-yet-created session (the home
 * route / new chat). Lets the panel open before a session exists; on first send
 * the draft panel is migrated onto the real session id via `migrateDraftPanel`.
 */
const DRAFT_SESSION_KEY = '__draft__';

function createShellStore() {
	let sidebarOpen = $state(true);
	let settingsOpen = $state(false);
	// Read localStorage synchronously so the very first rendered frame already
	// has the correct value — no IPC round-trip needed. New users (no stored
	// flag) get true; returning users get false. Zero flash either way.
	let introOpen = $state(!readHasSeenIntroSync());
	// Setup wizard: separate from the cinematic intro. A new user who hasn't
	// completed provider configuration sees the wizard after the intro ends.
	let setupOpen = $state(false);
	let composerPhase = $state<'centered' | 'docked'>('centered');
	/** Persisted default workspace (Settings); survives restarts. */
	let defaultWorkspacePath = $state('/');
	/** Active workspace for composer, skills, and @mention context. */
	let workspacePath = $state('/');
	/** Sidebar group ordering; updated on explicit commit (click, send, workspace picker). */
	let sidebarOrderWorkspacePath = $state('/');
	/** When true, Discord gateway sessions are ordered before workspace groups. */
	let sidebarOrderDiscordActive = $state(false);
	let bootMessage = $state('');
	let fullscreen = $state(false);
	let webPanelsBySession = $state<Record<string, SessionWebPanel>>({});
	let focusedPane = $state<FocusedPane>('chat');
	let addressBarFocusRequestId = $state(0);
	let composerFocusRequestId = $state(0);

	function activeSessionId(): string | null {
		return getActiveSessionId();
	}

	/** Resolves the storage key for the active session, or the draft sentinel. */
	function panelSessionKey(): string {
		return activeSessionId() ?? DRAFT_SESSION_KEY;
	}

	function panelForActiveSession(): SessionWebPanel | null {
		return webPanelsBySession[panelSessionKey()] ?? null;
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
		get setupOpen() {
			return setupOpen;
		},
		get composerPhase() {
			return composerPhase;
		},
		get defaultWorkspacePath() {
			return defaultWorkspacePath;
		},
		get workspacePath() {
			return workspacePath;
		},
		get sidebarOrderWorkspacePath() {
			return sidebarOrderWorkspacePath;
		},
		get sidebarOrderDiscordActive() {
			return sidebarOrderDiscordActive;
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
		/**
		 * Storage key for the active session's panel, or the draft sentinel when
		 * no session exists yet. Used to scope webview load tracking so the panel
		 * works on the home route before a session is created.
		 */
		get webPanelSessionKey() {
			return panelSessionKey();
		},
		get addressBarFocusRequestId() {
			return addressBarFocusRequestId;
		},
		get composerFocusRequestId() {
			return composerFocusRequestId;
		},
		/** Update persisted default; sync active when no session is open (home). */
		setDefaultWorkspacePath(path: string) {
			defaultWorkspacePath = path;
			if (!getActiveSessionId()) {
				workspacePath = path;
				sidebarOrderWorkspacePath = path;
				sidebarOrderDiscordActive = false;
			}
		},
		/** Boot: load default from Electron and align active workspace. */
		initializeDefaultWorkspace(path: string) {
			defaultWorkspacePath = path;
			workspacePath = path;
			sidebarOrderWorkspacePath = path;
			sidebarOrderDiscordActive = false;
		},
		setActiveWorkspacePath(path: string) {
			workspacePath = path;
		},
		/** @deprecated Use setActiveWorkspacePath for active-only updates. */
		setWorkspacePath(path: string) {
			workspacePath = path;
		},
		setSidebarOrderWorkspacePath(path: string) {
			sidebarOrderWorkspacePath = path;
		},
		setSidebarOrderDiscordActive(active: boolean) {
			sidebarOrderDiscordActive = active;
		},
		/** Active workspace + sidebar order; does not touch default or Electron. */
		commitActiveWorkspace(path: string) {
			workspacePath = path;
			sidebarOrderWorkspacePath = path;
			sidebarOrderDiscordActive = false;
		},
		resetActiveToDefault() {
			workspacePath = defaultWorkspacePath;
			sidebarOrderWorkspacePath = defaultWorkspacePath;
			sidebarOrderDiscordActive = false;
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
		openSetup() {
			setupOpen = true;
		},
		closeSetup() {
			setupOpen = false;
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
		requestComposerFocus() {
			focusedPane = 'chat';
			composerFocusRequestId += 1;
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
			addressBarFocusRequestId += 1;
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
			const sessionId = panelSessionKey();
			webPanelsBySession = {
				...webPanelsBySession,
				[sessionId]: { mode: 'url', url: '', visible: true }
			};
			focusedPane = 'web';
			syncWebPanelOpen(true);
			addressBarFocusRequestId += 1;
		},
		navigateWebPanel(url: string) {
			const sessionId = panelSessionKey();
			webPanelsBySession = {
				...webPanelsBySession,
				[sessionId]: { mode: 'url', url, visible: true }
			};
			focusedPane = 'web';
			syncWebPanelOpen(true);
			addressBarFocusRequestId += 1;
		},
		requestAddressBarFocus() {
			const sessionId = panelSessionKey();
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
			if (panelForActiveSession()) {
				this.requestAddressBarFocus();
				return;
			}
			this.openWebPanelEmpty();
		},
		toggleWebPanel() {
			const sessionId = panelSessionKey();
			const panel = webPanelsBySession[sessionId];
			if (!panel) return;
			const visible = !panel.visible;
			webPanelsBySession = {
				...webPanelsBySession,
				[sessionId]: { ...panel, visible }
			};
			focusedPane = visible ? 'web' : 'chat';
			syncWebPanelOpen(visible);
			if (visible && panel.mode === 'url') {
				addressBarFocusRequestId += 1;
			}
		},
		closeWebPanel() {
			const sessionId = panelSessionKey();
			if (!webPanelsBySession[sessionId]) return;
			const next = { ...webPanelsBySession };
			delete next[sessionId];
			webPanelsBySession = next;
			this.requestComposerFocus();
			syncWebPanelOpen(false);
		},
		clearWebPanelForSession(sessionId: string) {
			if (!webPanelsBySession[sessionId]) return;
			const next = { ...webPanelsBySession };
			delete next[sessionId];
			webPanelsBySession = next;
			if (activeSessionId() === sessionId) {
				this.requestComposerFocus();
				syncWebPanelOpen(false);
			}
		},
		/**
		 * Opens a workspace file in the panel for the active session, falling back
		 * to the draft sentinel when no session exists yet (home / new chat).
		 */
		openFilePreviewForActive(filePath: string) {
			this.openFilePreview(filePath, panelSessionKey());
		},
		/**
		 * Opens a URL in the panel for the active session, falling back to the
		 * draft sentinel when no session exists yet (home / new chat).
		 */
		openWebPanelForActive(url: string) {
			this.openWebPanel(url, panelSessionKey());
		},
		/**
		 * Moves any draft panel (opened before a session existed) onto the newly
		 * created session id. Called on first send from the home route.
		 */
		migrateDraftPanel(sessionId: string) {
			const draft = webPanelsBySession[DRAFT_SESSION_KEY];
			if (!draft) return;
			const next = { ...webPanelsBySession, [sessionId]: draft };
			delete next[DRAFT_SESSION_KEY];
			webPanelsBySession = next;
		},
		/** Discards a draft panel without migrating it. */
		clearDraftPanel() {
			if (!webPanelsBySession[DRAFT_SESSION_KEY]) return;
			const next = { ...webPanelsBySession };
			delete next[DRAFT_SESSION_KEY];
			webPanelsBySession = next;
		}
	};
}

export const shellStore = createShellStore();
