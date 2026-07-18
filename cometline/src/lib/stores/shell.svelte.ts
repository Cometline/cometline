import { getActiveSessionId } from '$lib/active-session';
import { readHasSeenIntroSync } from '$lib/stores/settings.svelte';
import type { WebContext } from '$lib/actions/start-chat';
import {
	canGoBack as historyCanGoBack,
	canGoForward as historyCanGoForward,
	createPanelHistoryState,
	currentEntry,
	goBack as historyGoBack,
	goForward as historyGoForward,
	pushEntry,
	type PanelHistoryEntry,
	type PanelHistoryState
} from '$lib/workspace/panel-history';

export type WebPanelMode = 'url' | 'file';

export type SessionWebPanel =
	| { mode: 'url'; url: string; visible: boolean }
	| { mode: 'file'; filePath: string; visible: boolean };

export type FocusedPane = 'chat' | 'web';

/** A page selected for the next turn whose body has not been read yet. */
export type PendingPageContext = {
	kind: 'page';
	title?: string;
	source: string;
	lazy: true;
};

/** Path-only “currently viewing” file reference (no body attached). */
export type PendingViewingFileContext = WebContext & {
	kind: 'file';
	role: 'viewing';
	content: '';
};

export type PendingWebContext = WebContext | PendingPageContext | PendingViewingFileContext;

function isPendingPageContext(context: PendingWebContext): context is PendingPageContext {
	return context.kind === 'page' && 'lazy' in context && context.lazy;
}

function isViewingFileContext(context: PendingWebContext): context is PendingViewingFileContext {
	return context.kind === 'file' && 'role' in context && context.role === 'viewing';
}

function toWireWebContext(context: PendingWebContext): WebContext | null {
	if (isPendingPageContext(context)) return null;
	if (isViewingFileContext(context)) {
		return {
			kind: 'file',
			title: context.title,
			source: context.source,
			content: ''
		};
	}
	return {
		kind: context.kind,
		title: context.title,
		source: context.source,
		content: context.content
	};
}

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
	let webContextsBySession = $state<Record<string, PendingWebContext[]>>({});
	let panelHistoryBySession = $state<Record<string, PanelHistoryState>>({});
	/** Suppresses history recording while applying back/forward navigation. */
	let applyingPanelHistory = false;
	let resolvePageContext: ((source: string) => Promise<WebContext | null>) | null = null;
	let focusedPane = $state<FocusedPane>('chat');
	let addressBarFocusRequestId = $state(0);
	let fileTreeFilterFocusRequestId = $state(0);
	/** Last ⌘O focus target while the browse tree is visible. */
	let lastWebPanelFocusTarget = $state<'filter' | 'address'>('filter');
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

	function clearWebContextsForSession(sessionId: string) {
		if (!(sessionId in webContextsBySession)) return;
		const nextContexts = { ...webContextsBySession };
		delete nextContexts[sessionId];
		webContextsBySession = nextContexts;
	}

	function clearPanelHistoryForSession(sessionId: string) {
		if (!(sessionId in panelHistoryBySession)) return;
		const next = { ...panelHistoryBySession };
		delete next[sessionId];
		panelHistoryBySession = next;
	}

	function historyFor(sessionId: string): PanelHistoryState {
		return panelHistoryBySession[sessionId] ?? createPanelHistoryState();
	}

	function recordPanelHistory(sessionId: string, entry: PanelHistoryEntry) {
		if (applyingPanelHistory) return;
		const next = pushEntry(historyFor(sessionId), entry);
		panelHistoryBySession = {
			...panelHistoryBySession,
			[sessionId]: next
		};
	}

	function applyPanelHistoryEntry(sessionId: string, entry: PanelHistoryEntry) {
		applyingPanelHistory = true;
		try {
			if (entry.kind === 'browse') {
				webPanelsBySession = {
					...webPanelsBySession,
					[sessionId]: { mode: 'url', url: '', visible: true }
				};
			} else if (entry.kind === 'file') {
				webPanelsBySession = {
					...webPanelsBySession,
					[sessionId]: { mode: 'file', filePath: entry.path, visible: true }
				};
			} else {
				webPanelsBySession = {
					...webPanelsBySession,
					[sessionId]: { mode: 'url', url: entry.url, visible: true }
				};
			}
			focusedPane = 'web';
			syncWebPanelOpen(true);
		} finally {
			applyingPanelHistory = false;
		}
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
		get pendingWebContexts(): PendingWebContext[] {
			return webContextsBySession[panelSessionKey()] ?? [];
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
		get fileTreeFilterFocusRequestId() {
			return fileTreeFilterFocusRequestId;
		},
		get composerFocusRequestId() {
			return composerFocusRequestId;
		},
		get canPanelHistoryBack() {
			return historyCanGoBack(historyFor(panelSessionKey()));
		},
		get canPanelHistoryForward() {
			return historyCanGoForward(historyFor(panelSessionKey()));
		},
		/** True when the active panel is the empty browse (file tree) state. */
		get webPanelBrowseOpen() {
			const panel = panelForActiveSession();
			return Boolean(panel?.visible && panel.mode === 'url' && !panel.url);
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
		addWebContextForActive(context: WebContext) {
			const key = panelSessionKey();
			const existing = webContextsBySession[key] ?? [];
			let next = existing;
			if (context.kind === 'page') {
				next = existing.filter(
					(item) => !(isPendingPageContext(item) && item.source === context.source)
				);
			}
			const nextContext: WebContext = {
				...context,
				content: context.content.trim().slice(0, 50000)
			};
			webContextsBySession = {
				...webContextsBySession,
				[key]: [...next, nextContext]
			};
		},
		setViewingFileContextForActive(source: string, title: string) {
			const key = panelSessionKey();
			const current = webContextsBySession[key] ?? [];
			const existingViewing = current.find(isViewingFileContext);
			if (existingViewing?.source === source && (existingViewing.title ?? '') === title) {
				return;
			}
			const existing = current.filter((item) => !isViewingFileContext(item));
			const viewing: PendingViewingFileContext = {
				kind: 'file',
				role: 'viewing',
				title,
				source,
				content: ''
			};
			webContextsBySession = {
				...webContextsBySession,
				[key]: [...existing, viewing]
			};
		},
		setPendingPageContextForActive(context: Omit<PendingPageContext, 'kind' | 'lazy'>) {
			const key = panelSessionKey();
			const existing = (webContextsBySession[key] ?? []).filter((item) => !isPendingPageContext(item));
			webContextsBySession = {
				...webContextsBySession,
				[key]: [
					...existing,
					{ kind: 'page', title: context.title, source: context.source, lazy: true }
				]
			};
		},
		registerPageContextResolver(resolver: (source: string) => Promise<WebContext | null>) {
			resolvePageContext = resolver;
			return () => {
				if (resolvePageContext === resolver) resolvePageContext = null;
			};
		},
		async resolvePendingWebContextsForActive(): Promise<WebContext[]> {
			const contexts = [...(webContextsBySession[panelSessionKey()] ?? [])];
			const resolved = await Promise.all(
				contexts.map(async (context) => {
					if (isPendingPageContext(context)) {
						return resolvePageContext?.(context.source) ?? null;
					}
					return toWireWebContext(context);
				})
			);
			return resolved.filter((context): context is WebContext => context !== null);
		},
		removeWebContextAt(index: number) {
			const key = panelSessionKey();
			const existing = webContextsBySession[key] ?? [];
			if (index < 0 || index >= existing.length) return;
			const next = existing.filter((_, i) => i !== index);
			if (next.length === 0) {
				const copy = { ...webContextsBySession };
				delete copy[key];
				webContextsBySession = copy;
				return;
			}
			webContextsBySession = {
				...webContextsBySession,
				[key]: next
			};
		},
		clearWebContextForActive() {
			const key = panelSessionKey();
			if (!(key in webContextsBySession)) return;
			const next = { ...webContextsBySession };
			delete next[key];
			webContextsBySession = next;
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
			if (url) {
				recordPanelHistory(sessionId, { kind: 'url', url });
			} else {
				recordPanelHistory(sessionId, { kind: 'browse' });
			}
			focusedPane = 'web';
			syncWebPanelOpen(true);
		},
		openFilePreview(filePath: string, sessionId: string) {
			webPanelsBySession = {
				...webPanelsBySession,
				[sessionId]: { mode: 'file', filePath, visible: true }
			};
			recordPanelHistory(sessionId, { kind: 'file', path: filePath });
			focusedPane = 'web';
			syncWebPanelOpen(true);
		},
		openWebPanelEmpty() {
			const sessionId = panelSessionKey();
			webPanelsBySession = {
				...webPanelsBySession,
				[sessionId]: { mode: 'url', url: '', visible: true }
			};
			recordPanelHistory(sessionId, { kind: 'browse' });
			focusedPane = 'web';
			syncWebPanelOpen(true);
			this.requestFileTreeFilterFocus();
		},
		navigateWebPanel(url: string) {
			const sessionId = panelSessionKey();
			webPanelsBySession = {
				...webPanelsBySession,
				[sessionId]: { mode: 'url', url, visible: true }
			};
			if (url) {
				recordPanelHistory(sessionId, { kind: 'url', url });
			} else {
				recordPanelHistory(sessionId, { kind: 'browse' });
			}
			focusedPane = 'web';
			syncWebPanelOpen(true);
			if (url) {
				this.requestAddressBarFocus();
			} else {
				this.requestFileTreeFilterFocus();
			}
		},
		panelHistoryBack() {
			const sessionId = panelSessionKey();
			const prev = historyFor(sessionId);
			if (!historyCanGoBack(prev)) return false;
			const next = historyGoBack(prev);
			panelHistoryBySession = { ...panelHistoryBySession, [sessionId]: next };
			const entry = currentEntry(next);
			if (!entry) return false;
			applyPanelHistoryEntry(sessionId, entry);
			return true;
		},
		panelHistoryForward() {
			const sessionId = panelSessionKey();
			const prev = historyFor(sessionId);
			if (!historyCanGoForward(prev)) return false;
			const next = historyGoForward(prev);
			panelHistoryBySession = { ...panelHistoryBySession, [sessionId]: next };
			const entry = currentEntry(next);
			if (!entry) return false;
			applyPanelHistoryEntry(sessionId, entry);
			return true;
		},
		ensureWebPanelVisible() {
			const sessionId = panelSessionKey();
			const panel = webPanelsBySession[sessionId];
			if (!panel) return null;
			if (!panel.visible) {
				webPanelsBySession = {
					...webPanelsBySession,
					[sessionId]: { ...panel, visible: true }
				};
				syncWebPanelOpen(true);
			}
			focusedPane = 'web';
			return webPanelsBySession[sessionId] ?? panel;
		},
		requestFileTreeFilterFocus() {
			if (!this.ensureWebPanelVisible()) return;
			lastWebPanelFocusTarget = 'filter';
			fileTreeFilterFocusRequestId += 1;
		},
		requestAddressBarFocus() {
			if (!this.ensureWebPanelVisible()) return;
			lastWebPanelFocusTarget = 'address';
			addressBarFocusRequestId += 1;
		},
		openWebPanelFromShortcut() {
			const panel = panelForActiveSession();
			if (!panel) {
				this.openWebPanelEmpty();
				return;
			}
			const visible = this.ensureWebPanelVisible();
			if (!visible) return;
			const browse = visible.mode === 'url' && !visible.url;
			if (browse) {
				if (lastWebPanelFocusTarget === 'address') {
					this.requestFileTreeFilterFocus();
				} else {
					this.requestAddressBarFocus();
				}
				return;
			}
			this.requestAddressBarFocus();
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
			if (visible && panel.mode === 'url' && !panel.url) {
				this.requestFileTreeFilterFocus();
			} else if (visible && panel.mode === 'url') {
				this.requestAddressBarFocus();
			}
		},
		closeWebPanel() {
			const sessionId = panelSessionKey();
			if (!webPanelsBySession[sessionId]) {
				clearWebContextsForSession(sessionId);
				clearPanelHistoryForSession(sessionId);
				return;
			}
			const next = { ...webPanelsBySession };
			delete next[sessionId];
			webPanelsBySession = next;
			clearWebContextsForSession(sessionId);
			clearPanelHistoryForSession(sessionId);
			this.requestComposerFocus();
			syncWebPanelOpen(false);
		},
		clearWebPanelForSession(sessionId: string) {
			if (!webPanelsBySession[sessionId]) return;
			const next = { ...webPanelsBySession };
			delete next[sessionId];
			webPanelsBySession = next;
			if (sessionId in webContextsBySession) {
				const nextContexts = { ...webContextsBySession };
				delete nextContexts[sessionId];
				webContextsBySession = nextContexts;
			}
			clearPanelHistoryForSession(sessionId);
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
			const draftHistory = panelHistoryBySession[DRAFT_SESSION_KEY];
			if (draftHistory) {
				const nextHistory = { ...panelHistoryBySession, [sessionId]: draftHistory };
				delete nextHistory[DRAFT_SESSION_KEY];
				panelHistoryBySession = nextHistory;
			}
		},
		/** Discards a draft panel without migrating it. */
		clearDraftPanel() {
			if (!webPanelsBySession[DRAFT_SESSION_KEY]) return;
			const next = { ...webPanelsBySession };
			delete next[DRAFT_SESSION_KEY];
			webPanelsBySession = next;
			if (DRAFT_SESSION_KEY in webContextsBySession) {
				const nextContexts = { ...webContextsBySession };
				delete nextContexts[DRAFT_SESSION_KEY];
				webContextsBySession = nextContexts;
			}
			clearPanelHistoryForSession(DRAFT_SESSION_KEY);
		}
	};
}

export const shellStore = createShellStore();
