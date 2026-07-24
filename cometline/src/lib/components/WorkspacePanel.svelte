<script lang="ts">
	import {
		ArrowLeft,
		ArrowRight,
		BookOpen,
		FileText,
		FolderTree,
		GitBranch,
		Play,
		Power,
		RotateCcw,
		RotateCw,
		Save,
		Search,
		SquareTerminal,
		X
	} from '@lucide/svelte';
	import { tick, untrack } from 'svelte';
	import ConfirmActionModal from '$lib/components/ConfirmActionModal.svelte';
	import FileTreeBrowser from '$lib/components/FileTreeBrowser.svelte';
	import WorkspaceFileSurface from '$lib/components/WorkspaceFileSurface.svelte';
	import GitChangesBrowser from '$lib/components/GitChangesBrowser.svelte';
	import GitDiffView from '$lib/components/GitDiffView.svelte';
	import TerminalPanel from '$lib/components/TerminalPanel.svelte';
	import Tooltip from '$lib/components/Tooltip.svelte';
	import WorkspaceWebSurface from '$lib/components/WorkspaceWebSurface.svelte';
	import { sessionStore } from '$lib/stores/session.svelte';
	import { shellStore } from '$lib/stores/shell.svelte';
	import { terminalStore } from '$lib/stores/terminal.svelte';
	import { isHttpUrl, normalizeUserUrl, openLink } from '$lib/open-link';
	import { openExternalLink } from '$lib/external-link';
	import { isWikiUiPath } from '$lib/wiki/paths';
	import { normalizeWorkspacePath } from '$lib/workspace/file-index';

	let addressInputEl = $state<HTMLInputElement | null>(null);
	let webCanGoBack = $state(false);
	let webCanGoForward = $state(false);
	let loading = $state(false);
	let addressInput = $state('');
	let webPageTitle = $state('');
	let addressEditing = $state(false);
	let lastObservedPanelUrl = $state<string | null>(null);
	let editorState = $state<{
		dirty: boolean;
		saving: boolean;
		saveError: string | null;
		save: () => Promise<void>;
		revert: () => void;
	} | null>(null);
	let capturingContext = $state(false);
	let webSurfaceRef = $state<{
		navigateBack: () => boolean;
		navigateForward: () => boolean;
		reload: () => void;
		captureContext: (source?: string) => Promise<import('$lib/actions/start-chat').WebContext | null>;
	} | null>(null);
	let wikiFilter = $state('');
	let workspaceFilter = $state('');
	let fileTreeFilterInputEl = $state<HTMLInputElement | null>(null);
	let satisfiedFilterFocusRequestId = 0;
	type TreeBrowserHandle = {
		moveSelection: (delta: number) => boolean;
		activateSelection: () => boolean;
		handleTreeKey: (event: KeyboardEvent) => boolean;
	};
	let wikiTreeBrowser = $state<TreeBrowserHandle | null>(null);
	let workspaceTreeBrowser = $state<TreeBrowserHandle | null>(null);
	let terminalPanelRef = $state<{ startTerminal: () => Promise<void> } | null>(null);
	let terminateConfirmOpen = $state(false);
	let discardChangesConfirmOpen = $state(false);
	let resolveDiscardChanges: ((discard: boolean) => void) | null = null;

	const panelOpen = $derived(shellStore.workspacePanelOpen);
	const onTerminalSurface = $derived(shellStore.workspacePanelSurface === 'terminal');
	const onWebSurface = $derived(shellStore.workspacePanelSurface === 'web');
	const panelMode = $derived(shellStore.workspacePanelMode);
	const panelUrl = $derived(shellStore.workspacePanelUrl);
	const panelFilePath = $derived(shellStore.workspacePanelFilePath);
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
	const changesDiffPath = $derived(
		changesContent?.mode === 'git-diff' ? changesContent.filePath : null
	);
	const webSearchUrl = $derived(webSearchContent?.mode === 'url' ? webSearchContent.url : null);
	const wikiHasContentDot = $derived(Boolean(wikiContent));
	const workspaceHasContentDot = $derived(Boolean(workspaceContent));
	const changesHasContentDot = $derived(Boolean(changesContent));
	const webSearchHasContentDot = $derived(Boolean(webSearchUrl));
	const canGoBack = $derived(webSearchUrl ? webCanGoBack : false);
	const canGoForward = $derived(webSearchUrl ? webCanGoForward : false);
	const pageTitle = $derived(webSearchUrl ? webPageTitle : '');

	const showWebview = $derived(Boolean(onWebSurface && webSurface === 'web-search' && webSearchUrl));
	const showFilePreview = $derived(
		Boolean(
			onWebSurface &&
				(webSurface === 'wiki' || webSurface === 'workspace') &&
				panelMode === 'file' &&
				panelFilePath
		)
	);
	const showGitDiff = $derived(
		Boolean(onWebSurface && webSurface === 'changes' && panelMode === 'git-diff' && panelGitDiffPath)
	);
	const wikiLayerActive = $derived(onWebSurface && webSurface === 'wiki' && !wikiContent);
	const workspaceLayerActive = $derived(
		onWebSurface && webSurface === 'workspace' && !workspaceContent
	);
	const changesLayerActive = $derived(onWebSurface && webSurface === 'changes' && !changesContent);
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
	const showChangesTitle = $derived(
		onWebSurface && webSurface === 'changes' && !changesContent
	);
	const showTerminalTitle = $derived(onTerminalSurface);
	const showWebSearchField = $derived(
		onWebSurface && webSurface === 'web-search' && !showFilePreview && !showGitDiff
	);
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
	const activeTreeBrowser = $derived(
		webSurface === 'workspace' ? workspaceTreeBrowser : wikiTreeBrowser
	);
	const dirty = $derived(Boolean(editorState?.dirty));
	const saving = $derived(Boolean(editorState?.saving));
	const toolbarCanGoBack = $derived(
		onWebSurface && ((showWebview && canGoBack) || shellStore.canPanelHistoryBack)
	);
	const toolbarCanGoForward = $derived(
		onWebSurface && ((showWebview && canGoForward) || shellStore.canPanelHistoryForward)
	);

	function syncFilterFromStore() {
		wikiFilter = shellStore.getFileTreeFilter('wiki');
		workspaceFilter = shellStore.getFileTreeFilter('workspace');
	}

	function setActiveBrowseFilter(value: string) {
		if (webSurface === 'workspace') {
			workspaceFilter = value;
			shellStore.setFileTreeFilter('workspace', value);
			return;
		}
		wikiFilter = value;
		shellStore.setFileTreeFilter('wiki', value);
	}

	const activeBrowseFilter = $derived(
		webSurface === 'workspace' ? workspaceFilter : wikiFilter
	);

	async function confirmTerminateTerminal() {
		const session = sessionStore.current;
		if (!session) return;
		terminateConfirmOpen = false;
		await terminalStore.terminate(session.id);
	}



	function syncAddressFromNavigation() {
		if (addressEditing) return;
		addressInput = panelUrl || '';
	}

	function updateWebNavigation(state: {
		url: string;
		title: string;
		canGoBack: boolean;
		canGoForward: boolean;
		loading: boolean;
	}) {
		webCanGoBack = state.canGoBack;
		webCanGoForward = state.canGoForward;
		loading = state.loading;
		webPageTitle = state.title;
		if (!addressEditing) addressInput = state.url || panelUrl || '';
		if (state.url.startsWith('http://') || state.url.startsWith('https://')) {
			shellStore.setPendingPageContextForActive({ title: state.title, source: state.url });
		}
	}

	function onBack() {
		if (showWebview && webSurfaceRef?.navigateBack()) return;
		shellStore.panelHistoryBack();
	}

	function onForward() {
		if (showWebview && webSurfaceRef?.navigateForward()) return;
		shellStore.panelHistoryForward();
	}

	/** Used by AppShell shared ⌘[ / ⌘] routing when the workspace panel is focused. */
	export function navigateBack() {
		onBack();
	}

	export function navigateForward() {
		onForward();
	}

	function onReload() {
		webSurfaceRef?.reload();
	}

	async function capturePageContext() {
		const context = await webSurfaceRef?.captureContext();
		if (context) shellStore.addWebContextForActive(context);
	}

	async function resolvePageContext(source: string) {
		return (await webSurfaceRef?.captureContext(source)) ?? null;
	}

	function captureFileContext(filePath: string) {
		const title = filePath.split(/[/\\]/).pop() || filePath;
		const source = isWikiUiPath(filePath) ? filePath : `workspace-file:${filePath}`;
		shellStore.setViewingFileContextForActive(source, title);
	}

	function onClose() {
		shellStore.closeWorkspacePanel();
	}

	function requestLeaveEditor(): boolean | Promise<boolean> {
		if (!dirty) return true;
		discardChangesConfirmOpen = true;
		return new Promise((resolve) => {
			resolveDiscardChanges = resolve;
		});
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

	function handlePanelKeydown(event: KeyboardEvent) {
		if (!panelOpen || shellStore.focusedPane !== 'web') return;
		if ((event.metaKey || event.ctrlKey) && (event.key === 's' || event.key === 'S')) {
			if (panelMode !== 'file' || !editorState) return;
			event.preventDefault();
			void editorState.save();
			return;
		}
		if ((event.metaKey || event.ctrlKey) && (event.key === 'r' || event.key === 'R')) {
			if (!showWebview) return;
			event.preventDefault();
			event.stopPropagation();
			onReload();
		}
	}

	function handlePanelMouseDown(event: MouseEvent) {
		if (onTerminalSurface) {
			shellStore.setFocusedPane('terminal');
			return;
		}
		shellStore.setFocusedPane('web');
		if (webSurface === 'wiki' || webSurface === 'workspace' || webSurface === 'changes') return;
		if (panelMode !== 'url' || event.button !== 0) return;
		const target = event.target;
		if (!(target instanceof HTMLElement)) {
			shellStore.requestAddressBarFocus();
			return;
		}
		if (target.closest('button, input, textarea, select, a, [role="button"]')) return;
		shellStore.requestAddressBarFocus();
	}

	function submitAddress() {
		const normalized = normalizeUserUrl(addressInput);
		if (!normalized) return;
		addressEditing = false;
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
			fileTreeFilterInputEl?.blur();
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
			addressEditing = false;
			syncAddressFromNavigation();
			addressInputEl?.blur();
		}
	}

	function onFilterFocus() {
		shellStore.setFocusedPane('web');
	}

	function onAddressFocus() {
		addressEditing = true;
		shellStore.setFocusedPane('web');
	}

	function onAddressBlur() {
		addressEditing = false;
		syncAddressFromNavigation();
	}

	function onNewWindow(url: string) {
		if (isHttpUrl(url)) {
			openLink(url);
			return;
		}
		openExternalLink(url);
	}

	// Tracks the focus request id we have already satisfied, so a remounting
	// input (or a late effect run) doesn't refocus twice and a brand-new request
	// always wins regardless of which path observes it first.
	let satisfiedFocusRequestId = 0;

	function applyAddressFocus(force = false) {
		const requestId = shellStore.addressBarFocusRequestId;
		if (!requestId || (!force && requestId === satisfiedFocusRequestId)) return;
		if (!shellStore.workspacePanelOpen) return;
		const el = addressInputEl;
		if (!el) return;
		satisfiedFocusRequestId = requestId;
		shellStore.setFocusedPane('web');
		el.focus({ preventScroll: true });
		el.select();
	}

	function trackAddressInput(node: HTMLInputElement) {
		addressInputEl = node;
		// The input may mount *after* a focus request was issued (panel reopen
		// rebuilds the URL field). Focus straight from mount so no request is lost.
		applyAddressFocus();
		return {
			destroy() {
				if (addressInputEl === node) addressInputEl = null;
			}
		};
	}

	function applyFileTreeFilterFocus(force = false) {
		const requestId = shellStore.fileTreeFilterFocusRequestId;
		if (!requestId || (!force && requestId === satisfiedFilterFocusRequestId)) return;
		if (!shellStore.workspacePanelOpen || !showBrowseFilter) return;
		const el = fileTreeFilterInputEl;
		if (!el) return;
		satisfiedFilterFocusRequestId = requestId;
		shellStore.setFocusedPane('web');
		el.focus({ preventScroll: true });
		el.select();
	}

	function trackFileTreeFilterInput(node: HTMLInputElement) {
		fileTreeFilterInputEl = node;
		applyFileTreeFilterFocus();
		return {
			destroy() {
				if (fileTreeFilterInputEl === node) fileTreeFilterInputEl = null;
			}
		};
	}

	$effect(() => shellStore.registerPageContextResolver(resolvePageContext));
	$effect(() => shellStore.registerWorkspacePanelLeaveGuard(requestLeaveEditor));

	$effect(() => {
		const url = panelUrl;
		if (url !== lastObservedPanelUrl) {
			lastObservedPanelUrl = url;
			// Programmatic navigation (e.g. clicking a link) should stop editing the address.
			addressEditing = false;
		}
		if (!addressEditing) {
			syncAddressFromNavigation();
		}
	});

	$effect(() => {
		if (!shellStore.hasWorkspacePanelForSession) {
			loading = false;
			editorState = null;
			if (!addressEditing) {
				addressInput = '';
			}
		}
	});

	$effect(() => {
		const filePath = showFilePreview ? panelFilePath : null;
		if (!filePath) return;
		// untrack: setViewingFileContextForActive reads+writes pending contexts; if
		// that read is tracked here, every write re-runs this effect forever.
		untrack(() => captureFileContext(filePath));
	});

	$effect(() => {
		void panelSessionKey;
		syncFilterFromStore();
	});

	$effect(() => {
		// Re-run whenever a focus is requested or the panel opens. The input may
		// already be mounted (panel was visible) so handle it here too; if it is
		// still mounting, trackAddressInput will pick up the same request id.
		const requestId = shellStore.addressBarFocusRequestId;
		const open = panelOpen;
		if (!requestId || !open) return;
		void tick().then(() =>
			requestAnimationFrame(() => requestAnimationFrame(() => applyAddressFocus(true)))
		);
	});

	$effect(() => {
		const requestId = shellStore.fileTreeFilterFocusRequestId;
		const open = panelOpen;
		const filterVisible = showBrowseFilter;
		if (!requestId || !open || !filterVisible) return;
		void tick().then(() =>
			requestAnimationFrame(() => requestAnimationFrame(() => applyFileTreeFilterFocus(true)))
		);
	});

</script>

<svelte:window onkeydown={handlePanelKeydown} />

<div class="workspace-panel" class:open={panelOpen} aria-hidden={!panelOpen}>
	<div
		class="workspace-panel-inner content-panel-surface"
		class:pane-focus-active={(shellStore.focusedPane === 'web' ||
			shellStore.focusedPane === 'terminal') &&
			panelOpen}
	>
		<!-- svelte-ignore a11y_no_static_element_interactions -->
		<header class="workspace-panel-toolbar" onmousedown={handlePanelMouseDown}>
			<div class="nav-actions">
				<Tooltip
					label={terminalAvailable ? 'Terminal' : 'Start a chat to use Terminal'}
					action={terminalAvailable ? 'openTerminal' : undefined}
				>
					<button
						type="button"
						class="icon-button"
						class:active={onTerminalSurface}
						disabled={!terminalAvailable}
						onclick={() => shellStore.requestTerminalFocus()}
						aria-label={terminalAvailable ? 'Terminal' : 'Start a chat to use Terminal'}
					>
						<SquareTerminal size={16} />
					</button>
				</Tooltip>
				<Tooltip label="Wiki files" action="openWikiPanel">
					<button
						type="button"
						class="icon-button"
						class:active={wikiActive}
						onclick={() => shellStore.setWorkspacePanelBrowseSource('wiki')}
						aria-label="Wiki files"
					>
						<BookOpen size={16} />
						{#if wikiHasContentDot}
							<span class="surface-content-dot" aria-hidden="true"></span>
						{/if}
					</button>
				</Tooltip>
				<Tooltip
					label={workspaceAvailable
						? 'Workspace files'
						: 'Select a workspace to browse files'}
					action={workspaceAvailable ? 'openWorkspacePanel' : undefined}
				>
					<button
						type="button"
						class="icon-button"
						class:active={workspaceActive}
						disabled={!workspaceAvailable}
						onclick={() => shellStore.setWorkspacePanelBrowseSource('workspace')}
						aria-label={workspaceAvailable
							? 'Workspace files'
							: 'Select a workspace to browse files'}
					>
						<FolderTree size={16} />
						{#if workspaceHasContentDot}
							<span class="surface-content-dot" aria-hidden="true"></span>
						{/if}
					</button>
				</Tooltip>
				<Tooltip label="Web search" action="openWebSearch">
					<button
						type="button"
						class="icon-button"
						class:active={webSearchActive}
						onclick={() => shellStore.openWebSearchPanel()}
						aria-label="Web search"
					>
						<Search size={16} />
						{#if webSearchHasContentDot}
							<span class="surface-content-dot" aria-hidden="true"></span>
						{/if}
					</button>
				</Tooltip>
				<Tooltip
					label={workspaceAvailable
						? 'Git changes'
						: 'Select a workspace to see git changes'}
					action={workspaceAvailable ? 'openGitPanel' : undefined}
				>
					<button
						type="button"
						class="icon-button"
						class:active={changesActive}
						disabled={!workspaceAvailable}
						onclick={() => shellStore.openGitChangesPanel()}
						aria-label={workspaceAvailable
							? 'Git changes'
							: 'Select a workspace to see git changes'}
					>
						<GitBranch size={16} />
						{#if changesHasContentDot}
							<span class="surface-content-dot" aria-hidden="true"></span>
						{/if}
					</button>
				</Tooltip>
				<Tooltip label="Back" action="navigateBack">
					<button
						type="button"
						class="icon-button"
						disabled={!toolbarCanGoBack}
						onclick={onBack}
						aria-label="Back"
					>
						<ArrowLeft size={16} />
					</button>
				</Tooltip>
				<Tooltip label="Forward" action="navigateForward">
					<button
						type="button"
						class="icon-button"
						disabled={!toolbarCanGoForward}
						onclick={onForward}
						aria-label="Forward"
					>
						<ArrowRight size={16} />
					</button>
				</Tooltip>
				{#if showWebview}
					<button
						type="button"
						class="icon-button"
						onclick={onReload}
						aria-label="Reload page"
						title="Reload page"
					>
						<RotateCw size={16} class={loading ? 'spin' : ''} />
					</button>
					<button
						type="button"
						class="icon-button"
						disabled={capturingContext}
						onclick={() => void capturePageContext()}
						aria-label="Add page to chat context"
						title="Add page to next message"
					>
						<FileText size={16} />
					</button>
				{/if}
			</div>
			<div class="url-field">
				{#if showTerminalTitle}
					<span class="page-title">{surfaceTitle}</span>
				{:else if showFilePreview && panelFilePath}
					<span class="page-title">
						{panelFilePath.split(/[/\\]/).pop()}{#if dirty}<span
								class="dirty-dot"
								aria-label="Unsaved changes"
							>
								•</span
							>{/if}
					</span>
					<span class="file-path-display" title={panelFilePath}>{panelFilePath}</span>
				{:else if showGitDiff && panelGitDiffPath}
					<span class="page-title">Diff</span>
					<span class="file-path-display" title={panelGitDiffPath}>{panelGitDiffPath}</span>
				{:else if showChangesTitle}
					<span class="page-title">{surfaceTitle}</span>
				{:else if showBrowseFilter}
					<div class="url-field-row">
						<span class="page-title surface-title">{surfaceTitle}</span>
						<input
							use:trackFileTreeFilterInput
							class="address-input browse-filter-input"
							type="text"
							spellcheck="false"
							autocomplete="off"
							placeholder={webSurface === 'wiki'
								? 'Filter wiki files…'
								: 'Filter workspace files…'}
							value={activeBrowseFilter}
							oninput={(event) =>
								setActiveBrowseFilter((event.currentTarget as HTMLInputElement).value)}
							onfocus={onFilterFocus}
							onkeydown={onFilterKeydown}
							aria-label="Filter files"
						/>
					</div>
				{:else if showWebSearchField}
					<div class="url-field-row">
						<span class="page-title surface-title">{surfaceTitle}</span>
						<div class="url-field-search">
							{#if pageTitle}
								<span class="page-title-sub">{pageTitle}</span>
							{/if}
							<input
								use:trackAddressInput
								class="address-input"
								type="text"
								inputmode="search"
								spellcheck="false"
								autocapitalize="off"
								autocomplete="off"
								placeholder="Search web or enter URL"
								bind:value={addressInput}
								onfocus={onAddressFocus}
								onblur={onAddressBlur}
								onkeydown={onAddressKeydown}
								aria-label="Workspace panel address"
							/>
						</div>
					</div>
				{:else}
					<span class="page-title">{surfaceTitle}</span>
				{/if}
			</div>
			{#if showTerminalTitle}
				<div class="file-actions">
					{#if activeTerminal?.status === 'running'}
						<button
							type="button"
							class="icon-button"
							onclick={() => (terminateConfirmOpen = true)}
							aria-label="Terminate terminal"
							title="Terminate terminal"
						>
							<Power size={16} />
						</button>
					{:else}
						<button
							type="button"
							class="icon-button"
							onclick={() => void terminalPanelRef?.startTerminal()}
							disabled={!terminalAvailable}
							aria-label="Start terminal"
							title="Start terminal"
						>
							<Play size={16} />
						</button>
					{/if}
				</div>
			{:else if showFilePreview && editorState}
				<div class="file-actions">
					<button
						type="button"
						class="icon-button"
						disabled={!dirty || saving}
						onclick={onRevertClick}
						aria-label="Revert changes"
						title="Revert changes"
					>
						<RotateCcw size={16} />
					</button>
					<button
						type="button"
						class="icon-button"
						disabled={!dirty || saving}
						onclick={onSaveClick}
						aria-label="Save file"
						title="Save (Cmd/Ctrl+S)"
					>
						<Save size={16} />
					</button>
				</div>
			{/if}
			<button
				type="button"
				class="icon-button close-button"
				onclick={onClose}
				aria-label="Close panel"
			>
				<X size={16} />
			</button>
		</header>
		<!-- svelte-ignore a11y_no_static_element_interactions -->
		<div class="workspace-panel-content" onmousedown={handlePanelMouseDown}>
			<div
				class="panel-layer panel-layer-terminal"
				class:active={terminalLayerActive}
				aria-hidden={!terminalLayerActive}
			>
				<TerminalPanel bind:this={terminalPanelRef} active={terminalLayerActive} />
			</div>
			{#if shellStore.hasWorkspacePanelForSession}
				<div
					class="panel-layer"
					class:active={wikiLayerActive}
					aria-hidden={!wikiLayerActive}
				>
					<FileTreeBrowser
						bind:this={wikiTreeBrowser}
						source="wiki"
						workspacePath={shellStore.workspacePath}
						filter={wikiFilter}
						onSelectFile={(path) => void shellStore.openFilePreviewForActive(path)}
					/>
				</div>
				<div
					class="panel-layer"
					class:active={workspaceLayerActive}
					aria-hidden={!workspaceLayerActive}
				>
					<FileTreeBrowser
						bind:this={workspaceTreeBrowser}
						source="workspace"
						workspacePath={shellStore.workspacePath}
						filter={workspaceFilter}
						onSelectFile={(path) => void shellStore.openFilePreviewForActive(path)}
					/>
				</div>
				<div
					class="panel-layer"
					class:active={changesLayerActive}
					aria-hidden={!changesLayerActive}
				>
					<GitChangesBrowser workspacePath={normalizedWorkspacePath} />
				</div>
				<WorkspaceFileSurface
					workspacePath={shellStore.workspacePath}
					{wikiFilePath}
					{workspaceFilePath}
					activeSurface={webSurface}
					active={onWebSurface}
					onEditorState={(state) => (editorState = state)}
				/>
				{#if changesDiffPath}
					<div
						class="panel-layer panel-layer-content"
						class:active={changesDiffActive}
						aria-hidden={!changesDiffActive}
					>
						<GitDiffView
							workspacePath={shellStore.workspacePath}
							filePath={changesDiffPath}
							scope="all"
							onBack={() => shellStore.panelHistoryBack()}
						/>
					</div>
				{/if}
				{#if webSearchUrl}
					<div
						class="panel-layer panel-layer-content"
						class:active={showWebview}
						aria-hidden={!showWebview}
					>
						<WorkspaceWebSurface
							bind:this={webSurfaceRef}
							url={webSearchUrl}
							sessionKey={panelSessionKey}
							onNavigationState={updateWebNavigation}
							onFocus={() => shellStore.setFocusedPane('web')}
							onNewWindow={onNewWindow}
							onCapturingChange={(value) => (capturingContext = value)}
						/>
					</div>
				{/if}
			{/if}
		</div>
	</div>
</div>

<ConfirmActionModal
	open={terminateConfirmOpen}
	title="Terminate terminal?"
	description="This will stop the shell and every program started from it, including tmux, development servers, and SSH connections. This cannot be undone."
	confirmLabel="Terminate terminal"
	onCancel={() => (terminateConfirmOpen = false)}
	onConfirm={() => void confirmTerminateTerminal()}
/>

<ConfirmActionModal
	open={discardChangesConfirmOpen}
	title="Discard unsaved changes?"
	description="Your edits to this file will be lost."
	confirmLabel="Discard changes"
	onCancel={() => resolveLeaveEditor(false)}
	onConfirm={() => resolveLeaveEditor(true)}
/>

<style>
	.workspace-panel {
		flex: 0 0 auto;
		width: 0;
		min-width: 0;
		height: 100%;
		overflow: hidden;
		pointer-events: none;
		box-sizing: border-box;
		transition: width var(--duration-fast) var(--ease-smooth);
	}

	.workspace-panel.open {
		width: var(--workspace-panel-slot-width);
		max-width: 100%;
		min-width: 0;
		flex-shrink: 1;
		pointer-events: auto;
	}

	.workspace-panel-inner {
		width: var(--workspace-panel-width);
		height: calc(100% - (2 * var(--content-panel-inset)));
		display: flex;
		flex-direction: column;
		margin: var(--content-panel-inset);
		margin-left: 0;
		box-sizing: border-box;
		overflow: hidden;
		transition:
			width var(--duration-fast) var(--ease-smooth),
			border-color var(--duration-fast) var(--ease-smooth),
			box-shadow var(--duration-fast) var(--ease-smooth);
	}

	.workspace-panel-toolbar {
		display: flex;
		align-items: center;
		gap: 8px;
		box-sizing: border-box;
		min-height: var(--panel-header-height);
		padding: 0 10px;
		border-bottom: 1px solid var(--border-soft);
		background: rgba(250, 250, 249, 0.95);
	}

	.nav-actions {
		display: flex;
		align-items: center;
		gap: 4px;
		flex-shrink: 0;
	}

	.file-actions {
		display: flex;
		align-items: center;
		gap: 4px;
		flex-shrink: 0;
	}

	.icon-button {
		position: relative;
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 28px;
		height: 28px;
		padding: 0;
		border: none;
		border-radius: 8px;
		background: transparent;
		color: var(--text-main);
		cursor: pointer;
	}

	.icon-button:hover:not(:disabled) {
		background: rgba(15, 23, 42, 0.06);
	}

	.icon-button.active {
		background: rgba(15, 23, 42, 0.08);
	}

	.icon-button:disabled {
		opacity: 0.35;
		cursor: default;
	}

	.surface-content-dot {
		position: absolute;
		top: 3px;
		right: 3px;
		width: 6px;
		height: 6px;
		border-radius: 999px;
		background: var(--hero-composer-glow-color, #72c0ff);
		box-shadow: 0 0 8px var(--hero-composer-glow-soft, rgba(114, 192, 255, 0.24));
		pointer-events: none;
	}

	.url-field {
		position: relative;
		flex: 1;
		min-width: 0;
		display: flex;
		flex-direction: column;
		justify-content: center;
		gap: 1px;
		padding: 0 4px;
	}

	.url-field-row {
		display: flex;
		align-items: center;
		gap: 10px;
		min-width: 0;
	}

	.url-field-search {
		flex: 1;
		min-width: 0;
		display: flex;
		flex-direction: column;
		gap: 1px;
	}

	.page-title {
		font-size: 12px;
		font-weight: 600;
		color: var(--text-main);
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.surface-title {
		flex-shrink: 0;
		max-width: 7.5rem;
	}

	.page-title-sub {
		font-size: 11px;
		font-weight: 500;
		color: var(--text-muted);
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.dirty-dot {
		color: var(--accent, #2563eb);
		font-weight: 700;
	}

	.address-input {
		flex: 1;
		width: 100%;
		min-width: 0;
		border: none;
		background: transparent;
		font-size: 11px;
		color: var(--text-muted);
		padding: 0;
		outline: none;
	}

	.browse-filter-input {
		font-size: 12px;
		color: var(--text-main);
	}

	.address-input:focus {
		color: var(--text-main);
	}

	.address-input::placeholder {
		color: var(--text-muted);
		opacity: 0.7;
	}

	.file-path-display {
		width: 100%;
		min-width: 0;
		font-size: 11px;
		color: var(--text-muted);
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.close-button {
		flex-shrink: 0;
	}

	.workspace-panel-content {
		flex: 1;
		min-height: 0;
		position: relative;
		background: #fff;
		overflow: hidden;
	}

	.panel-layer {
		position: absolute;
		inset: 0;
		display: flex;
		flex-direction: column;
		min-height: 0;
		background: #fff;
		transform: translateX(100%);
		opacity: 0;
		pointer-events: none;
		visibility: hidden;
		transition:
			transform 180ms var(--ease-smooth, ease),
			opacity 180ms var(--ease-smooth, ease),
			visibility 180ms;
		z-index: 1;
	}

	.panel-layer.active {
		transform: translateX(0);
		opacity: 1;
		pointer-events: auto;
		visibility: visible;
		z-index: 2;
	}

	.panel-layer-content.active,
	.panel-layer-terminal.active {
		z-index: 3;
	}

	.panel-layer :global(.file-tree-browser),
	.panel-layer :global(.git-changes),
	.panel-layer :global(.git-diff-view) {
		flex: 1;
		min-height: 0;
		height: 100%;
	}

	:global(.spin) {
		animation: workspace-panel-spin 0.8s linear infinite;
	}

	@keyframes workspace-panel-spin {
		to {
			transform: rotate(360deg);
		}
	}

	@media (prefers-reduced-motion: reduce) {
		.workspace-panel {
			transition: none;
		}

		.workspace-panel-inner {
			transition: none;
		}

		.panel-layer {
			transition: none;
		}
	}

	@media (max-width: 900px) {
		.workspace-panel {
			position: fixed;
			inset: 0;
			z-index: 40;
			width: 100% !important;
			transition: none;
			pointer-events: none;
		}

		.workspace-panel.open {
			pointer-events: auto;
		}

		.workspace-panel-inner {
			width: 100%;
			height: 100%;
			margin: 0;
			border: none;
			border-radius: 0;
			box-shadow: none;
			transform: translateX(100%);
			transition: transform var(--duration-fast) var(--ease-smooth);
		}

		.workspace-panel.open .workspace-panel-inner {
			transform: translateX(0);
		}
	}
</style>
