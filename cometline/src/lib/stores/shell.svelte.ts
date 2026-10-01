import { sessionStore } from '$lib/stores/session.svelte';
import { createShellChromeStore } from '$lib/stores/shell/chrome.svelte';
import { createFilePreview } from '$lib/stores/shell/file-preview.svelte';
import { createFileTreeStore } from '$lib/stores/shell/file-tree.svelte';
import { createShellFocusStore } from '$lib/stores/shell/focus.svelte';
import { createPanelHistoryStore } from '$lib/stores/shell/panel-history.svelte';
import { createPanelNavigation } from '$lib/stores/shell/panel-navigation.svelte';
import { createTerminalPanel } from '$lib/stores/shell/terminal-panel.svelte';
import { createWebContextStore } from '$lib/stores/shell/web-context.svelte';
import { createWebTabs } from '$lib/stores/shell/web-tabs.svelte';
import { createWorkspacePanelView } from '$lib/stores/shell/workspace-panel-view.svelte';
import { createWorkspacePanelStore } from '$lib/stores/shell/workspace-panel.svelte';
import { createWorkspacePathsStore } from '$lib/stores/shell/workspace-paths.svelte';

export type { WorkspacePanelMode } from '$lib/stores/shell/workspace-panel-view.svelte';
export type { FileTreeExpandSource } from '$lib/stores/shell/file-tree.svelte';
/** Surfaces that can own independent open content. */
export type {
	ContentSurface,
	SurfaceContent,
	SurfaceContentKey,
	WorkspacePanelSurface
} from '$lib/features/workspace/workspace-panel-state';
export type { ComposerFocusRequest, FocusedPane } from '$lib/stores/shell/focus.svelte';
export type {
	PendingPageContext,
	PendingViewingFileContext,
	PendingWebContext
} from '$lib/stores/shell/web-context.svelte';

function createShellStore() {
	const chrome = createShellChromeStore();
	const paths = createWorkspacePathsStore();
	const panel = createWorkspacePanelStore();
	const history = createPanelHistoryStore();
	const fileTree = createFileTreeStore();
	const webContext = createWebContextStore();
	const focus = createShellFocusStore();
	const view = createWorkspacePanelView(panel, history);
	const navigation = createPanelNavigation({ panel, history, focus, fileTree, webContext });
	const filePreview = createFilePreview({ panel, history, focus, fileTree, navigation });
	const webTabs = createWebTabs({ panel, history, focus, navigation });
	const terminal = createTerminalPanel({ panel, focus });

	return {
		get sidebarOpen() {
			return chrome.sidebarOpen;
		},
		get fullscreen() {
			return chrome.fullscreen;
		},
		get settingsOpen() {
			return chrome.settingsOpen;
		},
		get introOpen() {
			return chrome.introOpen;
		},
		get setupOpen() {
			return chrome.setupOpen;
		},
		get composerPhase() {
			return chrome.composerPhase;
		},
		get defaultWorkspacePath() {
			return paths.defaultWorkspacePath;
		},
		get workspacePath() {
			return paths.workspacePath;
		},
		get sidebarOrderWorkspacePath() {
			return paths.sidebarOrderWorkspacePath;
		},
		get sidebarOrderDiscordActive() {
			return paths.sidebarOrderDiscordActive;
		},
		get bootMessage() {
			return chrome.bootMessage;
		},
		get focusedPane() {
			return focus.focusedPane;
		},
		get workspacePanelOpen() {
			return view.workspacePanelOpen;
		},
		get workspacePanelSurface() {
			return view.workspacePanelSurface;
		},
		get terminalPanelOpen() {
			return view.terminalPanelOpen;
		},
		get workspacePanelMode() {
			return view.workspacePanelMode;
		},
		get workspacePanelUrl() {
			return view.workspacePanelUrl;
		},
		get workspacePanelUrlTabId() {
			return view.workspacePanelUrlTabId;
		},
		get workspacePanelUrlTabMeta() {
			return view.workspacePanelUrlTabMeta;
		},
		get workspacePanelFilePath() {
			return view.workspacePanelFilePath;
		},
		get workspacePanelFileTabs() {
			return view.workspacePanelFileTabs;
		},
		get wikiPanelFileTabs() {
			return view.wikiPanelFileTabs;
		},
		get workspaceSurfaceFileTabs() {
			return view.workspaceSurfaceFileTabs;
		},
		get workspacePanelUrlTabs() {
			return view.workspacePanelUrlTabs;
		},
		/** Live web guests follow tab ownership, not the currently selected session/surface. */
		get workspaceWebTabs() {
			return view.workspaceWebTabs;
		},
		get workspacePanelGitDiffPath() {
			return view.workspacePanelGitDiffPath;
		},
		get workspacePanelBrowseSource() {
			return view.workspacePanelBrowseSource;
		},
		/** Active inner surface for the web right-sidebar stack. */
		get contentSurface() {
			return view.contentSurface;
		},
		get pendingWebContexts() {
			return webContext.pendingWebContexts;
		},
		get hasWorkspacePanelForSession() {
			return view.hasWorkspacePanelForSession;
		},
		/** Storage key for the active session's panel, or null when none is open. */
		get workspacePanelSessionKey() {
			return view.workspacePanelSessionKey;
		},
		get addressBarFocusRequestId() {
			return focus.addressBarFocusRequestId;
		},
		get fileTreeFilterFocusRequestId() {
			return focus.fileTreeFilterFocusRequestId;
		},
		get gitChangesOpenRequestId() {
			return focus.gitChangesOpenRequestId;
		},
		/** Last ⌘O target while browsing: filter vs web address. */
		get lastWorkspacePanelFocusTarget() {
			return focus.lastWorkspacePanelFocusTarget;
		},
		get terminalFocusRequestId() {
			return focus.terminalFocusRequestId;
		},
		get composerFocusRequest() {
			return focus.composerFocusRequest;
		},
		get sessionFindRequestId() {
			return focus.sessionFindRequestId;
		},
		get canPanelHistoryBack() {
			return view.canPanelHistoryBack;
		},
		get canPanelHistoryForward() {
			return view.canPanelHistoryForward;
		},
		/** True when a browse layer is active and that surface has no open content. */
		get workspacePanelBrowseOpen() {
			return view.workspacePanelBrowseOpen;
		},
		get workspacePanelGitDiffOpen() {
			return view.workspacePanelGitDiffOpen;
		},
		getSurfaceContent: view.getSurfaceContent,
		setDefaultWorkspacePath: paths.setDefaultWorkspacePath,
		initializeDefaultWorkspace: paths.initializeDefaultWorkspace,
		setActiveWorkspacePath: paths.setActiveWorkspacePath,
		setSidebarOrderWorkspacePath: paths.setSidebarOrderWorkspacePath,
		setSidebarOrderDiscordActive: paths.setSidebarOrderDiscordActive,
		commitActiveWorkspace: paths.commitActiveWorkspace,
		resetActiveToDefault: paths.resetActiveToDefault,
		setBootMessage: chrome.setBootMessage,
		setFullscreen: chrome.setFullscreen,
		toggleSidebar: chrome.toggleSidebar,
		openSidebar: chrome.openSidebar,
		closeSidebar: chrome.closeSidebar,
		openSettings: chrome.openSettings,
		closeSettings: chrome.closeSettings,
		openIntro: chrome.openIntro,
		closeIntro: chrome.closeIntro,
		openSetup: chrome.openSetup,
		closeSetup: chrome.closeSetup,
		dockComposer: chrome.dockComposer,
		centerComposer: chrome.centerComposer,
		setFocusedPane: focus.setFocusedPane,
		addWebContextForActive: webContext.addWebContextForActive,
		setViewingFileContextForActive: webContext.setViewingFileContextForActive,
		noteVisibleContext: webContext.noteVisibleContext,
		setPendingPageContextForActive: webContext.setPendingPageContextForActive,
		registerPageContextResolver: webContext.registerPageContextResolver,
		registerWorkspacePanelLeaveGuard: filePreview.registerWorkspacePanelLeaveGuard,
		resolvePendingWebContextsForActive: webContext.resolvePendingWebContextsForActive,
		removeWebContextAt: webContext.removeWebContextAt,
		clearWebContextForActive: webContext.clearWebContextForActive,
		requestComposerFocus: focus.requestComposerFocus,
		requestSessionFind: focus.requestSessionFind,
		onActiveSessionChange: terminal.onActiveSessionChange,
		openWorkspacePanelUrl: webTabs.openWorkspacePanelUrl,
		openFilePreview: filePreview.openFilePreview,
		clearFileRevealForActive: filePreview.clearFileRevealForActive,
		openGitDiff: navigation.openGitDiff,
		openWorkspacePanelBrowse: navigation.openWorkspacePanelBrowse,
		setWorkspacePanelBrowseSource: navigation.setWorkspacePanelBrowseSource,
		syncWorkspacePanelUrlFromGuest: webTabs.syncWorkspacePanelUrlFromGuest,
		navigateWorkspacePanel: webTabs.navigateWorkspacePanel,
		panelHistoryBack: navigation.panelHistoryBack,
		panelHistoryForward: navigation.panelHistoryForward,
		ensureWorkspacePanelVisible: navigation.ensureWorkspacePanelVisible,
		requestFileTreeFilterFocus: navigation.requestFileTreeFilterFocus,
		requestAddressBarFocus: navigation.requestAddressBarFocus,
		openWebSearchPanel: webTabs.openWebSearchPanel,
		openGitChangesPanel: navigation.openGitChangesPanel,
		toggleWorkspacePanel: navigation.toggleWorkspacePanel,
		openTerminalPanel: terminal.openTerminalPanel,
		requestTerminalFocus: terminal.requestTerminalFocus,
		closeWorkspacePanel: navigation.closeWorkspacePanel,
		closeTerminalPanelForSession: terminal.closeTerminalPanelForSession,
		clearWorkspacePanelForSession: navigation.clearWorkspacePanelForSession,
		getFileTreeExpanded: fileTree.getFileTreeExpanded,
		setFileTreeExpanded: fileTree.setFileTreeExpanded,
		getFileTreeFilter: fileTree.getFileTreeFilter,
		setFileTreeFilter: fileTree.setFileTreeFilter,
		openFilePreviewForActive: filePreview.openFilePreviewForActive,
		activateFileTabForActive: filePreview.activateFileTabForActive,
		closeFileTabForActive: filePreview.closeFileTabForActive,
		activateUrlTabForActive: webTabs.activateUrlTabForActive,
		closeUrlTabForActive: webTabs.closeUrlTabForActive,
		openGitDiffForActive: navigation.openGitDiffForActive,
		openWorkspacePanelUrlForActive: webTabs.openWorkspacePanelUrlForActive
	};
}

export const shellStore = createShellStore();

sessionStore.onSessionRemoved((sessionId) => {
	if (shellStore.workspaceWebTabs.some((tab) => tab.sessionId === sessionId)) {
		shellStore.clearWorkspacePanelForSession(sessionId);
	}
});
