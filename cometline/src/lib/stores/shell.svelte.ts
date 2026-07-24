import { getActiveSessionId } from '$lib/active-session';
import { readHasSeenIntroSync } from '$lib/stores/settings.svelte';
import type { WebContext } from '$lib/actions/start-chat';
import {
	canGoBack as historyCanGoBack,
	canGoForward as historyCanGoForward,
	createPanelHistoryState,
	currentEntry,
	entriesEqual,
	goBack as historyGoBack,
	goForward as historyGoForward,
	pushEntry,
	type PanelHistoryEntry,
	type PanelHistoryState
} from '$lib/workspace/panel-history';
import {
	readWebPanelTreeSource,
	writeWebPanelTreeSource,
	type WebPanelTreeSource
} from '$lib/workspace/web-panel-prefs';
import { dirKeysToExpandForPaths } from '$lib/workspace/file-tree';
import { isWikiUiPath, toWikiRelative } from '$lib/wiki/paths';

export type WebPanelMode = 'url' | 'file' | 'git-diff';
/** File-tree sources that keep an expansion map (not Changes). */
export type FileTreeExpandSource = 'wiki' | 'workspace';
/** Surfaces that can own independent open content. */
export type SurfaceContentKey = 'wiki' | 'workspace' | 'changes' | 'web-search';
/**
 * Inner right-sidebar surface (under the web slot).
 * Content is owned by the active surface (no separate 'content' surface).
 */
export type WebPanelSurface = SurfaceContentKey;

export type SurfaceContent =
	| { mode: 'file'; filePath: string }
	| { mode: 'git-diff'; filePath: string }
	| { mode: 'url'; url: string };

/** @deprecated Prefer SurfaceContent — kept for callers that still say SessionWebPanel. */
export type SessionWebPanel =
	| { mode: 'url'; url: string; visible: boolean }
	| { mode: 'file'; filePath: string; visible: boolean }
	| { mode: 'git-diff'; filePath: string; visible: boolean };

export type FocusedPane = 'chat' | 'web' | 'terminal';
export type WorkspacePanelSurface = 'web' | 'terminal';

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
	/** Soft visibility for the web slot (false = hidden, state retained). */
	let webPanelVisibleBySession = $state<Record<string, boolean>>({});
	/** Per-surface open file / page / diff (independent lifecycles). */
	let contentBySessionSurface = $state<
		Record<string, Partial<Record<SurfaceContentKey, SurfaceContent>>>
	>({});
	let terminalPanelsBySession = $state<Record<string, boolean>>({});
	let workspacePanelSurfaceBySession = $state<Record<string, WorkspacePanelSurface>>({});
	/** Active inner surface while the outer slot is `web`. */
	let webPanelSurfaceBySession = $state<Record<string, WebPanelSurface>>({});
	let webContextsBySession = $state<Record<string, PendingWebContext[]>>({});
	/** Per-surface Back/Forward stacks. */
	let panelHistoryBySessionSurface = $state<
		Record<string, Partial<Record<SurfaceContentKey, PanelHistoryState>>>
	>({});
	/** Per-session Wiki/Workspace/Changes preference (last browse tab + history seed). */
	let browseSourceBySession = $state<Record<string, WebPanelTreeSource>>({});
	/**
	 * Expanded directory keys for Wiki/Workspace file trees, per session.
	 * Survives open-file → back so the tree does not collapse.
	 */
	let fileTreeExpandedBySession = $state<
		Record<string, Partial<Record<FileTreeExpandSource, Record<string, boolean>>>>
	>({});
	/** Per-session filter text for Wiki/Workspace trees (independent lifecycles). */
	let fileTreeFilterBySession = $state<
		Record<string, Partial<Record<FileTreeExpandSource, string>>>
	>({});
	/** Suppresses history recording while applying back/forward navigation. */
	let applyingPanelHistory = false;
	let resolvePageContext: ((source: string) => Promise<WebContext | null>) | null = null;
	let focusedPane = $state<FocusedPane>('chat');
	let addressBarFocusRequestId = $state(0);
	let fileTreeFilterFocusRequestId = $state(0);
	let gitChangesOpenRequestId = $state(0);
	let terminalFocusRequestId = $state(0);
	/** Last focus target while the web slot is open: filter vs web address. */
	let lastWebPanelFocusTarget = $state<'filter' | 'address'>('filter');
	let composerFocusRequestId = $state(0);

	function activeSessionId(): string | null {
		return getActiveSessionId();
	}

	/** Active session id used to scope panel state; null when none is open. */
	function panelSessionKey(): string | null {
		return activeSessionId();
	}

	function activeWorkspacePanelSurface(): WorkspacePanelSurface {
		const key = panelSessionKey();
		if (!key) return 'web';
		return workspacePanelSurfaceBySession[key] ?? 'web';
	}

	function workspacePanelOpenForActiveSession() {
		const sessionId = panelSessionKey();
		if (!sessionId) return false;
		if (activeWorkspacePanelSurface() === 'terminal')
			return terminalPanelsBySession[sessionId] === true;
		return webPanelVisibleBySession[sessionId] === true;
	}

	function syncWorkspacePanelOpen(open: boolean) {
		window.electronAPI?.setWebPanelOpen?.(open);
	}

	function syncWorkspacePanelOpenForActiveSession() {
		syncWorkspacePanelOpen(workspacePanelOpenForActiveSession());
	}

	function clearWebContextsForSession(sessionId: string) {
		if (!(sessionId in webContextsBySession)) return;
		const nextContexts = { ...webContextsBySession };
		delete nextContexts[sessionId];
		webContextsBySession = nextContexts;
	}

	function clearWebPanelContextsForSession(sessionId: string) {
		const contexts = webContextsBySession[sessionId];
		if (!contexts) return;
		const retained = contexts.filter((context) => context.kind === 'terminal');
		if (retained.length === 0) {
			clearWebContextsForSession(sessionId);
			return;
		}
		webContextsBySession = { ...webContextsBySession, [sessionId]: retained };
	}

	function clearPanelHistoryForSession(sessionId: string) {
		if (!(sessionId in panelHistoryBySessionSurface)) return;
		const next = { ...panelHistoryBySessionSurface };
		delete next[sessionId];
		panelHistoryBySessionSurface = next;
	}

	function historyFor(sessionId: string, surface: SurfaceContentKey): PanelHistoryState {
		return panelHistoryBySessionSurface[sessionId]?.[surface] ?? createPanelHistoryState();
	}

	function setHistoryFor(
		sessionId: string,
		surface: SurfaceContentKey,
		state: PanelHistoryState
	) {
		panelHistoryBySessionSurface = {
			...panelHistoryBySessionSurface,
			[sessionId]: {
				...panelHistoryBySessionSurface[sessionId],
				[surface]: state
			}
		};
	}

	function browseSourceFor(sessionId: string): WebPanelTreeSource {
		return browseSourceBySession[sessionId] ?? readWebPanelTreeSource();
	}

	function setBrowseSourceForSession(sessionId: string, source: WebPanelTreeSource) {
		browseSourceBySession = { ...browseSourceBySession, [sessionId]: source };
		writeWebPanelTreeSource(source);
	}

	function defaultWebPanelSurfaceFor(sessionId: string): WebPanelSurface {
		const source = browseSourceFor(sessionId);
		if (source === 'workspace' || source === 'changes') return source;
		return 'wiki';
	}

	function webPanelSurfaceFor(sessionId: string): WebPanelSurface {
		const surface = webPanelSurfaceBySession[sessionId] ?? defaultWebPanelSurfaceFor(sessionId);
		// Migrate legacy 'content' if any stale value lingered in memory during hot reload.
		if ((surface as string) === 'content') return defaultWebPanelSurfaceFor(sessionId);
		return surface;
	}

	function setWebPanelSurfaceForSession(sessionId: string, surface: WebPanelSurface) {
		webPanelSurfaceBySession = { ...webPanelSurfaceBySession, [sessionId]: surface };
		if (surface === 'wiki' || surface === 'workspace' || surface === 'changes') {
			setBrowseSourceForSession(sessionId, surface);
		}
	}

	function contentFor(sessionId: string, surface: SurfaceContentKey): SurfaceContent | null {
		return contentBySessionSurface[sessionId]?.[surface] ?? null;
	}

	function setContentFor(
		sessionId: string,
		surface: SurfaceContentKey,
		content: SurfaceContent | null
	) {
		const prev = contentBySessionSurface[sessionId] ?? {};
		if (content === null) {
			const nextSurface = { ...prev };
			delete nextSurface[surface];
			contentBySessionSurface = {
				...contentBySessionSurface,
				[sessionId]: nextSurface
			};
			return;
		}
		contentBySessionSurface = {
			...contentBySessionSurface,
			[sessionId]: {
				...prev,
				[surface]: content
			}
		};
	}

	function clearContentForSession(sessionId: string) {
		if (!(sessionId in contentBySessionSurface)) return;
		const next = { ...contentBySessionSurface };
		delete next[sessionId];
		contentBySessionSurface = next;
	}

	/** Toolbar order used when Cmd+W walks remaining content dots. */
	const SURFACE_CLOSE_ORDER: SurfaceContentKey[] = [
		'wiki',
		'workspace',
		'web-search',
		'changes'
	];

	/** Next surface (after `current`) that still owns open content, or null. */
	function nextSurfaceWithContent(
		sessionId: string,
		current: SurfaceContentKey
	): SurfaceContentKey | null {
		const start = SURFACE_CLOSE_ORDER.indexOf(current);
		if (start < 0) return null;
		for (let i = 1; i < SURFACE_CLOSE_ORDER.length; i++) {
			const candidate = SURFACE_CLOSE_ORDER[(start + i) % SURFACE_CLOSE_ORDER.length];
			if (contentFor(sessionId, candidate)) return candidate;
		}
		return null;
	}

	function activateWebSurface(sessionId: string, surface: SurfaceContentKey) {
		workspacePanelSurfaceBySession = {
			...workspacePanelSurfaceBySession,
			[sessionId]: 'web'
		};
		ensureWebPanelVisible(sessionId);
		setWebPanelSurfaceForSession(sessionId, surface);
		focusedPane = 'web';
		syncWorkspacePanelOpen(true);
	}

	function ensureWebPanelVisible(sessionId: string) {
		webPanelVisibleBySession = { ...webPanelVisibleBySession, [sessionId]: true };
	}

	function softHideWebPanel(sessionId: string) {
		if (!(sessionId in webPanelVisibleBySession)) return;
		webPanelVisibleBySession = { ...webPanelVisibleBySession, [sessionId]: false };
	}

	function hasWebPanelSession(sessionId: string): boolean {
		return sessionId in webPanelVisibleBySession;
	}

	function ownerSurfaceForFile(filePath: string): FileTreeExpandSource {
		return isWikiUiPath(filePath) ? 'wiki' : 'workspace';
	}

	function activeSurfaceContent(sessionId: string): SurfaceContent | null {
		return contentFor(sessionId, webPanelSurfaceFor(sessionId));
	}

	function fileTreeFilterFor(sessionId: string, source: FileTreeExpandSource): string {
		return fileTreeFilterBySession[sessionId]?.[source] ?? '';
	}

	function setFileTreeFilterForSession(
		sessionId: string,
		source: FileTreeExpandSource,
		value: string
	) {
		fileTreeFilterBySession = {
			...fileTreeFilterBySession,
			[sessionId]: {
				...fileTreeFilterBySession[sessionId],
				[source]: value
			}
		};
	}

	function clearFileTreeFilterForSession(sessionId: string) {
		if (!(sessionId in fileTreeFilterBySession)) return;
		const next = { ...fileTreeFilterBySession };
		delete next[sessionId];
		fileTreeFilterBySession = next;
	}

	function clearWebPanelSurfaceForSession(sessionId: string) {
		if (!(sessionId in webPanelSurfaceBySession)) return;
		const next = { ...webPanelSurfaceBySession };
		delete next[sessionId];
		webPanelSurfaceBySession = next;
	}

	function clearWebPanelVisibleForSession(sessionId: string) {
		if (!(sessionId in webPanelVisibleBySession)) return;
		const next = { ...webPanelVisibleBySession };
		delete next[sessionId];
		webPanelVisibleBySession = next;
	}

	function fileTreeExpandedFor(
		sessionId: string,
		source: FileTreeExpandSource
	): Record<string, boolean> {
		return fileTreeExpandedBySession[sessionId]?.[source] ?? {};
	}

	function setFileTreeExpandedForSession(
		sessionId: string,
		source: FileTreeExpandSource,
		expanded: Record<string, boolean>
	) {
		fileTreeExpandedBySession = {
			...fileTreeExpandedBySession,
			[sessionId]: {
				...fileTreeExpandedBySession[sessionId],
				[source]: { ...expanded }
			}
		};
	}

	/** Merge ancestor dirs for a relative path into the source expansion map. */
	function expandFileTreeToRelativePath(
		sessionId: string,
		source: FileTreeExpandSource,
		relativePath: string
	) {
		const parents = dirKeysToExpandForPaths([relativePath]);
		if (Object.keys(parents).length === 0) return;
		const prev = fileTreeExpandedFor(sessionId, source);
		setFileTreeExpandedForSession(sessionId, source, { ...prev, ...parents });
	}

	function clearFileTreeExpandedForSession(sessionId: string) {
		if (!(sessionId in fileTreeExpandedBySession)) return;
		const next = { ...fileTreeExpandedBySession };
		delete next[sessionId];
		fileTreeExpandedBySession = next;
	}

	function pushSurfaceHistory(
		state: PanelHistoryState,
		entry: PanelHistoryEntry,
		surface: SurfaceContentKey
	): PanelHistoryState {
		if (surface === 'web-search') {
			const current = currentEntry(state);
			if (current && entriesEqual(current, entry)) return state;
			let base = state.entries.slice(0, state.index + 1);
			if (base.length === 0 && entry.kind === 'url' && entry.url) {
				base = [{ kind: 'url', url: '' }];
			}
			return { entries: [...base, entry], index: base.length };
		}
		const seed: WebPanelTreeSource =
			surface === 'workspace' || surface === 'changes' ? surface : 'wiki';
		return pushEntry(state, entry, seed);
	}

	function recordPanelHistory(
		sessionId: string,
		surface: SurfaceContentKey,
		entry: PanelHistoryEntry
	) {
		if (applyingPanelHistory) return;
		const next = pushSurfaceHistory(historyFor(sessionId, surface), entry, surface);
		setHistoryFor(sessionId, surface, next);
	}

	function applyPanelHistoryEntry(
		sessionId: string,
		surface: SurfaceContentKey,
		entry: PanelHistoryEntry
	) {
		applyingPanelHistory = true;
		try {
			workspacePanelSurfaceBySession = {
				...workspacePanelSurfaceBySession,
				[sessionId]: 'web'
			};
			ensureWebPanelVisible(sessionId);
			if (entry.kind === 'browse') {
				setWebPanelSurfaceForSession(sessionId, entry.source);
				setContentFor(sessionId, entry.source, null);
			} else if (entry.kind === 'file') {
				const owner = ownerSurfaceForFile(entry.path);
				setWebPanelSurfaceForSession(sessionId, owner);
				setContentFor(sessionId, owner, { mode: 'file', filePath: entry.path });
			} else if (entry.kind === 'git-diff') {
				setWebPanelSurfaceForSession(sessionId, 'changes');
				setContentFor(sessionId, 'changes', { mode: 'git-diff', filePath: entry.path });
			} else if (entry.url) {
				setWebPanelSurfaceForSession(sessionId, 'web-search');
				setContentFor(sessionId, 'web-search', { mode: 'url', url: entry.url });
			} else {
				// Empty URL = empty web-search (or cleared content on that stack).
				setWebPanelSurfaceForSession(sessionId, surface === 'web-search' ? 'web-search' : surface);
				if (surface === 'web-search') {
					setContentFor(sessionId, 'web-search', null);
				}
			}
			focusedPane = 'web';
			syncWorkspacePanelOpen(true);
		} finally {
			applyingPanelHistory = false;
		}
	}

	/**
	 * Activate a browse/search surface without clearing its (or others') content.
	 */
	function promoteBrowseSurface(
		sessionId: string,
		source: WebPanelTreeSource,
		options: { recordHistory?: boolean } = {}
	) {
		const prevSurface = webPanelSurfaceFor(sessionId);
		const hadSession = hasWebPanelSession(sessionId);

		setWebPanelSurfaceForSession(sessionId, source);
		workspacePanelSurfaceBySession = {
			...workspacePanelSurfaceBySession,
			[sessionId]: 'web'
		};
		ensureWebPanelVisible(sessionId);

		if (options.recordHistory !== false) {
			const content = contentFor(sessionId, source);
			// Only record browse when landing on a surface with no content (tree visible).
			if (!content && (!hadSession || prevSurface !== source)) {
				recordPanelHistory(sessionId, source, { kind: 'browse', source });
			}
		}
		focusedPane = 'web';
		syncWorkspacePanelOpen(true);
	}

	function openBrowseSurface(sessionId: string, source: WebPanelTreeSource) {
		setWebPanelSurfaceForSession(sessionId, source);
		workspacePanelSurfaceBySession = {
			...workspacePanelSurfaceBySession,
			[sessionId]: 'web'
		};
		ensureWebPanelVisible(sessionId);
		recordPanelHistory(sessionId, source, { kind: 'browse', source });
		focusedPane = 'web';
		syncWorkspacePanelOpen(true);
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
			const key = panelSessionKey();
			return (
				activeWorkspacePanelSurface() === 'web' &&
				Boolean(key && webPanelVisibleBySession[key] === true)
			);
		},
		get workspacePanelOpen() {
			return workspacePanelOpenForActiveSession();
		},
		get workspacePanelSurface() {
			return activeWorkspacePanelSurface();
		},
		get terminalPanelOpen() {
			const key = panelSessionKey();
			if (!key) return false;
			return (
				activeWorkspacePanelSurface() === 'terminal' && terminalPanelsBySession[key] === true
			);
		},
		get webPanelMode(): WebPanelMode | null {
			const key = panelSessionKey();
			if (!key) return null;
			return activeSurfaceContent(key)?.mode ?? null;
		},
		get webPanelUrl() {
			const key = panelSessionKey();
			if (!key) return null;
			const content = activeSurfaceContent(key);
			return content?.mode === 'url' ? content.url : null;
		},
		get webPanelFilePath() {
			const key = panelSessionKey();
			if (!key) return null;
			const content = activeSurfaceContent(key);
			return content?.mode === 'file' ? content.filePath : null;
		},
		get webPanelGitDiffPath() {
			const key = panelSessionKey();
			if (!key) return null;
			const content = activeSurfaceContent(key);
			return content?.mode === 'git-diff' ? content.filePath : null;
		},
		get webPanelBrowseSource(): WebPanelTreeSource {
			const key = panelSessionKey();
			return key ? browseSourceFor(key) : readWebPanelTreeSource();
		},
		/** Active inner surface for the web right-sidebar stack. */
		get webPanelSurface(): WebPanelSurface {
			const key = panelSessionKey();
			return key ? webPanelSurfaceFor(key) : 'wiki';
		},
		get pendingWebContexts(): PendingWebContext[] {
			const key = panelSessionKey();
			return key ? (webContextsBySession[key] ?? []) : [];
		},
		get hasWebPanelForSession() {
			const key = panelSessionKey();
			return Boolean(key && hasWebPanelSession(key));
		},
		/** Storage key for the active session's panel, or null when none is open. */
		get webPanelSessionKey() {
			return panelSessionKey();
		},
		get addressBarFocusRequestId() {
			return addressBarFocusRequestId;
		},
		get fileTreeFilterFocusRequestId() {
			return fileTreeFilterFocusRequestId;
		},
		get gitChangesOpenRequestId() {
			return gitChangesOpenRequestId;
		},
		/** Last ⌘O target while browsing: filter vs web address. */
		get lastWebPanelFocusTarget(): 'filter' | 'address' {
			return lastWebPanelFocusTarget;
		},
		get terminalFocusRequestId() {
			return terminalFocusRequestId;
		},
		get composerFocusRequestId() {
			return composerFocusRequestId;
		},
		get canPanelHistoryBack() {
			const key = panelSessionKey();
			if (!key) return false;
			return historyCanGoBack(historyFor(key, webPanelSurfaceFor(key)));
		},
		get canPanelHistoryForward() {
			const key = panelSessionKey();
			if (!key) return false;
			return historyCanGoForward(historyFor(key, webPanelSurfaceFor(key)));
		},
		/** True when a browse layer is active and that surface has no open content. */
		get webPanelBrowseOpen() {
			const key = panelSessionKey();
			if (!key || webPanelVisibleBySession[key] !== true) return false;
			const surface = webPanelSurfaceFor(key);
			if (surface !== 'wiki' && surface !== 'workspace' && surface !== 'changes') return false;
			return contentFor(key, surface) === null;
		},
		get webPanelGitDiffOpen() {
			const key = panelSessionKey();
			if (!key || webPanelVisibleBySession[key] !== true) return false;
			const content = contentFor(key, 'changes');
			return webPanelSurfaceFor(key) === 'changes' && content?.mode === 'git-diff';
		},
		/** Read content owned by a specific surface (for stacked UI layers). */
		getSurfaceContent(surface: SurfaceContentKey): SurfaceContent | null {
			const key = panelSessionKey();
			return key ? contentFor(key, surface) : null;
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
			if (!key) return;
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
			if (!key) return;
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
			if (!key) return;
			const existing = (webContextsBySession[key] ?? []).filter(
				(item) => !isPendingPageContext(item)
			);
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
			const key = panelSessionKey();
			const contexts = key ? [...(webContextsBySession[key] ?? [])] : [];
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
			if (!key) return;
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
			if (!key || !(key in webContextsBySession)) return;
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
			syncWorkspacePanelOpenForActiveSession();
		},
		openWebPanel(url: string, sessionId: string) {
			workspacePanelSurfaceBySession = {
				...workspacePanelSurfaceBySession,
				[sessionId]: 'web'
			};
			ensureWebPanelVisible(sessionId);
			if (url) {
				setWebPanelSurfaceForSession(sessionId, 'web-search');
				setContentFor(sessionId, 'web-search', { mode: 'url', url });
				recordPanelHistory(sessionId, 'web-search', { kind: 'url', url });
			} else {
				const surface = defaultWebPanelSurfaceFor(sessionId);
				setWebPanelSurfaceForSession(sessionId, surface);
				recordPanelHistory(sessionId, surface, {
					kind: 'browse',
					source: surface === 'web-search' ? 'wiki' : surface
				});
			}
			focusedPane = 'web';
			syncWorkspacePanelOpen(true);
		},
		openFilePreview(filePath: string, sessionId: string) {
			const owner = ownerSurfaceForFile(filePath);
			const relative = isWikiUiPath(filePath) ? toWikiRelative(filePath) : filePath;
			expandFileTreeToRelativePath(sessionId, owner, relative);
			workspacePanelSurfaceBySession = {
				...workspacePanelSurfaceBySession,
				[sessionId]: 'web'
			};
			ensureWebPanelVisible(sessionId);
			setWebPanelSurfaceForSession(sessionId, owner);
			setContentFor(sessionId, owner, { mode: 'file', filePath });
			recordPanelHistory(sessionId, owner, { kind: 'file', path: filePath });
			focusedPane = 'web';
			syncWorkspacePanelOpen(true);
		},
		openGitDiff(filePath: string, sessionId: string) {
			workspacePanelSurfaceBySession = {
				...workspacePanelSurfaceBySession,
				[sessionId]: 'web'
			};
			ensureWebPanelVisible(sessionId);
			setWebPanelSurfaceForSession(sessionId, 'changes');
			setContentFor(sessionId, 'changes', { mode: 'git-diff', filePath });
			recordPanelHistory(sessionId, 'changes', { kind: 'git-diff', path: filePath });
			focusedPane = 'web';
			syncWorkspacePanelOpen(true);
		},
		openWebPanelEmpty() {
			const sessionId = panelSessionKey();
			if (!sessionId) return;
			openBrowseSurface(sessionId, browseSourceFor(sessionId));
			this.requestFileTreeFilterFocus();
		},
		/** Switch Wiki / Workspace / Changes — covers other layers; does not destroy them. */
		setWebPanelBrowseSource(source: WebPanelTreeSource) {
			const sessionId = panelSessionKey();
			if (!sessionId) return;
			promoteBrowseSurface(sessionId, source);
			const content = contentFor(sessionId, source);
			if (content) {
				// Surface already has open content — keep it visible, no filter steal.
				focusedPane = 'web';
				return;
			}
			if (source === 'wiki' || source === 'workspace') {
				this.requestFileTreeFilterFocus();
			} else {
				lastWebPanelFocusTarget = 'filter';
				focusedPane = 'web';
			}
		},
		navigateWebPanel(url: string) {
			const sessionId = panelSessionKey();
			if (!sessionId) return;
			workspacePanelSurfaceBySession = {
				...workspacePanelSurfaceBySession,
				[sessionId]: 'web'
			};
			ensureWebPanelVisible(sessionId);
			if (url) {
				setWebPanelSurfaceForSession(sessionId, 'web-search');
				setContentFor(sessionId, 'web-search', { mode: 'url', url });
				recordPanelHistory(sessionId, 'web-search', { kind: 'url', url });
				focusedPane = 'web';
				syncWorkspacePanelOpen(true);
				this.requestAddressBarFocus();
				return;
			}
			const surface = defaultWebPanelSurfaceFor(sessionId);
			setWebPanelSurfaceForSession(sessionId, surface);
			setContentFor(sessionId, 'web-search', null);
			recordPanelHistory(sessionId, surface, {
				kind: 'browse',
				source: surface === 'web-search' ? 'wiki' : surface
			});
			focusedPane = 'web';
			syncWorkspacePanelOpen(true);
			this.requestFileTreeFilterFocus();
		},
		panelHistoryBack() {
			const sessionId = panelSessionKey();
			if (!sessionId) return false;
			const surface = webPanelSurfaceFor(sessionId);
			const prev = historyFor(sessionId, surface);
			if (!historyCanGoBack(prev)) return false;
			const next = historyGoBack(prev);
			setHistoryFor(sessionId, surface, next);
			const entry = currentEntry(next);
			if (!entry) return false;
			applyPanelHistoryEntry(sessionId, surface, entry);
			return true;
		},
		panelHistoryForward() {
			const sessionId = panelSessionKey();
			if (!sessionId) return false;
			const surface = webPanelSurfaceFor(sessionId);
			const prev = historyFor(sessionId, surface);
			if (!historyCanGoForward(prev)) return false;
			const next = historyGoForward(prev);
			setHistoryFor(sessionId, surface, next);
			const entry = currentEntry(next);
			if (!entry) return false;
			applyPanelHistoryEntry(sessionId, surface, entry);
			return true;
		},
		ensureWebPanelVisible() {
			const sessionId = panelSessionKey();
			if (!sessionId) return null;
			if (!hasWebPanelSession(sessionId)) return null;
			workspacePanelSurfaceBySession = {
				...workspacePanelSurfaceBySession,
				[sessionId]: 'web'
			};
			ensureWebPanelVisible(sessionId);
			focusedPane = 'web';
			syncWorkspacePanelOpen(true);
			return true;
		},
		requestFileTreeFilterFocus() {
			const sessionId = panelSessionKey();
			if (!sessionId) return;
			if (!hasWebPanelSession(sessionId)) {
				openBrowseSurface(sessionId, browseSourceFor(sessionId));
			} else {
				this.ensureWebPanelVisible();
			}
			lastWebPanelFocusTarget = 'filter';
			fileTreeFilterFocusRequestId += 1;
		},
		requestAddressBarFocus() {
			const sessionId = panelSessionKey();
			if (!sessionId) return;
			if (!hasWebPanelSession(sessionId)) {
				openBrowseSurface(sessionId, browseSourceFor(sessionId));
			} else {
				this.ensureWebPanelVisible();
			}
			lastWebPanelFocusTarget = 'address';
			addressBarFocusRequestId += 1;
		},
		/** ⌘O: open the right sidebar on web search (address bar). */
		openWebSearchPanel() {
			const sessionId = panelSessionKey();
			if (!sessionId) return;
			if (!hasWebPanelSession(sessionId)) {
				ensureWebPanelVisible(sessionId);
			}
			workspacePanelSurfaceBySession = {
				...workspacePanelSurfaceBySession,
				[sessionId]: 'web'
			};
			ensureWebPanelVisible(sessionId);
			setWebPanelSurfaceForSession(sessionId, 'web-search');
			focusedPane = 'web';
			syncWorkspacePanelOpen(true);
			this.requestAddressBarFocus();
		},
		/** @deprecated Use openWebSearchPanel — kept as the ⌘O action id wiring. */
		openWebPanelFromShortcut() {
			this.openWebSearchPanel();
		},
		/** Open the web panel browse surface on the Git Changes tab (⌘⇧G). */
		openGitChangesPanel() {
			this.setWebPanelBrowseSource('changes');
			gitChangesOpenRequestId += 1;
		},
		toggleWebPanel() {
			const sessionId = panelSessionKey();
			if (!sessionId || !hasWebPanelSession(sessionId)) return;
			workspacePanelSurfaceBySession = {
				...workspacePanelSurfaceBySession,
				[sessionId]: 'web'
			};
			const visible = webPanelVisibleBySession[sessionId] !== true;
			webPanelVisibleBySession = { ...webPanelVisibleBySession, [sessionId]: visible };
			focusedPane = visible ? 'web' : 'chat';
			syncWorkspacePanelOpen(visible);
			if (!visible) return;
			const surface = webPanelSurfaceFor(sessionId);
			const content = contentFor(sessionId, surface);
			if (surface === 'web-search' || content?.mode === 'url') {
				this.requestAddressBarFocus();
			} else if (
				(surface === 'wiki' || surface === 'workspace') &&
				content === null
			) {
				this.requestFileTreeFilterFocus();
			}
		},
		openTerminalPanel() {
			const sessionId = activeSessionId();
			if (!sessionId) return false;
			terminalPanelsBySession = { ...terminalPanelsBySession, [sessionId]: true };
			workspacePanelSurfaceBySession = {
				...workspacePanelSurfaceBySession,
				[sessionId]: 'terminal'
			};
			focusedPane = 'terminal';
			terminalFocusRequestId += 1;
			syncWorkspacePanelOpen(true);
			return true;
		},
		requestTerminalFocus() {
			// openTerminalPanel already bumps terminalFocusRequestId.
			this.openTerminalPanel();
		},
		closeWorkspacePanel() {
			const sessionId = panelSessionKey();
			if (!sessionId) {
				this.requestComposerFocus();
				syncWorkspacePanelOpen(false);
				return;
			}
			if (activeWorkspacePanelSurface() === 'terminal') {
				// Terminal soft-hide only — leave web surface content (dots) intact.
				terminalPanelsBySession = { ...terminalPanelsBySession, [sessionId]: false };
				this.requestComposerFocus();
				syncWorkspacePanelOpen(false);
				return;
			}

			const surface = webPanelSurfaceFor(sessionId);
			const content = contentFor(sessionId, surface);
			if (content) {
				// 1) Dismiss open file/page on this surface → back to browse/search.
				setContentFor(sessionId, surface, null);
				if (surface === 'wiki' || surface === 'workspace') {
					recordPanelHistory(sessionId, surface, { kind: 'browse', source: surface });
					this.requestFileTreeFilterFocus();
				} else if (surface === 'web-search') {
					recordPanelHistory(sessionId, 'web-search', { kind: 'url', url: '' });
					this.requestAddressBarFocus();
				} else {
					recordPanelHistory(sessionId, 'changes', { kind: 'browse', source: 'changes' });
					focusedPane = 'web';
				}
				return;
			}

			// 2) Surface already at browse/search — jump to another surface that still has a content dot.
			const next = nextSurfaceWithContent(sessionId, surface);
			if (next) {
				activateWebSurface(sessionId, next);
				return;
			}

			// 3) No remaining content dots — soft-hide sidebar (keep trees/history).
			softHideWebPanel(sessionId);
			this.requestComposerFocus();
			syncWorkspacePanelOpen(false);
		},
		closeWebPanel() {
			this.closeWorkspacePanel();
		},
		closeTerminalPanelForSession(sessionId: string) {
			const next = { ...terminalPanelsBySession };
			delete next[sessionId];
			terminalPanelsBySession = next;
			if (activeSessionId() !== sessionId || activeWorkspacePanelSurface() !== 'terminal') return;
			this.requestComposerFocus();
			syncWorkspacePanelOpen(false);
		},
		clearWebPanelForSession(sessionId: string) {
			clearWebPanelVisibleForSession(sessionId);
			clearContentForSession(sessionId);
			const nextTerminalPanels = { ...terminalPanelsBySession };
			delete nextTerminalPanels[sessionId];
			terminalPanelsBySession = nextTerminalPanels;
			const nextSurfaces = { ...workspacePanelSurfaceBySession };
			delete nextSurfaces[sessionId];
			workspacePanelSurfaceBySession = nextSurfaces;
			clearWebPanelSurfaceForSession(sessionId);
			clearWebContextsForSession(sessionId);
			clearPanelHistoryForSession(sessionId);
			clearFileTreeExpandedForSession(sessionId);
			clearFileTreeFilterForSession(sessionId);
			if (activeSessionId() === sessionId) {
				this.requestComposerFocus();
				syncWorkspacePanelOpen(false);
			}
		},
		/** Read expanded dir keys for Wiki or Workspace tree (session-scoped). */
		getFileTreeExpanded(source: FileTreeExpandSource): Record<string, boolean> {
			const key = panelSessionKey();
			return key ? fileTreeExpandedFor(key, source) : {};
		},
		/** Persist expanded dir keys (user toggles / open-to-path). */
		setFileTreeExpanded(source: FileTreeExpandSource, expanded: Record<string, boolean>) {
			const key = panelSessionKey();
			if (!key) return;
			setFileTreeExpandedForSession(key, source, expanded);
		},
		getFileTreeFilter(source: FileTreeExpandSource): string {
			const key = panelSessionKey();
			return key ? fileTreeFilterFor(key, source) : '';
		},
		setFileTreeFilter(source: FileTreeExpandSource, value: string) {
			const key = panelSessionKey();
			if (!key) return;
			setFileTreeFilterForSession(key, source, value);
		},
		/** Opens a workspace file in the panel for the active session. */
		openFilePreviewForActive(filePath: string) {
			const sessionId = panelSessionKey();
			if (!sessionId) return;
			this.openFilePreview(filePath, sessionId);
		},
		openGitDiffForActive(filePath: string) {
			const sessionId = panelSessionKey();
			if (!sessionId) return;
			this.openGitDiff(filePath, sessionId);
		},
		/** Opens a URL in the panel for the active session. */
		openWebPanelForActive(url: string) {
			const sessionId = panelSessionKey();
			if (!sessionId) return;
			this.openWebPanel(url, sessionId);
		}
	};
}

export const shellStore = createShellStore();
