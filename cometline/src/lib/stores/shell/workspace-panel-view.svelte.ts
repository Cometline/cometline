import { getActiveSessionId } from '$lib/active-session';
import {
	readWorkspacePanelTreeSource,
	type WorkspacePanelTreeSource
} from '$lib/features/workspace/workspace-panel-prefs';
import {
	fileTabsFor,
	urlTabsFor,
	type ContentSurface,
	type SurfaceContent,
	type SurfaceContentKey,
	type WorkspacePanelState
} from '$lib/features/workspace/workspace-panel-state';
import type { PanelHistoryStore } from './panel-history.svelte';
import type { WorkspacePanelStore } from './workspace-panel.svelte';

export type WorkspacePanelMode = 'url' | 'file' | 'git-diff';

/** Read-only, active-session view of the workspace panel for components. */
export function createWorkspacePanelView(panel: WorkspacePanelStore, history: PanelHistoryStore) {
	return {
		get workspacePanelOpen() {
			return panel.isOpenForActive();
		},
		get workspacePanelSurface() {
			return panel.activeSurface();
		},
		get terminalPanelOpen() {
			const key = getActiveSessionId();
			if (!key) return false;
			return panel.activeSurface() === 'terminal' && panel.isTerminalVisible(key);
		},
		get workspacePanelMode(): WorkspacePanelMode | null {
			const key = getActiveSessionId();
			if (!key) return null;
			return panel.activeSurfaceContent(key)?.mode ?? null;
		},
		get workspacePanelUrl() {
			const key = getActiveSessionId();
			if (!key) return null;
			const content = panel.activeSurfaceContent(key);
			return content?.mode === 'url' ? content.url : null;
		},
		get workspacePanelUrlTabId() {
			const key = getActiveSessionId();
			if (!key) return null;
			const content = panel.activeSurfaceContent(key);
			return content?.mode === 'url' ? (content.tabId ?? content.url) : null;
		},
		get workspacePanelUrlTabMeta() {
			const key = getActiveSessionId();
			if (!key) return {} as WorkspacePanelState['urlTabMeta'];
			return panel.panelStateFor(key).urlTabMeta;
		},
		get workspacePanelFilePath() {
			const key = getActiveSessionId();
			if (!key) return null;
			const content = panel.activeSurfaceContent(key);
			return content?.mode === 'file' ? content.filePath : null;
		},
		get workspacePanelFileTabs() {
			const key = getActiveSessionId();
			if (!key) return [] as string[];
			const state = panel.panelStateFor(key);
			const surface = state.contentSurface;
			if (surface !== 'wiki' && surface !== 'workspace') return [] as string[];
			return fileTabsFor(state, surface);
		},
		get wikiPanelFileTabs() {
			const key = getActiveSessionId();
			if (!key) return [] as string[];
			return fileTabsFor(panel.panelStateFor(key), 'wiki');
		},
		get workspaceSurfaceFileTabs() {
			const key = getActiveSessionId();
			if (!key) return [] as string[];
			return fileTabsFor(panel.panelStateFor(key), 'workspace');
		},
		get workspacePanelUrlTabs() {
			const key = getActiveSessionId();
			if (!key) return [] as string[];
			return urlTabsFor(panel.panelStateFor(key));
		},
		get workspaceWebTabs() {
			return panel.webTabs();
		},
		get workspacePanelGitDiffPath() {
			const key = getActiveSessionId();
			if (!key) return null;
			const content = panel.activeSurfaceContent(key);
			return content?.mode === 'git-diff' ? content.filePath : null;
		},
		get workspacePanelBrowseSource(): WorkspacePanelTreeSource {
			const key = getActiveSessionId();
			return key ? panel.browseSourceFor(key) : readWorkspacePanelTreeSource();
		},
		/** Active inner surface for the web right-sidebar stack. */
		get contentSurface(): ContentSurface {
			const key = getActiveSessionId();
			return key ? panel.contentSurfaceFor(key) : 'wiki';
		},
		get hasWorkspacePanelForSession() {
			const key = getActiveSessionId();
			return Boolean(key && panel.hasSession(key));
		},
		/** Storage key for the active session's panel, or null when none is open. */
		get workspacePanelSessionKey() {
			return getActiveSessionId();
		},
		get canPanelHistoryBack() {
			const key = getActiveSessionId();
			if (!key) return false;
			return history.canGoBack(key, panel.contentSurfaceFor(key));
		},
		get canPanelHistoryForward() {
			const key = getActiveSessionId();
			if (!key) return false;
			return history.canGoForward(key, panel.contentSurfaceFor(key));
		},
		/** True when a browse layer is active and that surface has no open content. */
		get workspacePanelBrowseOpen() {
			const key = getActiveSessionId();
			if (!key || !panel.isVisible(key)) return false;
			const surface = panel.contentSurfaceFor(key);
			if (surface !== 'wiki' && surface !== 'workspace' && surface !== 'changes')
				return false;
			return panel.contentFor(key, surface) === null;
		},
		get workspacePanelGitDiffOpen() {
			const key = getActiveSessionId();
			if (!key || !panel.isVisible(key)) return false;
			const content = panel.contentFor(key, 'changes');
			return panel.contentSurfaceFor(key) === 'changes' && content?.mode === 'git-diff';
		},
		/** Read content owned by a specific surface (for stacked UI layers). */
		getSurfaceContent(surface: SurfaceContentKey): SurfaceContent | null {
			const key = getActiveSessionId();
			return key ? panel.contentFor(key, surface) : null;
		}
	};
}
