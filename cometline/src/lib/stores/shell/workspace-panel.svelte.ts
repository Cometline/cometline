import { getActiveSessionId } from '#lib/active-session.js';
import {
	readWorkspacePanelTreeSource,
	writeWorkspacePanelTreeSource,
	type WorkspacePanelTreeSource
} from '#lib/features/workspace/workspace-panel-prefs.js';
import type {
	ContentSurface,
	SurfaceContent,
	SurfaceContentKey,
	TabSurfaceKey,
	WorkspacePanelState,
	WorkspacePanelSurface
} from '#lib/features/workspace/workspace-panel-state.js';
import { isWikiUiPath } from '#lib/wiki/paths.js';
import type { FileTreeExpandSource } from './file-tree.svelte';

export function syncWorkspacePanelOpen(open: boolean) {
	window.electronAPI?.setWorkspacePanelOpen?.(open);
}

export function ownerSurfaceForFile(filePath: string): FileTreeExpandSource {
	return isWikiUiPath(filePath) ? 'wiki' : 'workspace';
}

/** Per-session workspace panel state (visibility, surfaces, content, tabs) for the right slot. */
export function createWorkspacePanelStore() {
	/** Soft visibility for the web slot (false = hidden, state retained). */
	let workspacePanelVisibleBySession = $state<Record<string, boolean>>({});
	/** Per-surface open file / page / diff (independent lifecycles). */
	let contentBySessionSurface = $state<
		Record<string, Partial<Record<SurfaceContentKey, SurfaceContent>>>
	>({});
	let tabsBySession = $state<Record<string, Partial<Record<TabSurfaceKey, string[]>>>>({});
	let urlTabMetaBySession = $state<Record<string, WorkspacePanelState['urlTabMeta']>>({});
	let terminalPanelsBySession = $state<Record<string, boolean>>({});
	let workspacePanelSurfaceBySession = $state<Record<string, WorkspacePanelSurface>>({});
	/** Active inner surface while the outer slot is `web`. */
	let contentSurfaceBySession = $state<Record<string, ContentSurface>>({});
	/** Per-session Wiki/Workspace/Changes preference (last browse tab + history seed). */
	let browseSourceBySession = $state<Record<string, WorkspacePanelTreeSource>>({});

	function activeSurface(): WorkspacePanelSurface {
		const key = getActiveSessionId();
		if (!key) return 'web';
		return workspacePanelSurfaceBySession[key] ?? 'web';
	}

	function panelStateFor(sessionId: string): WorkspacePanelState {
		return {
			visible: workspacePanelVisibleBySession[sessionId] === true,
			surface: workspacePanelSurfaceBySession[sessionId] ?? 'web',
			terminalVisible: terminalPanelsBySession[sessionId] === true,
			contentSurface: contentSurfaceFor(sessionId),
			content: contentBySessionSurface[sessionId] ?? {},
			tabs: tabsBySession[sessionId] ?? {},
			urlTabMeta: urlTabMetaBySession[sessionId] ?? {}
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
		tabsBySession = {
			...tabsBySession,
			[sessionId]: state.tabs
		};
		urlTabMetaBySession = {
			...urlTabMetaBySession,
			[sessionId]: state.urlTabMeta
		};
	}

	function isOpenForActive() {
		const sessionId = getActiveSessionId();
		if (!sessionId) return false;
		if (activeSurface() === 'terminal') return terminalPanelsBySession[sessionId] === true;
		return workspacePanelVisibleBySession[sessionId] === true;
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
		return contentSurfaceBySession[sessionId] ?? defaultContentSurfaceFor(sessionId);
	}

	function contentFor(sessionId: string, surface: SurfaceContentKey): SurfaceContent | null {
		return contentBySessionSurface[sessionId]?.[surface] ?? null;
	}

	function clearContentForSession(sessionId: string) {
		if (!(sessionId in contentBySessionSurface)) return;
		const next = { ...contentBySessionSurface };
		delete next[sessionId];
		contentBySessionSurface = next;
		if (sessionId in tabsBySession) {
			const nextTabs = { ...tabsBySession };
			delete nextTabs[sessionId];
			tabsBySession = nextTabs;
		}
		if (sessionId in urlTabMetaBySession) {
			const nextMeta = { ...urlTabMetaBySession };
			delete nextMeta[sessionId];
			urlTabMetaBySession = nextMeta;
		}
	}

	function hasSession(sessionId: string): boolean {
		return sessionId in workspacePanelVisibleBySession;
	}

	function removeTerminal(sessionId: string) {
		const next = { ...terminalPanelsBySession };
		delete next[sessionId];
		terminalPanelsBySession = next;
	}

	return {
		activeSurface,
		panelStateFor,
		applyPanelState,
		isOpenForActive,
		syncOpenForActive() {
			syncWorkspacePanelOpen(isOpenForActive());
		},
		browseSourceFor,
		defaultContentSurfaceFor,
		contentSurfaceFor,
		setContentSurface(sessionId: string, surface: ContentSurface) {
			contentSurfaceBySession = { ...contentSurfaceBySession, [sessionId]: surface };
			if (surface === 'wiki' || surface === 'workspace' || surface === 'changes') {
				setBrowseSourceForSession(sessionId, surface);
			}
		},
		contentFor,
		setContentFor(
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
				if (surface === 'wiki' || surface === 'workspace' || surface === 'web-search') {
					const tabs = { ...(tabsBySession[sessionId] ?? {}) };
					delete tabs[surface];
					tabsBySession = { ...tabsBySession, [sessionId]: tabs };
				}
				if (surface === 'web-search') {
					const nextMeta = { ...urlTabMetaBySession };
					delete nextMeta[sessionId];
					urlTabMetaBySession = nextMeta;
				}
				return;
			}
			contentBySessionSurface = {
				...contentBySessionSurface,
				[sessionId]: {
					...prev,
					[surface]: content
				}
			};
		},
		activeSurfaceContent(sessionId: string): SurfaceContent | null {
			return contentFor(sessionId, contentSurfaceFor(sessionId));
		},
		hasSession,
		isVisible(sessionId: string) {
			return workspacePanelVisibleBySession[sessionId] === true;
		},
		isTerminalVisible(sessionId: string) {
			return terminalPanelsBySession[sessionId] === true;
		},
		surfaceFor(sessionId: string): WorkspacePanelSurface | undefined {
			return workspacePanelSurfaceBySession[sessionId];
		},
		/** Switch the outer slot to `web` and mark the session's panel visible. */
		showWebSurface(sessionId: string) {
			workspacePanelSurfaceBySession = {
				...workspacePanelSurfaceBySession,
				[sessionId]: 'web'
			};
			workspacePanelVisibleBySession = {
				...workspacePanelVisibleBySession,
				[sessionId]: true
			};
		},
		showTerminalSurface(sessionId: string) {
			terminalPanelsBySession = { ...terminalPanelsBySession, [sessionId]: true };
			workspacePanelSurfaceBySession = {
				...workspacePanelSurfaceBySession,
				[sessionId]: 'terminal'
			};
		},
		/** Hide the web slot but keep its surfaces, content, and history. */
		softHide(sessionId: string) {
			if (!hasSession(sessionId)) return;
			workspacePanelVisibleBySession = {
				...workspacePanelVisibleBySession,
				[sessionId]: false
			};
		},
		removeTerminal,
		clearForSession(sessionId: string) {
			if (sessionId in workspacePanelVisibleBySession) {
				const next = { ...workspacePanelVisibleBySession };
				delete next[sessionId];
				workspacePanelVisibleBySession = next;
			}
			clearContentForSession(sessionId);
			removeTerminal(sessionId);
			const nextSurfaces = { ...workspacePanelSurfaceBySession };
			delete nextSurfaces[sessionId];
			workspacePanelSurfaceBySession = nextSurfaces;
			if (sessionId in contentSurfaceBySession) {
				const next = { ...contentSurfaceBySession };
				delete next[sessionId];
				contentSurfaceBySession = next;
			}
		},
		/** Live web guests follow tab ownership, not the currently selected session/surface. */
		webTabs() {
			return Object.entries(tabsBySession).flatMap(([sessionId, tabs]) =>
				(tabs['web-search'] ?? []).map((id) => ({
					key: `${sessionId}:${id}`,
					sessionId,
					id,
					...(urlTabMetaBySession[sessionId]?.[id] ?? { url: id, title: '' })
				}))
			);
		}
	};
}

export type WorkspacePanelStore = ReturnType<typeof createWorkspacePanelStore>;
