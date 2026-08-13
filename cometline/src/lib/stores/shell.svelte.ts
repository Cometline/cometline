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
	readWorkspacePanelTreeSource,
	writeWorkspacePanelTreeSource,
	type WorkspacePanelTreeSource
} from '$lib/workspace/workspace-panel-prefs';
import { dirKeysToExpandForPaths } from '$lib/workspace/file-tree';
import {
	clearFileReveal,
	closeWorkspacePanel as closeWorkspacePanelState,
	openWorkspacePanelFile,
	replacesActiveFile,
	type ContentSurface,
	type FileRevealRange,
	type SurfaceContent,
	type SurfaceContentKey,
	type WorkspacePanelState,
	type WorkspacePanelSurface
} from '$lib/workspace/workspace-panel-state';
import { isWikiUiPath, toWikiRelative } from '$lib/wiki/paths';

export type WorkspacePanelMode = 'url' | 'file' | 'git-diff';
/** File-tree sources that keep an expansion map (not Changes). */
export type FileTreeExpandSource = 'wiki' | 'workspace';
/** Surfaces that can own independent open content. */
export type {
	ContentSurface,
	SurfaceContent,
	SurfaceContentKey,
	WorkspacePanelSurface
} from '$lib/workspace/workspace-panel-state';

export type FocusedPane = 'chat' | 'web' | 'terminal';

export type ComposerFocusRequest = {
	id: number;
	sessionId: string | null;
};

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
	let workspacePanelVisibleBySession = $state<Record<string, boolean>>({});
	/** Per-surface open file / page / diff (independent lifecycles). */
	let contentBySessionSurface = $state<
		Record<string, Partial<Record<SurfaceContentKey, SurfaceContent>>>
	>({});
	let terminalPanelsBySession = $state<Record<string, boolean>>({});
	let workspacePanelSurfaceBySession = $state<Record<string, WorkspacePanelSurface>>({});
	/** Active inner surface while the outer slot is `web`. */
	let contentSurfaceBySession = $state<Record<string, ContentSurface>>({});
	let webContextsBySession = $state<Record<string, PendingWebContext[]>>({});
	/** Per-surface Back/Forward stacks. */
	let panelHistoryBySessionSurface = $state<
		Record<string, Partial<Record<SurfaceContentKey, PanelHistoryState>>>
	>({});
	/** Per-session Wiki/Workspace/Changes preference (last browse tab + history seed). */
	let browseSourceBySession = $state<Record<string, WorkspacePanelTreeSource>>({});
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
	let requestWorkspacePanelLeave: (() => boolean | Promise<boolean>) | null = null;
	let focusedPane = $state<FocusedPane>('chat');
	let addressBarFocusRequestId = $state(0);
	let fileTreeFilterFocusRequestId = $state(0);
	let gitChangesOpenRequestId = $state(0);
	let terminalFocusRequestId = $state(0);
	/** Last focus target while the web slot is open: filter vs web address. */
	let lastWorkspacePanelFocusTarget = $state<'filter' | 'address'>('filter');
	let composerFocusRequest = $state<ComposerFocusRequest>({ id: 0, sessionId: null });
	let sessionFindRequestId = $state(0);

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

	function panelStateFor(sessionId: string): WorkspacePanelState {
		return {
			visible: workspacePanelVisibleBySession[sessionId] === true,
			surface: workspacePanelSurfaceBySession[sessionId] ?? 'web',
			terminalVisible: terminalPanelsBySession[sessionId] === true,
			contentSurface: contentSurfaceFor(sessionId),
			content: contentBySessionSurface[sessionId] ?? {}
		};
	}

	function applyPanelState(sessionId: string, state: WorkspacePanelState) {
		workspacePanelVisibleBySession = {
			...workspacePanelVisibleBySession,
			[sessionId]: state.visible
		};
		terminalPanelsBySession = {
			...terminalPanelsBySession,
			[sessionId]: state.terminalVisible
		};
		workspacePanelSurfaceBySession = {
			...workspacePanelSurfaceBySession,
			[sessionId]: state.surface
		};
		contentSurfaceBySession = {
			...contentSurfaceBySession,
			[sessionId]: state.contentSurface
		};
		contentBySessionSurface = {
			...contentBySessionSurface,
			[sessionId]: state.content
		};
	}

	function workspacePanelOpenForActiveSession() {
		const sessionId = panelSessionKey();
		if (!sessionId) return false;
		if (activeWorkspacePanelSurface() === 'terminal')
			return terminalPanelsBySession[sessionId] === true;
		return workspacePanelVisibleBySession[sessionId] === true;
	}

	function syncWorkspacePanelOpen(open: boolean) {
		window.electronAPI?.setWorkspacePanelOpen?.(open);
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

	function browseSourceFor(sessionId: string): WorkspacePanelTreeSource {
		return browseSourceBySession[sessionId] ?? readWorkspacePanelTreeSource();
	}

	function setBrowseSourceForSession(sessionId: string, source: WorkspacePanelTreeSource) {
		browseSourceBySession = { ...browseSourceBySession, [sessionId]: source };
		writeWorkspacePanelTreeSource(source);
	}

	function defaultContentSurfaceFor(sessionId: string): ContentSurface {
		const source = browseSourceFor(sessionId);
		if (source === 'workspace' || source === 'changes') return source;
		return 'wiki';
	}

	function contentSurfaceFor(sessionId: string): ContentSurface {
		const surface = contentSurfaceBySession[sessionId] ?? defaultContentSurfaceFor(sessionId);
		// Migrate legacy 'content' if any stale value lingered in memory during hot reload.
		if ((surface as string) === 'content') return defaultContentSurfaceFor(sessionId);
		return surface;
	}

	function setContentSurfaceForSession(sessionId: string, surface: ContentSurface) {
		contentSurfaceBySession = { ...contentSurfaceBySession, [sessionId]: surface };
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
	function ensureWorkspacePanelVisible(sessionId: string) {
		workspacePanelVisibleBySession = { ...workspacePanelVisibleBySession, [sessionId]: true };
	}

	function hasWorkspacePanelSession(sessionId: string): boolean {
		return sessionId in workspacePanelVisibleBySession;
	}

	function ownerSurfaceForFile(filePath: string): FileTreeExpandSource {
		return isWikiUiPath(filePath) ? 'wiki' : 'workspace';
	}

	function activeSurfaceContent(sessionId: string): SurfaceContent | null {
		return contentFor(sessionId, contentSurfaceFor(sessionId));
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

	function clearContentSurfaceForSession(sessionId: string) {
		if (!(sessionId in contentSurfaceBySession)) return;
		const next = { ...contentSurfaceBySession };
		delete next[sessionId];
		contentSurfaceBySession = next;
	}

	function clearWorkspacePanelVisibleForSession(sessionId: string) {
		if (!(sessionId in workspacePanelVisibleBySession)) return;
		const next = { ...workspacePanelVisibleBySession };
		delete next[sessionId];
		workspacePanelVisibleBySession = next;
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
		const seed: WorkspacePanelTreeSource =
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
			ensureWorkspacePanelVisible(sessionId);
			if (entry.kind === 'browse') {
				setContentSurfaceForSession(sessionId, entry.source);
				setContentFor(sessionId, entry.source, null);
			} else if (entry.kind === 'file') {
				const owner = ownerSurfaceForFile(entry.path);
				setContentSurfaceForSession(sessionId, owner);
				setContentFor(sessionId, owner, { mode: 'file', filePath: entry.path });
			} else if (entry.kind === 'git-diff') {
				setContentSurfaceForSession(sessionId, 'changes');
				setContentFor(sessionId, 'changes', { mode: 'git-diff', filePath: entry.path });
			} else if (entry.url) {
				setContentSurfaceForSession(sessionId, 'web-search');
				setContentFor(sessionId, 'web-search', { mode: 'url', url: entry.url });
			} else {
				// Empty URL = empty web-search (or cleared content on that stack).
				setContentSurfaceForSession(sessionId, surface === 'web-search' ? 'web-search' : surface);
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
		source: WorkspacePanelTreeSource,
		options: { recordHistory?: boolean } = {}
	) {
		const prevSurface = contentSurfaceFor(sessionId);
		const hadSession = hasWorkspacePanelSession(sessionId);

		setContentSurfaceForSession(sessionId, source);
		workspacePanelSurfaceBySession = {
			...workspacePanelSurfaceBySession,
			[sessionId]: 'web'
		};
		ensureWorkspacePanelVisible(sessionId);

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

	function openBrowseSurface(sessionId: string, source: WorkspacePanelTreeSource) {
		setContentSurfaceForSession(sessionId, source);
		workspacePanelSurfaceBySession = {
			...workspacePanelSurfaceBySession,
			[sessionId]: 'web'
		};
		ensureWorkspacePanelVisible(sessionId);
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
		get workspacePanelMode(): WorkspacePanelMode | null {
			const key = panelSessionKey();
			if (!key) return null;
			return activeSurfaceContent(key)?.mode ?? null;
		},
		get workspacePanelUrl() {
			const key = panelSessionKey();
			if (!key) return null;
			const content = activeSurfaceContent(key);
			return content?.mode === 'url' ? content.url : null;
		},
		get workspacePanelFilePath() {
			const key = panelSessionKey();
			if (!key) return null;
			const content = activeSurfaceContent(key);
			return content?.mode === 'file' ? content.filePath : null;
		},
		get workspacePanelGitDiffPath() {
			const key = panelSessionKey();
			if (!key) return null;
			const content = activeSurfaceContent(key);
			return content?.mode === 'git-diff' ? content.filePath : null;
		},
		get workspacePanelBrowseSource(): WorkspacePanelTreeSource {
			const key = panelSessionKey();
			return key ? browseSourceFor(key) : readWorkspacePanelTreeSource();
		},
		/** Active inner surface for the web right-sidebar stack. */
		get contentSurface(): ContentSurface {
			const key = panelSessionKey();
			return key ? contentSurfaceFor(key) : 'wiki';
		},
		get pendingWebContexts(): PendingWebContext[] {
			const key = panelSessionKey();
			return key ? (webContextsBySession[key] ?? []) : [];
		},
		get hasWorkspacePanelForSession() {
			const key = panelSessionKey();
			return Boolean(key && hasWorkspacePanelSession(key));
		},
		/** Storage key for the active session's panel, or null when none is open. */
		get workspacePanelSessionKey() {
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
		get lastWorkspacePanelFocusTarget(): 'filter' | 'address' {
			return lastWorkspacePanelFocusTarget;
		},
		get terminalFocusRequestId() {
			return terminalFocusRequestId;
		},
		get composerFocusRequest() {
			return composerFocusRequest;
		},
		get sessionFindRequestId() {
			return sessionFindRequestId;
		},
		get canPanelHistoryBack() {
			const key = panelSessionKey();
			if (!key) return false;
			return historyCanGoBack(historyFor(key, contentSurfaceFor(key)));
		},
		get canPanelHistoryForward() {
			const key = panelSessionKey();
			if (!key) return false;
			return historyCanGoForward(historyFor(key, contentSurfaceFor(key)));
		},
		/** True when a browse layer is active and that surface has no open content. */
		get workspacePanelBrowseOpen() {
			const key = panelSessionKey();
			if (!key || workspacePanelVisibleBySession[key] !== true) return false;
			const surface = contentSurfaceFor(key);
			if (surface !== 'wiki' && surface !== 'workspace' && surface !== 'changes') return false;
			return contentFor(key, surface) === null;
		},
		get workspacePanelGitDiffOpen() {
			const key = panelSessionKey();
			if (!key || workspacePanelVisibleBySession[key] !== true) return false;
			const content = contentFor(key, 'changes');
			return contentSurfaceFor(key) === 'changes' && content?.mode === 'git-diff';
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
			const nextContext: WebContext = {
				...context,
				content: context.content.trim().slice(0, 50000)
			};
			if (
				nextContext.kind === 'message' &&
				existing.some(
					(item) =>
						item.kind === 'message' &&
						item.source === nextContext.source &&
						item.content.replace(/\s+/g, ' ').trim() ===
							nextContext.content.replace(/\s+/g, ' ').trim()
				)
			) {
				return;
			}
			let next = existing;
			if (context.kind === 'page') {
				next = existing.filter(
					(item) => !(isPendingPageContext(item) && item.source === context.source)
				);
			}
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
		registerWorkspacePanelLeaveGuard(guard: () => boolean | Promise<boolean>) {
			requestWorkspacePanelLeave = guard;
			return () => {
				if (requestWorkspacePanelLeave === guard) requestWorkspacePanelLeave = null;
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
		requestComposerFocus(sessionId = activeSessionId()) {
			focusedPane = 'chat';
			composerFocusRequest = { id: composerFocusRequest.id + 1, sessionId };
		},
		requestSessionFind() {
			focusedPane = 'chat';
			sessionFindRequestId += 1;
		},
		onActiveSessionChange() {
			focusedPane = 'chat';
			syncWorkspacePanelOpenForActiveSession();
		},
		openWorkspacePanelUrl(url: string, sessionId: string) {
			workspacePanelSurfaceBySession = {
				...workspacePanelSurfaceBySession,
				[sessionId]: 'web'
			};
			ensureWorkspacePanelVisible(sessionId);
			if (url) {
				setContentSurfaceForSession(sessionId, 'web-search');
				setContentFor(sessionId, 'web-search', { mode: 'url', url });
				recordPanelHistory(sessionId, 'web-search', { kind: 'url', url });
			} else {
				const surface = defaultContentSurfaceFor(sessionId);
				setContentSurfaceForSession(sessionId, surface);
				recordPanelHistory(sessionId, surface, {
					kind: 'browse',
					source: surface === 'web-search' ? 'wiki' : surface
				});
			}
			focusedPane = 'web';
			syncWorkspacePanelOpen(true);
		},
		async openFilePreview(
			filePath: string,
			sessionId: string,
			reveal?: { startLine: number; endLine: number } | null
		) {
			const owner = ownerSurfaceForFile(filePath);
			const current = panelStateFor(sessionId);
			const nextContent: SurfaceContent = {
				mode: 'file',
				filePath,
				...(reveal
					? { startLine: reveal.startLine, endLine: reveal.endLine }
					: {})
			};
			if (
				replacesActiveFile(current, owner, nextContent) &&
				requestWorkspacePanelLeave &&
				!(await requestWorkspacePanelLeave())
			) {
				return false;
			}

			const relative = isWikiUiPath(filePath) ? toWikiRelative(filePath) : filePath;
			expandFileTreeToRelativePath(sessionId, owner, relative);
			applyPanelState(sessionId, openWorkspacePanelFile(current, owner, filePath, reveal));
			recordPanelHistory(sessionId, owner, { kind: 'file', path: filePath });
			focusedPane = 'web';
			syncWorkspacePanelOpen(true);
			return true;
		},
		clearFileRevealForActive() {
			const sessionId = panelSessionKey();
			if (!sessionId) return;
			const owner = contentSurfaceFor(sessionId);
			if (owner !== 'wiki' && owner !== 'workspace') return;
			const current = panelStateFor(sessionId);
			applyPanelState(sessionId, clearFileReveal(current, owner));
		},
		openGitDiff(filePath: string, sessionId: string) {
			workspacePanelSurfaceBySession = {
				...workspacePanelSurfaceBySession,
				[sessionId]: 'web'
			};
			ensureWorkspacePanelVisible(sessionId);
			setContentSurfaceForSession(sessionId, 'changes');
			setContentFor(sessionId, 'changes', { mode: 'git-diff', filePath });
			recordPanelHistory(sessionId, 'changes', { kind: 'git-diff', path: filePath });
			focusedPane = 'web';
			syncWorkspacePanelOpen(true);
		},
		openWorkspacePanelBrowse() {
			const sessionId = panelSessionKey();
			if (!sessionId) return;
			openBrowseSurface(sessionId, browseSourceFor(sessionId));
			this.requestFileTreeFilterFocus();
		},
		/** Switch Wiki / Workspace / Changes — covers other layers; does not destroy them. */
		setWorkspacePanelBrowseSource(source: WorkspacePanelTreeSource) {
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
				lastWorkspacePanelFocusTarget = 'filter';
				focusedPane = 'web';
			}
		},
		navigateWorkspacePanel(url: string) {
			const sessionId = panelSessionKey();
			if (!sessionId) return;
			workspacePanelSurfaceBySession = {
				...workspacePanelSurfaceBySession,
				[sessionId]: 'web'
			};
			ensureWorkspacePanelVisible(sessionId);
			if (url) {
				setContentSurfaceForSession(sessionId, 'web-search');
				setContentFor(sessionId, 'web-search', { mode: 'url', url });
				recordPanelHistory(sessionId, 'web-search', { kind: 'url', url });
				focusedPane = 'web';
				syncWorkspacePanelOpen(true);
				this.requestAddressBarFocus();
				return;
			}
			const surface = defaultContentSurfaceFor(sessionId);
			setContentSurfaceForSession(sessionId, surface);
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
			const surface = contentSurfaceFor(sessionId);
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
			const surface = contentSurfaceFor(sessionId);
			const prev = historyFor(sessionId, surface);
			if (!historyCanGoForward(prev)) return false;
			const next = historyGoForward(prev);
			setHistoryFor(sessionId, surface, next);
			const entry = currentEntry(next);
			if (!entry) return false;
			applyPanelHistoryEntry(sessionId, surface, entry);
			return true;
		},
		ensureWorkspacePanelVisible() {
			const sessionId = panelSessionKey();
			if (!sessionId) return null;
			if (!hasWorkspacePanelSession(sessionId)) return null;
			workspacePanelSurfaceBySession = {
				...workspacePanelSurfaceBySession,
				[sessionId]: 'web'
			};
			ensureWorkspacePanelVisible(sessionId);
			focusedPane = 'web';
			syncWorkspacePanelOpen(true);
			return true;
		},
		requestFileTreeFilterFocus() {
			const sessionId = panelSessionKey();
			if (!sessionId) return;
			if (!hasWorkspacePanelSession(sessionId)) {
				openBrowseSurface(sessionId, browseSourceFor(sessionId));
			} else {
				this.ensureWorkspacePanelVisible();
			}
			lastWorkspacePanelFocusTarget = 'filter';
			fileTreeFilterFocusRequestId += 1;
		},
		requestAddressBarFocus() {
			const sessionId = panelSessionKey();
			if (!sessionId) return;
			if (!hasWorkspacePanelSession(sessionId)) {
				openBrowseSurface(sessionId, browseSourceFor(sessionId));
			} else {
				this.ensureWorkspacePanelVisible();
			}
			lastWorkspacePanelFocusTarget = 'address';
			addressBarFocusRequestId += 1;
		},
		/** ⌘O: open the right sidebar on web search (address bar). */
		openWebSearchPanel() {
			const sessionId = panelSessionKey();
			if (!sessionId) return;
			if (!hasWorkspacePanelSession(sessionId)) {
				ensureWorkspacePanelVisible(sessionId);
			}
			workspacePanelSurfaceBySession = {
				...workspacePanelSurfaceBySession,
				[sessionId]: 'web'
			};
			ensureWorkspacePanelVisible(sessionId);
			setContentSurfaceForSession(sessionId, 'web-search');
			focusedPane = 'web';
			syncWorkspacePanelOpen(true);
			this.requestAddressBarFocus();
		},
		/** Open the workspace panel browse surface on the Git Changes tab (⌘⇧G). */
		openGitChangesPanel() {
			this.setWorkspacePanelBrowseSource('changes');
			gitChangesOpenRequestId += 1;
		},
		/**
		 * Toggle the right workspace panel.
		 * - First open for a session → workspace file tree (⌘L-like).
		 * - Soft-hidden session → re-show last surface/content.
		 * - Currently open → soft-hide (no content-dot walk; use closeWorkspacePanel / ⌘W for that).
		 */
		toggleWorkspacePanel() {
			const sessionId = panelSessionKey();
			if (!sessionId) return;

			// Soft-hide the open panel without dismissing open files/pages.
			if (workspacePanelOpenForActiveSession()) {
				if (activeWorkspacePanelSurface() === 'terminal') {
					const nextTerminal = { ...terminalPanelsBySession };
					delete nextTerminal[sessionId];
					terminalPanelsBySession = nextTerminal;
				}
				if (hasWorkspacePanelSession(sessionId)) {
					workspacePanelVisibleBySession = {
						...workspacePanelVisibleBySession,
						[sessionId]: false
					};
				}
				focusedPane = 'chat';
				this.requestComposerFocus();
				syncWorkspacePanelOpen(false);
				return;
			}

			// Re-show a previously soft-hidden panel with its last surface.
			if (hasWorkspacePanelSession(sessionId)) {
				workspacePanelSurfaceBySession = {
					...workspacePanelSurfaceBySession,
					[sessionId]: 'web'
				};
				ensureWorkspacePanelVisible(sessionId);
				focusedPane = 'web';
				syncWorkspacePanelOpen(true);
				const surface = contentSurfaceFor(sessionId);
				const content = contentFor(sessionId, surface);
				if (surface === 'web-search' || content?.mode === 'url') {
					this.requestAddressBarFocus();
				} else if (
					(surface === 'wiki' || surface === 'workspace') &&
					content === null
				) {
					this.requestFileTreeFilterFocus();
				}
				return;
			}

			// First open for this session: coding-first workspace file tree.
			this.setWorkspacePanelBrowseSource('workspace');
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
			const current = panelStateFor(sessionId);
			const next = closeWorkspacePanelState(current);
			if (current.surface === 'terminal') {
				// Terminal soft-hide only — leave web surface content (dots) intact.
				applyPanelState(sessionId, next);
				this.requestComposerFocus();
				syncWorkspacePanelOpen(false);
				return;
			}

			const surface = current.contentSurface;
			const content = current.content[surface] ?? null;
			if (content) {
				// 1) Dismiss open file/page on this surface → back to browse/search.
				applyPanelState(sessionId, next);
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
			if (next.contentSurface !== surface) {
				applyPanelState(sessionId, next);
				focusedPane = 'web';
				syncWorkspacePanelOpen(true);
				return;
			}

			// 3) No remaining content dots — soft-hide sidebar (keep trees/history).
			applyPanelState(sessionId, next);
			this.requestComposerFocus();
			syncWorkspacePanelOpen(false);
		},
		closeTerminalPanelForSession(sessionId: string) {
			const next = { ...terminalPanelsBySession };
			delete next[sessionId];
			terminalPanelsBySession = next;
			if (activeSessionId() !== sessionId || activeWorkspacePanelSurface() !== 'terminal') return;
			this.requestComposerFocus();
			syncWorkspacePanelOpen(false);
		},
		clearWorkspacePanelForSession(sessionId: string) {
			clearWorkspacePanelVisibleForSession(sessionId);
			clearContentForSession(sessionId);
			const nextTerminalPanels = { ...terminalPanelsBySession };
			delete nextTerminalPanels[sessionId];
			terminalPanelsBySession = nextTerminalPanels;
			const nextSurfaces = { ...workspacePanelSurfaceBySession };
			delete nextSurfaces[sessionId];
			workspacePanelSurfaceBySession = nextSurfaces;
			clearContentSurfaceForSession(sessionId);
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
		openFilePreviewForActive(filePath: string, reveal?: FileRevealRange | null) {
			const sessionId = panelSessionKey();
			if (!sessionId) return Promise.resolve(false);
			return this.openFilePreview(filePath, sessionId, reveal);
		},
		openGitDiffForActive(filePath: string) {
			const sessionId = panelSessionKey();
			if (!sessionId) return;
			this.openGitDiff(filePath, sessionId);
		},
		/** Opens a URL in the panel for the active session. */
		openWorkspacePanelUrlForActive(url: string) {
			const sessionId = panelSessionKey();
			if (!sessionId) return;
			this.openWorkspacePanelUrl(url, sessionId);
		}
	};
}

export const shellStore = createShellStore();
