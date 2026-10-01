import { getActiveSessionId } from '$lib/active-session';
import {
	activateWorkspacePanelFileTab,
	clearFileReveal,
	closeWorkspacePanelFileTab,
	openWorkspacePanelFile,
	replacesActiveFile,
	type FileRevealRange,
	type SurfaceContent
} from '$lib/features/workspace/workspace-panel-state';
import { isWikiUiPath, toWikiRelative } from '$lib/wiki/paths';
import type { FileTreeStore } from './file-tree.svelte';
import type { ShellFocusStore } from './focus.svelte';
import type { PanelHistoryStore } from './panel-history.svelte';
import type { PanelNavigation } from './panel-navigation.svelte';
import {
	ownerSurfaceForFile,
	syncWorkspacePanelOpen,
	type WorkspacePanelStore
} from './workspace-panel.svelte';

type FilePreviewDeps = {
	panel: WorkspacePanelStore;
	history: PanelHistoryStore;
	focus: ShellFocusStore;
	fileTree: FileTreeStore;
	navigation: PanelNavigation;
};

/** Wiki/Workspace file tabs and the dirty-editor leave guard that can veto replacing them. */
export function createFilePreview({
	panel,
	history,
	focus,
	fileTree,
	navigation
}: FilePreviewDeps) {
	let requestWorkspacePanelLeave: (() => boolean | Promise<boolean>) | null = null;

	async function openFilePreview(
		filePath: string,
		sessionId: string,
		reveal?: { startLine: number; endLine: number } | null
	) {
		const owner = ownerSurfaceForFile(filePath);
		const current = panel.panelStateFor(sessionId);
		const nextContent: SurfaceContent = {
			mode: 'file',
			filePath,
			...(reveal ? { startLine: reveal.startLine, endLine: reveal.endLine } : {})
		};
		if (
			replacesActiveFile(current, nextContent) &&
			requestWorkspacePanelLeave &&
			!(await requestWorkspacePanelLeave())
		) {
			return false;
		}

		const relative = isWikiUiPath(filePath) ? toWikiRelative(filePath) : filePath;
		fileTree.expandToRelativePath(sessionId, owner, relative);
		panel.applyPanelState(sessionId, openWorkspacePanelFile(current, owner, filePath, reveal));
		history.record(sessionId, owner, { kind: 'file', path: filePath });
		focus.setFocusedPane('web');
		syncWorkspacePanelOpen(true);
		return true;
	}

	return {
		registerWorkspacePanelLeaveGuard(guard: () => boolean | Promise<boolean>) {
			requestWorkspacePanelLeave = guard;
			return () => {
				if (requestWorkspacePanelLeave === guard) requestWorkspacePanelLeave = null;
			};
		},
		openFilePreview,
		/** Opens a workspace file in the panel for the active session. */
		openFilePreviewForActive(filePath: string, reveal?: FileRevealRange | null) {
			const sessionId = getActiveSessionId();
			if (!sessionId) return Promise.resolve(false);
			return openFilePreview(filePath, sessionId, reveal);
		},
		clearFileRevealForActive() {
			const sessionId = getActiveSessionId();
			if (!sessionId) return;
			const owner = panel.contentSurfaceFor(sessionId);
			if (owner !== 'wiki' && owner !== 'workspace') return;
			const current = panel.panelStateFor(sessionId);
			panel.applyPanelState(sessionId, clearFileReveal(current, owner));
		},
		activateFileTabForActive(filePath: string) {
			const sessionId = getActiveSessionId();
			if (!sessionId) return;
			const current = panel.panelStateFor(sessionId);
			const surface = current.contentSurface;
			if (surface !== 'wiki' && surface !== 'workspace') return;
			panel.applyPanelState(
				sessionId,
				activateWorkspacePanelFileTab(current, surface, filePath)
			);
			focus.setFocusedPane('web');
			syncWorkspacePanelOpen(true);
		},
		closeFileTabForActive(filePath: string) {
			const sessionId = getActiveSessionId();
			if (!sessionId) return;
			const current = panel.panelStateFor(sessionId);
			const surface = current.contentSurface;
			if (surface !== 'wiki' && surface !== 'workspace') return;
			const next = closeWorkspacePanelFileTab(current, surface, filePath);
			const stillFile = next.content[surface]?.mode === 'file';
			panel.applyPanelState(sessionId, next);
			if (stillFile) {
				focus.setFocusedPane('web');
				syncWorkspacePanelOpen(true);
				return;
			}
			history.record(sessionId, surface, { kind: 'browse', source: surface });
			navigation.requestFileTreeFilterFocus();
		}
	};
}
