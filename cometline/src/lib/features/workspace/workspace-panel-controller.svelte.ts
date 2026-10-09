import { tick } from 'svelte';
import { webTabActivity } from '#lib/features/workspace/web-tab-activity.svelte.js';
import { sessionStore } from '#lib/stores/session.svelte.js';
import { shellStore } from '#lib/stores/shell.svelte.js';
import { terminalStore } from '#lib/stores/terminal.svelte.js';
import { isHttpUrl, normalizeUserUrl } from '#lib/open-link.js';
import { openExternalLink } from '#lib/external-link.js';
import { isWikiUiPath } from '#lib/wiki/paths.js';
import { createWorkspacePanelFocus } from '#lib/features/workspace/workspace-panel-controller-focus.svelte.js';
import type { WorkspacePanelView } from '#lib/features/workspace/workspace-panel-view.svelte.js';

export type WorkspaceEditorState = {
	dirty: boolean;
	saving: boolean;
	saveError: string | null;
	save: () => Promise<void>;
	revert: () => void;
};

type TreeBrowserHandle = {
	moveSelection: (delta: number) => boolean;
	activateSelection: () => boolean;
	handleTreeKey: (event: KeyboardEvent) => boolean;
};

type TerminalPanelHandle = { startTerminal: () => Promise<void> };

function fileContext(filePath: string) {
	const title = filePath.split(/[/\\]/).pop() || filePath;
	const source = isWikiUiPath(filePath) ? filePath : `workspace-file:${filePath}`;
	return { source, title };
}

export function createWorkspacePanelController(view: WorkspacePanelView) {
	const focus = createWorkspacePanelFocus(view);
	const { applyOwnedFocus } = focus;
	let editorState = $state<WorkspaceEditorState | null>(null);
	let dirtyByPath = $state<Record<string, boolean>>({});
	let wikiFilter = $state('');
	let workspaceFilter = $state('');
	let wikiTreeBrowser = $state<TreeBrowserHandle | null>(null);
	let workspaceTreeBrowser = $state<TreeBrowserHandle | null>(null);
	let terminalPanelRef = $state<TerminalPanelHandle | null>(null);
	let terminateConfirmOpen = $state(false);
	let discardChangesConfirmOpen = $state(false);
	let resolveDiscardChanges: ((discard: boolean) => void) | null = null;

	const activeTreeBrowser = $derived(
		view.webSurface === 'workspace' ? workspaceTreeBrowser : wikiTreeBrowser
	);
	const dirty = $derived(Boolean(editorState?.dirty));
	const saving = $derived(Boolean(editorState?.saving));
	const activeBrowseFilter = $derived(
		view.webSurface === 'workspace' ? workspaceFilter : wikiFilter
	);

	function syncFilterFromStore() {
		wikiFilter = shellStore.getFileTreeFilter('wiki');
		workspaceFilter = shellStore.getFileTreeFilter('workspace');
	}

	function setActiveBrowseFilter(value: string) {
		if (view.webSurface === 'workspace') {
			workspaceFilter = value;
			shellStore.setFileTreeFilter('workspace', value);
			return;
		}
		wikiFilter = value;
		shellStore.setFileTreeFilter('wiki', value);
	}

	async function confirmTerminateTerminal() {
		const session = sessionStore.current;
		if (!session) return;
		terminateConfirmOpen = false;
		await terminalStore.terminate(session.id);
	}

	function onBack() {
		if (view.showWebview && view.webSurfaceRef?.navigateBack()) return;
		shellStore.panelHistoryBack();
	}

	function onForward() {
		if (view.showWebview && view.webSurfaceRef?.navigateForward()) return;
		shellStore.panelHistoryForward();
	}

	function onReload() {
		view.webSurfaceRef?.reload();
	}

	async function resolvePageContext(source: string) {
		const matches = view.webTabs.filter(
			(tab) => tab.sessionId === view.panelSessionKey && tab.url === source
		);
		const tab = matches.find((tab) => tab.key === view.activeWebTabKey) ?? matches[0];
		return tab
			? ((await webTabActivity.get(tab.key)?.surface.captureContext(source)) ?? null)
			: null;
	}

	function noteVisibleContext() {
		const page =
			view.panelOpen && view.showWebview && view.webSearchUrl && isHttpUrl(view.webSearchUrl)
				? { source: view.webSearchUrl, title: view.pageTitle }
				: undefined;
		const file =
			view.showFilePreview && view.panelFilePath
				? fileContext(view.panelFilePath)
				: undefined;
		shellStore.noteVisibleContext({ page, file });
	}

	function requestLeaveEditor(): boolean | Promise<boolean> {
		if (!dirty) return true;
		discardChangesConfirmOpen = true;
		return new Promise((resolve) => {
			resolveDiscardChanges = resolve;
		});
	}

	function requestLeaveTab(filePath: string): boolean | Promise<boolean> {
		if (!dirtyByPath[filePath]) return true;
		discardChangesConfirmOpen = true;
		return new Promise((resolve) => {
			resolveDiscardChanges = resolve;
		});
	}

	async function closeFileTab(filePath: string): Promise<void> {
		if (!(await requestLeaveTab(filePath))) return;
		if (filePath === view.panelFilePath) {
			shellStore.closeWorkspacePanel();
		} else {
			shellStore.closeFileTabForActive(filePath);
		}
		if (shellStore.focusedPane === 'web' && view.panelOpen) {
			await tick();
			applyOwnedFocus();
		}
	}

	function closeUrlTab(id: string) {
		if (id === view.panelUrlTabId) shellStore.closeWorkspacePanel();
		else shellStore.closeUrlTabForActive(id);
		if (shellStore.focusedPane === 'web' && view.panelOpen) {
			void tick().then(() => applyOwnedFocus());
		}
	}

	function resolveLeaveEditor(discard: boolean) {
		discardChangesConfirmOpen = false;
		resolveDiscardChanges?.(discard);
		resolveDiscardChanges = null;
	}

	function onSaveClick() {
		void editorState?.save();
	}

	function onRevertClick() {
		editorState?.revert();
	}

	function setDirtyByPath(next: Record<string, boolean>) {
		const prev = dirtyByPath;
		const keys = new Set([...Object.keys(prev), ...Object.keys(next)]);
		for (const key of keys) {
			if (Boolean(prev[key]) !== Boolean(next[key])) {
				dirtyByPath = next;
				return;
			}
		}
	}

	function handlePanelKeydown(event: KeyboardEvent) {
		if (!view.panelOpen || shellStore.focusedPane !== 'web') return;
		if ((event.metaKey || event.ctrlKey) && (event.key === 's' || event.key === 'S')) {
			if (view.panelMode !== 'file' || !editorState) return;
			event.preventDefault();
			void editorState.save();
			return;
		}
		if ((event.metaKey || event.ctrlKey) && (event.key === 'r' || event.key === 'R')) {
			if (!view.showWebview) return;
			event.preventDefault();
			event.stopPropagation();
			onReload();
		}
	}

	function handlePanelMouseDown(event: MouseEvent) {
		if (view.onTerminalSurface) {
			shellStore.setFocusedPane('terminal');
			return;
		}
		shellStore.setFocusedPane('web');
		const webSurface = view.webSurface;
		if (webSurface === 'wiki' || webSurface === 'workspace' || webSurface === 'changes') return;
		if (view.panelMode !== 'url' || event.button !== 0) return;
		const target = event.target;
		if (!(target instanceof HTMLElement)) {
			shellStore.requestAddressBarFocus();
			return;
		}
		if (target.closest('button, input, textarea, select, a, [role="button"]')) return;
		shellStore.requestAddressBarFocus();
	}

	function submitAddress() {
		const normalized = normalizeUserUrl(view.shownAddress);
		if (!normalized) return;
		view.endAddressEdit();
		shellStore.navigateWorkspacePanel(normalized);
	}

	function onFilterKeydown(event: KeyboardEvent) {
		if (
			event.key === 'ArrowUp' ||
			event.key === 'ArrowDown' ||
			event.key === 'Enter' ||
			event.key === 'ArrowLeft' ||
			event.key === 'ArrowRight'
		) {
			if (activeTreeBrowser?.handleTreeKey(event)) return;
		}
		if (event.key === 'Escape') {
			event.preventDefault();
			focus.fileTreeFilterInputEl?.blur();
		}
	}

	function onAddressKeydown(event: KeyboardEvent) {
		if (event.key === 'Enter') {
			event.preventDefault();
			submitAddress();
			return;
		}
		if (event.key === 'Escape') {
			event.preventDefault();
			view.endAddressEdit();
			focus.addressInputEl?.blur();
		}
	}

	function onFilterFocus() {
		shellStore.setFocusedPane('web');
	}

	function onAddressFocus() {
		view.beginAddressEdit();
		shellStore.setFocusedPane('web');
	}

	function onAddressBlur() {
		view.endAddressEdit();
	}

	function onNewWindow(url: string) {
		if (isHttpUrl(url)) {
			void shellStore.openWorkspacePanelUrlForActive(url);
			return;
		}
		openExternalLink(url);
	}

	function keepPaneFocus(event: MouseEvent) {
		event.preventDefault();
	}

	function openNewWebTab() {
		shellStore.openWebSearchPanel();
	}

	async function openTreeFile(path: string) {
		shellStore.setFocusedPane('web');
		const opened = await shellStore.openFilePreviewForActive(path);
		if (opened === false) return;
		await tick();
		applyOwnedFocus();
	}

	function switchBrowseSource(source: 'wiki' | 'workspace') {
		shellStore.setWorkspacePanelBrowseSource(source);
		void tick().then(() => applyOwnedFocus());
	}

	return {
		get editorState() {
			return editorState;
		},
		set editorState(next: WorkspaceEditorState | null) {
			editorState = next;
		},
		get dirtyByPath() {
			return dirtyByPath;
		},
		get panelFocusEl() {
			return focus.panelFocusEl;
		},
		set panelFocusEl(next: HTMLDivElement | null) {
			focus.panelFocusEl = next;
		},
		get wikiTreeBrowser() {
			return wikiTreeBrowser;
		},
		set wikiTreeBrowser(next: TreeBrowserHandle | null) {
			wikiTreeBrowser = next;
		},
		get workspaceTreeBrowser() {
			return workspaceTreeBrowser;
		},
		set workspaceTreeBrowser(next: TreeBrowserHandle | null) {
			workspaceTreeBrowser = next;
		},
		get terminalPanelRef() {
			return terminalPanelRef;
		},
		set terminalPanelRef(next: TerminalPanelHandle | null) {
			terminalPanelRef = next;
		},
		get terminateConfirmOpen() {
			return terminateConfirmOpen;
		},
		set terminateConfirmOpen(next: boolean) {
			terminateConfirmOpen = next;
		},
		get discardChangesConfirmOpen() {
			return discardChangesConfirmOpen;
		},
		get wikiFilter() {
			return wikiFilter;
		},
		get workspaceFilter() {
			return workspaceFilter;
		},
		get activeBrowseFilter() {
			return activeBrowseFilter;
		},
		get dirty() {
			return dirty;
		},
		get saving() {
			return saving;
		},
		syncFilterFromStore,
		setActiveBrowseFilter,
		confirmTerminateTerminal,
		onBack,
		onForward,
		onReload,
		resolvePageContext,
		noteVisibleContext,
		requestLeaveEditor,
		closeFileTab,
		closeUrlTab,
		resolveLeaveEditor,
		onSaveClick,
		onRevertClick,
		setDirtyByPath,
		handlePanelKeydown,
		handlePanelMouseDown,
		onFilterKeydown,
		onAddressKeydown,
		onFilterFocus,
		onAddressFocus,
		onAddressBlur,
		onNewWindow,
		keepPaneFocus,
		openNewWebTab,
		openTreeFile,
		switchBrowseSource,
		applyOwnedFocus,
		applyAddressFocus: focus.applyAddressFocus,
		applyFileTreeFilterFocus: focus.applyFileTreeFilterFocus,
		trackAddressInput: focus.trackAddressInput,
		trackFileTreeFilterInput: focus.trackFileTreeFilterInput
	};
}

export type WorkspacePanelController = ReturnType<typeof createWorkspacePanelController>;
