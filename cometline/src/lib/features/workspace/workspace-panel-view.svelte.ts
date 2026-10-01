import { webTabActivity } from '$lib/features/workspace/web-tab-activity.svelte';
import { sessionStore } from '$lib/stores/session.svelte';
import { settingsStore } from '$lib/stores/settings.svelte';
import { shellStore } from '$lib/stores/shell.svelte';
import { terminalStore } from '$lib/stores/terminal.svelte';
import { normalizeWorkspacePath } from '$lib/features/workspace/file-index';
import { isBlankTabUrl, urlTabChipLabel } from '$lib/features/workspace/workspace-panel-state';

function displayAddress(url: string | null | undefined): string {
	if (!url || isBlankTabUrl(url)) return '';
	return url;
}

export function createWorkspacePanelView() {
	let addressDraft = $state<string | null>(null);
	let addressDraftUrl = $state<string | null>(null);

	const panelOpen = $derived(shellStore.workspacePanelOpen);
	const onTerminalSurface = $derived(shellStore.workspacePanelSurface === 'terminal');
	const onWebSurface = $derived(shellStore.workspacePanelSurface === 'web');
	const panelMode = $derived(shellStore.workspacePanelMode);
	const panelUrl = $derived(shellStore.workspacePanelUrl);
	const panelUrlTabId = $derived(shellStore.workspacePanelUrlTabId);
	const panelUrlTabMeta = $derived(shellStore.workspacePanelUrlTabMeta);
	const panelFilePath = $derived(shellStore.workspacePanelFilePath);
	const panelFileTabs = $derived(shellStore.workspacePanelFileTabs);
	const panelUrlTabs = $derived(shellStore.workspacePanelUrlTabs);
	const webTabs = $derived(shellStore.workspaceWebTabs);
	const wikiFileTabs = $derived(shellStore.wikiPanelFileTabs);
	const workspaceSurfaceFileTabs = $derived(shellStore.workspaceSurfaceFileTabs);
	const panelGitDiffPath = $derived(shellStore.workspacePanelGitDiffPath);
	const panelSessionKey = $derived(shellStore.workspacePanelSessionKey);
	const webSurface = $derived(shellStore.contentSurface);

	const wikiContent = $derived(shellStore.getSurfaceContent('wiki'));
	const workspaceContent = $derived(shellStore.getSurfaceContent('workspace'));
	const changesContent = $derived(shellStore.getSurfaceContent('changes'));
	const webSearchContent = $derived(shellStore.getSurfaceContent('web-search'));
	const wikiFilePath = $derived(wikiContent?.mode === 'file' ? wikiContent.filePath : null);
	const workspaceFilePath = $derived(
		workspaceContent?.mode === 'file' ? workspaceContent.filePath : null
	);
	const wikiRevealRange = $derived(
		wikiContent?.mode === 'file' && wikiContent.startLine != null && wikiContent.endLine != null
			? { startLine: wikiContent.startLine, endLine: wikiContent.endLine }
			: null
	);
	const workspaceRevealRange = $derived(
		workspaceContent?.mode === 'file' &&
			workspaceContent.startLine != null &&
			workspaceContent.endLine != null
			? { startLine: workspaceContent.startLine, endLine: workspaceContent.endLine }
			: null
	);
	const changesDiffPath = $derived(
		changesContent?.mode === 'git-diff' ? changesContent.filePath : null
	);
	const webSearchUrl = $derived(webSearchContent?.mode === 'url' ? webSearchContent.url : null);
	const wikiHasContentDot = $derived(Boolean(wikiContent));
	const workspaceHasContentDot = $derived(Boolean(workspaceContent));
	const changesHasContentDot = $derived(Boolean(changesContent));
	const webSearchHasContentDot = $derived(Boolean(webSearchUrl));
	const activeWebTabKey = $derived(
		panelSessionKey && panelUrlTabId ? `${panelSessionKey}:${panelUrlTabId}` : null
	);
	const webSurfaceRef = $derived(
		activeWebTabKey ? webTabActivity.get(activeWebTabKey)?.surface : undefined
	);
	const canGoBack = $derived(webSurfaceRef?.pageState?.canGoBack ?? false);
	const canGoForward = $derived(webSurfaceRef?.pageState?.canGoForward ?? false);
	const pageTitle = $derived(panelUrlTabMeta[panelUrlTabId ?? '']?.title ?? '');
	const loading = $derived(webSurfaceRef?.pageState?.loading ?? false);
	const capturingContext = $derived(webSurfaceRef?.pageState?.capturing ?? false);

	const shownAddress = $derived.by(() => {
		if (!shellStore.hasWorkspacePanelForSession) return '';
		if (addressDraft !== null && addressDraftUrl === (panelUrl ?? '')) return addressDraft;
		return displayAddress(panelUrl);
	});
	const addressEditing = $derived(
		shellStore.hasWorkspacePanelForSession &&
			addressDraft !== null &&
			addressDraftUrl === (panelUrl ?? '')
	);

	function urlTabLabel(id: string, active: boolean): string {
		const meta = panelUrlTabMeta[id] ?? { url: id, title: '' };
		return urlTabChipLabel(meta, active ? pageTitle : '');
	}

	const showWebview = $derived(
		Boolean(
			onWebSurface &&
			webSurface === 'web-search' &&
			webSearchUrl &&
			!isBlankTabUrl(webSearchUrl)
		)
	);
	const showFilePreview = $derived(
		Boolean(
			onWebSurface &&
			(webSurface === 'wiki' || webSurface === 'workspace') &&
			panelMode === 'file' &&
			panelFilePath
		)
	);
	const showGitDiff = $derived(
		Boolean(
			onWebSurface && webSurface === 'changes' && panelMode === 'git-diff' && panelGitDiffPath
		)
	);
	const wikiLayerActive = $derived(onWebSurface && webSurface === 'wiki' && !wikiContent);
	const workspaceLayerActive = $derived(
		onWebSurface && webSurface === 'workspace' && !workspaceContent
	);
	const changesLayerActive = $derived(
		onWebSurface && webSurface === 'changes' && !changesContent
	);
	const changesDiffActive = $derived(
		onWebSurface && webSurface === 'changes' && Boolean(changesDiffPath)
	);
	// Must track terminalPanelOpen (not just surface): soft-hide/exit leave
	// surface as `terminal` while the slot is closed — otherwise the embedded
	// TerminalPanel stays "active" and auto-restarts the PTY.
	const terminalLayerActive = $derived(shellStore.terminalPanelOpen);
	const terminalAvailable = $derived(Boolean(sessionStore.current));
	const activeTerminal = $derived(
		sessionStore.current ? terminalStore.getSnapshot(sessionStore.current.id) : null
	);
	const normalizedWorkspacePath = $derived(normalizeWorkspacePath(shellStore.workspacePath));
	const workspaceAvailable = $derived(
		Boolean(normalizedWorkspacePath && normalizedWorkspacePath !== '/')
	);
	const wikiActive = $derived(onWebSurface && webSurface === 'wiki');
	const workspaceActive = $derived(onWebSurface && webSurface === 'workspace');
	const changesActive = $derived(onWebSurface && webSurface === 'changes');
	const webSearchActive = $derived(onWebSurface && webSurface === 'web-search');
	const showBrowseFilter = $derived(
		onWebSurface &&
			shellStore.workspacePanelBrowseOpen &&
			(webSurface === 'wiki' || webSurface === 'workspace')
	);
	const showChangesTitle = $derived(onWebSurface && webSurface === 'changes' && !changesContent);
	const showTerminalTitle = $derived(onTerminalSurface);
	const showWebSearchField = $derived(
		onWebSurface && webSurface === 'web-search' && !showFilePreview && !showGitDiff
	);
	const showCenteredWebSearch = $derived(showWebSearchField && !showWebview);
	const showAddressOverlay = $derived(showWebSearchField && showWebview && addressEditing);
	const showContentSearch = $derived(showCenteredWebSearch || showAddressOverlay);
	const searchCaretTrail = $derived(settingsStore.settings.appearance.caretTrail);
	const searchCaretColor = $derived(settingsStore.settings.appearance.heroComposer.glowColor);
	const searchCaretTrailEnabled = $derived(searchCaretTrail.enabled);
	const surfaceTitle = $derived.by(() => {
		if (onTerminalSurface) {
			return activeTerminal?.status === 'exited' ? 'Terminal exited' : 'Terminal';
		}
		if (webSurface === 'wiki') return 'Wiki';
		if (webSurface === 'workspace') return 'Workspace';
		if (webSurface === 'changes') return 'Changes';
		if (webSurface === 'web-search') return 'Web';
		return '';
	});
	const toolbarCanGoBack = $derived(
		onWebSurface && ((showWebview && canGoBack) || shellStore.canPanelHistoryBack)
	);
	const toolbarCanGoForward = $derived(
		onWebSurface && ((showWebview && canGoForward) || shellStore.canPanelHistoryForward)
	);

	function beginAddressEdit() {
		addressDraftUrl = panelUrl ?? '';
		addressDraft = displayAddress(panelUrl);
	}

	function endAddressEdit() {
		addressDraft = null;
		addressDraftUrl = null;
	}

	function setAddressDraft(next: string) {
		addressDraftUrl = panelUrl ?? '';
		addressDraft = next;
	}

	return {
		get panelOpen() {
			return panelOpen;
		},
		get onTerminalSurface() {
			return onTerminalSurface;
		},
		get onWebSurface() {
			return onWebSurface;
		},
		get panelMode() {
			return panelMode;
		},
		get panelUrl() {
			return panelUrl;
		},
		get panelUrlTabId() {
			return panelUrlTabId;
		},
		get panelUrlTabMeta() {
			return panelUrlTabMeta;
		},
		get panelFilePath() {
			return panelFilePath;
		},
		get panelFileTabs() {
			return panelFileTabs;
		},
		get panelUrlTabs() {
			return panelUrlTabs;
		},
		get webTabs() {
			return webTabs;
		},
		get wikiFileTabs() {
			return wikiFileTabs;
		},
		get workspaceSurfaceFileTabs() {
			return workspaceSurfaceFileTabs;
		},
		get panelGitDiffPath() {
			return panelGitDiffPath;
		},
		get panelSessionKey() {
			return panelSessionKey;
		},
		get webSurface() {
			return webSurface;
		},
		get wikiFilePath() {
			return wikiFilePath;
		},
		get workspaceFilePath() {
			return workspaceFilePath;
		},
		get wikiRevealRange() {
			return wikiRevealRange;
		},
		get workspaceRevealRange() {
			return workspaceRevealRange;
		},
		get changesDiffPath() {
			return changesDiffPath;
		},
		get webSearchUrl() {
			return webSearchUrl;
		},
		get wikiHasContentDot() {
			return wikiHasContentDot;
		},
		get workspaceHasContentDot() {
			return workspaceHasContentDot;
		},
		get changesHasContentDot() {
			return changesHasContentDot;
		},
		get webSearchHasContentDot() {
			return webSearchHasContentDot;
		},
		get activeWebTabKey() {
			return activeWebTabKey;
		},
		get webSurfaceRef() {
			return webSurfaceRef;
		},
		get pageTitle() {
			return pageTitle;
		},
		get loading() {
			return loading;
		},
		get capturingContext() {
			return capturingContext;
		},
		get shownAddress() {
			return shownAddress;
		},
		get addressEditing() {
			return addressEditing;
		},
		get showWebview() {
			return showWebview;
		},
		get showFilePreview() {
			return showFilePreview;
		},
		get showGitDiff() {
			return showGitDiff;
		},
		get wikiLayerActive() {
			return wikiLayerActive;
		},
		get workspaceLayerActive() {
			return workspaceLayerActive;
		},
		get changesLayerActive() {
			return changesLayerActive;
		},
		get changesDiffActive() {
			return changesDiffActive;
		},
		get terminalLayerActive() {
			return terminalLayerActive;
		},
		get terminalAvailable() {
			return terminalAvailable;
		},
		get activeTerminal() {
			return activeTerminal;
		},
		get normalizedWorkspacePath() {
			return normalizedWorkspacePath;
		},
		get workspaceAvailable() {
			return workspaceAvailable;
		},
		get wikiActive() {
			return wikiActive;
		},
		get workspaceActive() {
			return workspaceActive;
		},
		get changesActive() {
			return changesActive;
		},
		get webSearchActive() {
			return webSearchActive;
		},
		get showBrowseFilter() {
			return showBrowseFilter;
		},
		get showChangesTitle() {
			return showChangesTitle;
		},
		get showTerminalTitle() {
			return showTerminalTitle;
		},
		get showWebSearchField() {
			return showWebSearchField;
		},
		get showCenteredWebSearch() {
			return showCenteredWebSearch;
		},
		get showAddressOverlay() {
			return showAddressOverlay;
		},
		get showContentSearch() {
			return showContentSearch;
		},
		get searchCaretTrail() {
			return searchCaretTrail;
		},
		get searchCaretColor() {
			return searchCaretColor;
		},
		get searchCaretTrailEnabled() {
			return searchCaretTrailEnabled;
		},
		get surfaceTitle() {
			return surfaceTitle;
		},
		get toolbarCanGoBack() {
			return toolbarCanGoBack;
		},
		get toolbarCanGoForward() {
			return toolbarCanGoForward;
		},
		urlTabLabel,
		beginAddressEdit,
		endAddressEdit,
		setAddressDraft
	};
}

export type WorkspacePanelView = ReturnType<typeof createWorkspacePanelView>;
