import { getActiveSessionId } from '$lib/active-session';
import type { PanelHistoryEntry } from '$lib/features/workspace/panel-history';
import type { WorkspacePanelTreeSource } from '$lib/features/workspace/workspace-panel-prefs';
import {
	closeWorkspacePanel as closeWorkspacePanelState,
	openWorkspacePanelFile,
	openWorkspacePanelUrl as openWorkspacePanelUrlState,
	type SurfaceContentKey
} from '$lib/features/workspace/workspace-panel-state';
import type { FileTreeStore } from './file-tree.svelte';
import type { ShellFocusStore } from './focus.svelte';
import type { PanelHistoryStore } from './panel-history.svelte';
import type { WebContextStore } from './web-context.svelte';
import {
	ownerSurfaceForFile,
	syncWorkspacePanelOpen,
	type WorkspacePanelStore
} from './workspace-panel.svelte';

type PanelNavigationDeps = {
	panel: WorkspacePanelStore;
	history: PanelHistoryStore;
	focus: ShellFocusStore;
	fileTree: FileTreeStore;
	webContext: WebContextStore;
};

/** Browse surfaces, Back/Forward, focus requests, and show/hide/close of the workspace panel. */
export function createPanelNavigation({
	panel,
	history,
	focus,
	fileTree,
	webContext
}: PanelNavigationDeps) {
	function applyPanelHistoryEntry(
		sessionId: string,
		surface: SurfaceContentKey,
		entry: PanelHistoryEntry
	) {
		history.whileApplying(() => {
			panel.showWebSurface(sessionId);
			if (entry.kind === 'browse') {
				panel.setContentSurface(sessionId, entry.source);
				panel.setContentFor(sessionId, entry.source, null);
			} else if (entry.kind === 'file') {
				const owner = ownerSurfaceForFile(entry.path);
				panel.applyPanelState(
					sessionId,
					openWorkspacePanelFile(panel.panelStateFor(sessionId), owner, entry.path)
				);
			} else if (entry.kind === 'git-diff') {
				panel.setContentSurface(sessionId, 'changes');
				panel.setContentFor(sessionId, 'changes', {
					mode: 'git-diff',
					filePath: entry.path
				});
			} else if (entry.url) {
				panel.applyPanelState(
					sessionId,
					openWorkspacePanelUrlState(panel.panelStateFor(sessionId), entry.url)
				);
			} else {
				// Empty URL = empty web-search (or cleared content on that stack).
				panel.setContentSurface(
					sessionId,
					surface === 'web-search' ? 'web-search' : surface
				);
				if (surface === 'web-search') {
					panel.setContentFor(sessionId, 'web-search', null);
				}
			}
			focus.setFocusedPane('web');
			syncWorkspacePanelOpen(true);
		});
	}

	/**
	 * Activate a browse/search surface without clearing its (or others') content.
	 */
	function promoteBrowseSurface(
		sessionId: string,
		source: WorkspacePanelTreeSource,
		options: { recordHistory?: boolean } = {}
	) {
		const prevSurface = panel.contentSurfaceFor(sessionId);
		const hadSession = panel.hasSession(sessionId);

		panel.setContentSurface(sessionId, source);
		panel.showWebSurface(sessionId);

		if (options.recordHistory !== false) {
			const content = panel.contentFor(sessionId, source);
			// Only record browse when landing on a surface with no content (tree visible).
			if (!content && (!hadSession || prevSurface !== source)) {
				history.record(sessionId, source, { kind: 'browse', source });
			}
		}
		focus.setFocusedPane('web');
		syncWorkspacePanelOpen(true);
	}

	function openBrowseSurface(sessionId: string, source: WorkspacePanelTreeSource) {
		panel.setContentSurface(sessionId, source);
		panel.showWebSurface(sessionId);
		history.record(sessionId, source, { kind: 'browse', source });
		focus.setFocusedPane('web');
		syncWorkspacePanelOpen(true);
	}

	function ensureWorkspacePanelVisible() {
		const sessionId = getActiveSessionId();
		if (!sessionId) return null;
		if (!panel.hasSession(sessionId)) return null;
		panel.showWebSurface(sessionId);
		focus.setFocusedPane('web');
		syncWorkspacePanelOpen(true);
		return true;
	}

	function requestFileTreeFilterFocus() {
		const sessionId = getActiveSessionId();
		if (!sessionId) return;
		if (!panel.hasSession(sessionId)) {
			openBrowseSurface(sessionId, panel.browseSourceFor(sessionId));
		} else {
			ensureWorkspacePanelVisible();
		}
		focus.bumpFileTreeFilterFocus();
	}

	function requestAddressBarFocus() {
		const sessionId = getActiveSessionId();
		if (!sessionId) return;
		if (!panel.hasSession(sessionId)) {
			openBrowseSurface(sessionId, panel.browseSourceFor(sessionId));
		} else {
			ensureWorkspacePanelVisible();
		}
		focus.bumpAddressBarFocus();
	}

	/** Switch Wiki / Workspace / Changes — covers other layers; does not destroy them. */
	function setWorkspacePanelBrowseSource(source: WorkspacePanelTreeSource) {
		const sessionId = getActiveSessionId();
		if (!sessionId) return;
		promoteBrowseSurface(sessionId, source);
		const content = panel.contentFor(sessionId, source);
		if (content) {
			// Surface already has open content — keep it visible, no filter steal.
			focus.setFocusedPane('web');
			return;
		}
		if (source === 'wiki' || source === 'workspace') {
			requestFileTreeFilterFocus();
		} else {
			focus.setLastWorkspacePanelFocusTarget('filter');
			focus.setFocusedPane('web');
		}
	}

	function openGitDiff(filePath: string, sessionId: string) {
		panel.showWebSurface(sessionId);
		panel.setContentSurface(sessionId, 'changes');
		panel.setContentFor(sessionId, 'changes', { mode: 'git-diff', filePath });
		history.record(sessionId, 'changes', { kind: 'git-diff', path: filePath });
		focus.setFocusedPane('web');
		syncWorkspacePanelOpen(true);
	}

	function stepHistory(direction: 'back' | 'forward') {
		const sessionId = getActiveSessionId();
		if (!sessionId) return false;
		const surface = panel.contentSurfaceFor(sessionId);
		const entry =
			direction === 'back'
				? history.stepBack(sessionId, surface)
				: history.stepForward(sessionId, surface);
		if (!entry) return false;
		applyPanelHistoryEntry(sessionId, surface, entry);
		return true;
	}

	return {
		ensureWorkspacePanelVisible,
		requestFileTreeFilterFocus,
		requestAddressBarFocus,
		setWorkspacePanelBrowseSource,
		openGitDiff,
		openGitDiffForActive(filePath: string) {
			const sessionId = getActiveSessionId();
			if (!sessionId) return;
			openGitDiff(filePath, sessionId);
		},
		openWorkspacePanelBrowse() {
			const sessionId = getActiveSessionId();
			if (!sessionId) return;
			openBrowseSurface(sessionId, panel.browseSourceFor(sessionId));
			requestFileTreeFilterFocus();
		},
		panelHistoryBack() {
			return stepHistory('back');
		},
		panelHistoryForward() {
			return stepHistory('forward');
		},
		/** Open the workspace panel browse surface on the Git Changes tab (⌘⇧G). */
		openGitChangesPanel() {
			setWorkspacePanelBrowseSource('changes');
			focus.bumpGitChangesOpen();
		},
		/**
		 * Toggle the right workspace panel.
		 * - First open for a session → workspace file tree (⌘L-like).
		 * - Soft-hidden session → re-show last surface/content.
		 * - Currently open → soft-hide (no content-dot walk; use closeWorkspacePanel / ⌘W for that).
		 */
		toggleWorkspacePanel() {
			const sessionId = getActiveSessionId();
			if (!sessionId) return;

			// Soft-hide the open panel without dismissing open files/pages.
			if (panel.isOpenForActive()) {
				if (panel.activeSurface() === 'terminal') panel.removeTerminal(sessionId);
				panel.softHide(sessionId);
				focus.setFocusedPane('chat');
				focus.requestComposerFocus();
				syncWorkspacePanelOpen(false);
				return;
			}

			// Re-show a previously soft-hidden panel with its last surface.
			if (panel.hasSession(sessionId)) {
				panel.showWebSurface(sessionId);
				focus.setFocusedPane('web');
				syncWorkspacePanelOpen(true);
				const surface = panel.contentSurfaceFor(sessionId);
				const content = panel.contentFor(sessionId, surface);
				if (surface === 'web-search' || content?.mode === 'url') {
					requestAddressBarFocus();
				} else if ((surface === 'wiki' || surface === 'workspace') && content === null) {
					requestFileTreeFilterFocus();
				}
				return;
			}

			// First open for this session: coding-first workspace file tree.
			setWorkspacePanelBrowseSource('workspace');
		},
		closeWorkspacePanel() {
			const sessionId = getActiveSessionId();
			if (!sessionId) {
				focus.requestComposerFocus();
				syncWorkspacePanelOpen(false);
				return;
			}
			const current = panel.panelStateFor(sessionId);
			const next = closeWorkspacePanelState(current);
			if (current.surface === 'terminal') {
				// Terminal soft-hide only — leave web surface content (dots) intact.
				panel.applyPanelState(sessionId, next);
				focus.requestComposerFocus();
				syncWorkspacePanelOpen(false);
				return;
			}

			const surface = current.contentSurface;
			const content = current.content[surface] ?? null;
			if (content) {
				// 1) Close active file tab, or dismiss page → browse/search when last tab gone.
				panel.applyPanelState(sessionId, next);
				const stillFile = next.content[surface]?.mode === 'file';
				const stillUrl = next.content[surface]?.mode === 'url';
				if (stillFile || stillUrl) {
					focus.setFocusedPane('web');
					return;
				}
				if (surface === 'wiki' || surface === 'workspace') {
					history.record(sessionId, surface, { kind: 'browse', source: surface });
					requestFileTreeFilterFocus();
				} else if (surface === 'web-search') {
					history.record(sessionId, 'web-search', { kind: 'url', url: '' });
					requestAddressBarFocus();
				} else {
					history.record(sessionId, 'changes', { kind: 'browse', source: 'changes' });
					focus.setFocusedPane('web');
				}
				return;
			}

			// 2) Surface already at browse/search — jump to another surface that still has a content dot.
			if (next.contentSurface !== surface) {
				panel.applyPanelState(sessionId, next);
				focus.setFocusedPane('web');
				syncWorkspacePanelOpen(true);
				return;
			}

			// 3) No remaining content dots — soft-hide sidebar (keep trees/history).
			panel.applyPanelState(sessionId, next);
			focus.requestComposerFocus();
			syncWorkspacePanelOpen(false);
		},
		clearWorkspacePanelForSession(sessionId: string) {
			panel.clearForSession(sessionId);
			webContext.clearForSession(sessionId);
			history.clearForSession(sessionId);
			fileTree.clearForSession(sessionId);
			if (getActiveSessionId() === sessionId) {
				focus.requestComposerFocus();
				syncWorkspacePanelOpen(false);
			}
		}
	};
}

export type PanelNavigation = ReturnType<typeof createPanelNavigation>;
