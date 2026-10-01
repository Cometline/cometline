import { getActiveSessionId } from '$lib/active-session';
import {
	activateWorkspacePanelUrlTab,
	closeWorkspacePanelUrlTab,
	navigateWorkspacePanelUrl,
	openWorkspacePanelUrl as openWorkspacePanelUrlState,
	syncUrlTab
} from '$lib/features/workspace/workspace-panel-state';
import type { ShellFocusStore } from './focus.svelte';
import type { PanelHistoryStore } from './panel-history.svelte';
import type { PanelNavigation } from './panel-navigation.svelte';
import { syncWorkspacePanelOpen, type WorkspacePanelStore } from './workspace-panel.svelte';

type WebTabsDeps = {
	panel: WorkspacePanelStore;
	history: PanelHistoryStore;
	focus: ShellFocusStore;
	navigation: PanelNavigation;
};

/** Web-search surface: URL tabs, address-bar navigation, and guest URL sync. */
export function createWebTabs({ panel, history, focus, navigation }: WebTabsDeps) {
	function openWorkspacePanelUrl(url: string, sessionId: string) {
		panel.showWebSurface(sessionId);
		if (url) {
			const current = panel.panelStateFor(sessionId);
			panel.applyPanelState(sessionId, openWorkspacePanelUrlState(current, url));
			history.record(sessionId, 'web-search', { kind: 'url', url });
		} else {
			const surface = panel.defaultContentSurfaceFor(sessionId);
			panel.setContentSurface(sessionId, surface);
			history.record(sessionId, surface, {
				kind: 'browse',
				source: surface === 'web-search' ? 'wiki' : surface
			});
		}
		focus.setFocusedPane('web');
		syncWorkspacePanelOpen(true);
	}

	return {
		openWorkspacePanelUrl,
		/** Opens a URL in the panel for the active session. */
		openWorkspacePanelUrlForActive(url: string) {
			const sessionId = getActiveSessionId();
			if (!sessionId) return;
			openWorkspacePanelUrl(url, sessionId);
		},
		syncWorkspacePanelUrlFromGuest(sessionId: string, tabId: string, url: string, title = '') {
			const next = url.trim();
			if (!next.startsWith('http://') && !next.startsWith('https://')) return;
			const current = panel.panelStateFor(sessionId);
			const nextState = syncUrlTab(current, tabId, next, title);
			if (nextState === current) return;
			panel.applyPanelState(sessionId, nextState);
		},
		navigateWorkspacePanel(url: string) {
			const sessionId = getActiveSessionId();
			if (!sessionId) return;
			panel.showWebSurface(sessionId);
			if (url) {
				const current = panel.panelStateFor(sessionId);
				panel.applyPanelState(sessionId, navigateWorkspacePanelUrl(current, url));
				history.record(sessionId, 'web-search', { kind: 'url', url });
				focus.setFocusedPane('web');
				syncWorkspacePanelOpen(true);
				navigation.requestAddressBarFocus();
				return;
			}
			const surface = panel.defaultContentSurfaceFor(sessionId);
			panel.setContentSurface(sessionId, surface);
			panel.setContentFor(sessionId, 'web-search', null);
			history.record(sessionId, surface, {
				kind: 'browse',
				source: surface === 'web-search' ? 'wiki' : surface
			});
			focus.setFocusedPane('web');
			syncWorkspacePanelOpen(true);
			navigation.requestFileTreeFilterFocus();
		},
		/** ⌘O: new web tab (Chrome-like) and focus the address bar. */
		openWebSearchPanel() {
			const sessionId = getActiveSessionId();
			if (!sessionId) return;
			panel.showWebSurface(sessionId);
			const current = panel.panelStateFor(sessionId);
			// Unique blank so repeated ⌘O stacks new tabs (about:blank alone would activate).
			const blankUrl = `about:blank#${Date.now().toString(36)}`;
			panel.applyPanelState(sessionId, openWorkspacePanelUrlState(current, blankUrl));
			history.record(sessionId, 'web-search', { kind: 'url', url: blankUrl });
			focus.setFocusedPane('web');
			syncWorkspacePanelOpen(true);
			navigation.requestAddressBarFocus();
		},
		activateUrlTabForActive(url: string) {
			const sessionId = getActiveSessionId();
			if (!sessionId) return;
			const current = panel.panelStateFor(sessionId);
			panel.applyPanelState(sessionId, activateWorkspacePanelUrlTab(current, url));
			focus.setFocusedPane('web');
			syncWorkspacePanelOpen(true);
		},
		closeUrlTabForActive(url: string) {
			const sessionId = getActiveSessionId();
			if (!sessionId) return;
			const current = panel.panelStateFor(sessionId);
			const next = closeWorkspacePanelUrlTab(current, url);
			const stillUrl = next.content['web-search']?.mode === 'url';
			panel.applyPanelState(sessionId, next);
			if (stillUrl) {
				focus.setFocusedPane('web');
				syncWorkspacePanelOpen(true);
				return;
			}
			history.record(sessionId, 'web-search', { kind: 'url', url: '' });
			navigation.requestAddressBarFocus();
		}
	};
}
