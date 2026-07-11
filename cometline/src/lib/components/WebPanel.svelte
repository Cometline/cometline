<script lang="ts">
	import { ArrowLeft, ArrowRight, FileText, RotateCcw, RotateCw, Save, X } from '@lucide/svelte';
	import { tick } from 'svelte';
	import FilePreview from '$lib/components/FilePreview.svelte';
	import { shellStore } from '$lib/stores/shell.svelte';
	import { isWebPanelUrl, normalizeUserUrl, openLink } from '$lib/open-link';
	import { openExternalLink } from '$lib/external-link';
	import { readWorkspaceFileContent } from '$lib/client/cometmind';
	import { loadWebPanelFileOptions } from '$lib/workspace/web-panel-input-options';

	type WebviewElement = HTMLElement & {
		src: string;
		goBack(): void;
		goForward(): void;
		reload(): void;
		stop(): void;
		canGoBack(): boolean;
		canGoForward(): boolean;
		getURL(): string;
		getTitle(): string;
		executeJavaScript<T = unknown>(code: string, userGesture?: boolean): Promise<T>;
	};

	type FileEditorState = {
		dirty: boolean;
		saving: boolean;
		saveError: string | null;
		save: () => Promise<void>;
		revert: () => void;
	};

	type CachedPageContext = {
		sessionKey: string;
		url: string;
		title: string;
		content: string;
		capturedAt: number;
	};

	const PAGE_CONTEXT_CAPTURE_DELAY_MS = 250;
	const PAGE_CONTEXT_CACHE_TTL_MS = 10_000;

	let webviewEl = $state<WebviewElement | null>(null);
	let addressInputEl = $state<HTMLInputElement | null>(null);
	let canGoBack = $state(false);
	let canGoForward = $state(false);
	let loading = $state(false);
	let addressInput = $state('');
	let addressOptionHighlight = $state(0);
	let fileOptions = $state<string[]>([]);
	let fileOptionsLoading = $state(false);
	let pageTitle = $state('');
	let webviewSessionId = $state<string | null>(null);
	let webviewLoadedUrl = $state<string | null>(null);
	let addressEditing = $state(false);
	let lastObservedPanelUrl = $state<string | null>(null);
	let editorState = $state<FileEditorState | null>(null);
	let displayedFilePath = $state<string | null>(null);
	let capturingContext = $state(false);
	let announcingContextCapture = $state(false);
	let contextMessage = $state('');
	let pageCaptureRun = 0;
	let fileCaptureRun = 0;
	let pageCaptureTimer: ReturnType<typeof setTimeout> | null = null;
	let cachedPageContext = $state<CachedPageContext | null>(null);

	const panelOpen = $derived(shellStore.webPanelOpen);
	const panelMode = $derived(shellStore.webPanelMode);
	const panelUrl = $derived(shellStore.webPanelUrl);
	const panelFilePath = $derived(shellStore.webPanelFilePath);
	const panelSessionKey = $derived(shellStore.webPanelSessionKey);
	const showWebview = $derived(
		panelMode === 'url' && Boolean(shellStore.hasWebPanelForSession && panelUrl)
	);
	const showFilePreview = $derived(
		panelMode === 'file' && Boolean(shellStore.hasWebPanelForSession && displayedFilePath)
	);
	const dirty = $derived(Boolean(editorState?.dirty));
	const saving = $derived(Boolean(editorState?.saving));
	const addressQuery = $derived(addressInput.trim());
	const showAddressOptions = $derived(
		panelMode === 'url' && addressEditing && Boolean(addressQuery)
	);
	const addressOptionCount = $derived(showAddressOptions ? 1 + fileOptions.length : 0);
	const webOptionVerb = $derived.by(() => {
		const normalized = normalizeUserUrl(addressInput);
		if (!normalized) return 'Search web';
		try {
			const parsed = new URL(normalized);
			return parsed.hostname === 'www.google.com' && parsed.pathname === '/search'
				? 'Search web'
				: 'Open URL';
		} catch {
			return 'Search web';
		}
	});

	function syncAddressFromNavigation() {
		if (addressEditing) return;
		const el = webviewEl;
		if (el) {
			try {
				addressInput = el.getURL() || panelUrl || '';
			} catch {
				addressInput = panelUrl || '';
			}
			return;
		}
		addressInput = panelUrl || '';
	}

	function updateNavigationState() {
		const el = webviewEl;
		if (!el) return;
		canGoBack = el.canGoBack();
		canGoForward = el.canGoForward();
		syncAddressFromNavigation();
		try {
			pageTitle = el.getTitle() || '';
		} catch {
			pageTitle = '';
		}
	}

	function onBack() {
		if (!webviewEl?.canGoBack()) return;
		webviewEl.goBack();
	}

	function onForward() {
		if (!webviewEl?.canGoForward()) return;
		webviewEl.goForward();
	}

	function onReload() {
		webviewEl?.reload();
	}

	function cancelScheduledPageCapture() {
		if (!pageCaptureTimer) return;
		clearTimeout(pageCaptureTimer);
		pageCaptureTimer = null;
	}

	function queuePageContextCapture() {
		cancelScheduledPageCapture();
		pageCaptureTimer = setTimeout(() => {
			pageCaptureTimer = null;
			void capturePageContext();
		}, PAGE_CONTEXT_CAPTURE_DELAY_MS);
	}

	function addCachedPageContext(context: CachedPageContext) {
		shellStore.addWebContextForActive({
			kind: 'page',
			title: context.title,
			source: context.url,
			content: context.content
		});
	}

	async function capturePageContext({ announce = false } = {}) {
		const el = webviewEl;
		if (!el || panelMode !== 'url' || !panelUrl || capturingContext) return;
		const captureRun = ++pageCaptureRun;
		const capturedPanelUrl = panelUrl;
		const capturedSessionKey = panelSessionKey;
		let currentUrl = panelUrl;
		try {
			currentUrl = String(el.getURL() || panelUrl).trim();
		} catch {
			// A newly-mounted webview may not expose its URL yet; use panel state.
		}
		const cached = cachedPageContext;
		if (
			cached &&
			cached.sessionKey === capturedSessionKey &&
			cached.url === currentUrl &&
			Date.now() - cached.capturedAt < PAGE_CONTEXT_CACHE_TTL_MS
		) {
			addCachedPageContext(cached);
			if (announce) contextMessage = 'Page context added to the next message.';
			return;
		}

		capturingContext = true;
		announcingContextCapture = announce;
		if (announce) contextMessage = '';
		try {
			const page = await el.executeJavaScript<{
				title?: string;
				url?: string;
				content?: string;
			}>(
				`(() => ({
					title: document.title || '',
					url: location.href || '',
					content: (document.body?.innerText || '').replace(/\\n{3,}/g, '\\n\\n').trim().slice(0, 50000)
				}))()`,
				true
			);
			const url = String(page?.url || el.getURL() || panelUrl).trim();
			const content = String(page?.content || '').trim();
			if (
				captureRun !== pageCaptureRun ||
				shellStore.webPanelUrl !== capturedPanelUrl ||
				shellStore.webPanelSessionKey !== capturedSessionKey
			)
				return;
			if (!url.startsWith('http://') && !url.startsWith('https://')) {
				throw new Error('Only http(s) pages can be added to chat context.');
			}
			if (!content) {
				throw new Error('This page has no readable text content.');
			}
			const context = {
				sessionKey: capturedSessionKey,
				url,
				title: String(page?.title || pageTitle || '').trim(),
				content,
				capturedAt: Date.now()
			};
			cachedPageContext = context;
			addCachedPageContext(context);
			if (announce) contextMessage = 'Page context added to the next message.';
		} catch (error) {
			if (announce) {
				contextMessage =
					error instanceof Error ? error.message : 'Could not read this page.';
			}
		} finally {
			capturingContext = false;
			announcingContextCapture = false;
		}
	}

	async function captureFileContext(workspacePath: string, filePath: string) {
		const captureRun = ++fileCaptureRun;
		const capturedSessionKey = panelSessionKey;
		try {
			const result = await readWorkspaceFileContent(workspacePath, filePath);
			if (
				captureRun !== fileCaptureRun ||
				shellStore.webPanelFilePath !== filePath ||
				shellStore.webPanelSessionKey !== capturedSessionKey
			)
				return;
			if (result.kind !== 'text' || !result.content.trim()) return;
			shellStore.addWebContextForActive({
				kind: 'file',
				title: filePath.split(/[/\\]/).pop() || filePath,
				source: `workspace-file:${filePath}`,
				content: result.content
			});
		} catch {
			// FilePreview owns the visible error state; context capture is best effort.
		}
	}

	function confirmDiscardIfDirty(): boolean {
		if (!dirty) return true;
		return window.confirm('Discard unsaved changes?');
	}

	function onClose() {
		if (!confirmDiscardIfDirty()) return;
		shellStore.closeWebPanel();
	}

	function onSaveClick() {
		void editorState?.save();
	}

	function onRevertClick() {
		editorState?.revert();
	}

	function handlePanelKeydown(event: KeyboardEvent) {
		if ((event.metaKey || event.ctrlKey) && (event.key === 's' || event.key === 'S')) {
			if (panelMode !== 'file' || !editorState) return;
			event.preventDefault();
			void editorState.save();
		}
	}

	function handlePanelMouseDown(event: MouseEvent) {
		shellStore.setFocusedPane('web');
		if (panelMode !== 'url' || event.button !== 0) return;
		const target = event.target;
		if (!(target instanceof HTMLElement)) {
			shellStore.requestAddressBarFocus();
			return;
		}
		if (target.closest('button, input, textarea, select, a, [role="button"]')) return;
		shellStore.requestAddressBarFocus();
	}

	function selectWebOption() {
		const normalized = normalizeUserUrl(addressInput);
		if (!normalized) return;
		addressEditing = false;
		fileOptions = [];
		shellStore.navigateWebPanel(normalized);
	}

	function selectFileOption(path: string) {
		addressEditing = false;
		fileOptions = [];
		addressInput = path;
		shellStore.openFilePreviewForActive(path);
	}

	function selectHighlightedAddressOption() {
		if (!showAddressOptions || addressOptionHighlight === 0) {
			selectWebOption();
			return;
		}
		const filePath = fileOptions[addressOptionHighlight - 1];
		if (filePath) {
			selectFileOption(filePath);
			return;
		}
		selectWebOption();
	}

	function onAddressInput() {
		addressOptionHighlight = 0;
	}

	function onAddressKeydown(event: KeyboardEvent) {
		if (showAddressOptions && event.key === 'ArrowDown') {
			event.preventDefault();
			addressOptionHighlight = (addressOptionHighlight + 1) % Math.max(addressOptionCount, 1);
			return;
		}
		if (showAddressOptions && event.key === 'ArrowUp') {
			event.preventDefault();
			addressOptionHighlight =
				(addressOptionHighlight - 1 + Math.max(addressOptionCount, 1)) %
				Math.max(addressOptionCount, 1);
			return;
		}
		if (event.key === 'Enter') {
			event.preventDefault();
			selectHighlightedAddressOption();
			return;
		}
		if (event.key === 'Escape') {
			event.preventDefault();
			addressEditing = false;
			fileOptions = [];
			syncAddressFromNavigation();
			addressInputEl?.blur();
		}
	}

	function onAddressFocus() {
		addressEditing = true;
		shellStore.setFocusedPane('web');
	}

	function onAddressBlur() {
		addressEditing = false;
		fileOptions = [];
		syncAddressFromNavigation();
	}

	function keepAddressFocus(event: MouseEvent) {
		event.preventDefault();
	}

	function onNewWindow(event: Event & { url?: string; preventDefault?: () => void }) {
		event.preventDefault?.();
		const url = event.url;
		if (!url) return;
		if (isWebPanelUrl(url)) {
			openLink(url);
			return;
		}
		openExternalLink(url);
	}

	function attachWebview(el: WebviewElement) {
		el.setAttribute('sandbox', 'allow-scripts allow-same-origin allow-popups allow-forms');
		const onNavigate = () => {
			updateNavigationState();
		};
		const onInPageNavigate = () => {
			updateNavigationState();
			queuePageContextCapture();
		};
		const onStartLoading = () => {
			loading = true;
		};
		const onStopLoading = () => {
			loading = false;
			updateNavigationState();
			queuePageContextCapture();
		};
		const onTitleUpdated = (event: Event & { title?: string }) => {
			pageTitle = event.title ?? '';
		};
		const onFocus = () => {
			shellStore.setFocusedPane('web');
		};

		el.addEventListener('did-navigate', onNavigate);
		el.addEventListener('did-navigate-in-page', onInPageNavigate);
		el.addEventListener('did-start-loading', onStartLoading);
		el.addEventListener('did-stop-loading', onStopLoading);
		el.addEventListener('page-title-updated', onTitleUpdated);
		el.addEventListener('new-window', onNewWindow);
		el.addEventListener('focus', onFocus);

		return () => {
			cancelScheduledPageCapture();
			el.removeEventListener('did-navigate', onNavigate);
			el.removeEventListener('did-navigate-in-page', onInPageNavigate);
			el.removeEventListener('did-start-loading', onStartLoading);
			el.removeEventListener('did-stop-loading', onStopLoading);
			el.removeEventListener('page-title-updated', onTitleUpdated);
			el.removeEventListener('new-window', onNewWindow);
			el.removeEventListener('focus', onFocus);
			try {
				el.stop();
			} catch {
				// ignore teardown errors
			}
		};
	}

	// Tracks the focus request id we have already satisfied, so a remounting
	// input (or a late effect run) doesn't refocus twice and a brand-new request
	// always wins regardless of which path observes it first.
	let satisfiedFocusRequestId = 0;
	let fileOptionSeq = 0;

	function applyAddressFocus() {
		const requestId = shellStore.addressBarFocusRequestId;
		if (!requestId || requestId === satisfiedFocusRequestId) return;
		if (!shellStore.webPanelOpen) return;
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

	$effect(() => {
		const el = webviewEl;
		if (!el) return;
		return attachWebview(el);
	});

	$effect(() => {
		const el = webviewEl;
		const sessionKey = panelSessionKey;
		const url = panelUrl;
		const open = panelOpen;
		if (!el || !open || !sessionKey || !url) return;
		if (webviewSessionId !== sessionKey || webviewLoadedUrl !== url) {
			el.src = url;
			webviewSessionId = sessionKey;
			webviewLoadedUrl = url;
			if (!addressEditing) {
				addressInput = url;
			}
		}
	});

	$effect(() => {
		const url = panelUrl;
		if (url !== lastObservedPanelUrl) {
			lastObservedPanelUrl = url;
			// Programmatic navigation, such as clicking a link in an assistant
			// response, must dismiss any stale address suggestions first.
			addressEditing = false;
			fileOptions = [];
			addressOptionHighlight = 0;
		}
		if (!addressEditing) {
			syncAddressFromNavigation();
		}
	});

	$effect(() => {
		if (!shellStore.hasWebPanelForSession) {
			cancelScheduledPageCapture();
			cachedPageContext = null;
			loading = false;
			canGoBack = false;
			canGoForward = false;
			pageTitle = '';
			webviewLoadedUrl = null;
			webviewSessionId = null;
			displayedFilePath = null;
			editorState = null;
			if (!addressEditing) {
				addressInput = '';
			}
		}
	});

	// Guard file switches behind an unsaved-change confirmation. The store path
	// changes immediately, but FilePreview only reloads the locally-tracked
	// displayedFilePath, so cancelling keeps the current (dirty) file open.
	$effect(() => {
		const nextFilePath = panelMode === 'file' ? panelFilePath : null;
		if (nextFilePath === displayedFilePath) return;
		if (displayedFilePath !== null && nextFilePath !== null && dirty) {
			if (!window.confirm('Discard unsaved changes?')) {
				return;
			}
		}
		displayedFilePath = nextFilePath;
	});

	$effect(() => {
		const workspace = shellStore.workspacePath;
		const filePath = panelMode === 'file' ? panelFilePath : null;
		if (!filePath) {
			fileCaptureRun += 1;
			return;
		}
		void captureFileContext(workspace, filePath);
	});

	$effect(() => {
		// Re-run whenever a focus is requested or the panel opens. The input may
		// already be mounted (panel was visible) so handle it here too; if it is
		// still mounting, trackAddressInput will pick up the same request id.
		const requestId = shellStore.addressBarFocusRequestId;
		const open = panelOpen;
		if (!requestId || !open) return;
		void tick().then(applyAddressFocus);
	});

	$effect(() => {
		const workspacePath = shellStore.workspacePath;
		const query = addressQuery;
		if (!addressEditing || !query) {
			fileOptionSeq += 1;
			fileOptions = [];
			fileOptionsLoading = false;
			return;
		}

		const seq = ++fileOptionSeq;
		fileOptionsLoading = true;
		void loadWebPanelFileOptions(workspacePath, query)
			.then((options) => {
				if (seq !== fileOptionSeq) return;
				fileOptions = options;
				if (addressOptionHighlight >= 1 + options.length) {
					addressOptionHighlight = 0;
				}
			})
			.catch(() => {
				if (seq !== fileOptionSeq) return;
				fileOptions = [];
				addressOptionHighlight = 0;
			})
			.finally(() => {
				if (seq === fileOptionSeq) fileOptionsLoading = false;
			});
	});
</script>

<svelte:window onkeydown={handlePanelKeydown} />

<div class="web-panel" class:open={panelOpen} aria-hidden={!panelOpen}>
	<div
		class="web-panel-inner content-panel-surface"
		class:pane-focus-active={shellStore.focusedPane === 'web' && panelOpen}
	>
		<!-- svelte-ignore a11y_no_static_element_interactions -->
		<header class="web-panel-toolbar" onmousedown={handlePanelMouseDown}>
			{#if panelMode === 'url'}
				<div class="nav-actions">
					<button
						type="button"
						class="icon-button"
						disabled={!canGoBack}
						onclick={onBack}
						aria-label="Back"
					>
						<ArrowLeft size={16} />
					</button>
					<button
						type="button"
						class="icon-button"
						disabled={!canGoForward}
						onclick={onForward}
						aria-label="Forward"
					>
						<ArrowRight size={16} />
					</button>
					<button
						type="button"
						class="icon-button"
						disabled={!showWebview}
						onclick={onReload}
						aria-label="Reload"
					>
						<RotateCw size={16} class={loading ? 'spin' : ''} />
					</button>
					<button
						type="button"
						class="icon-button"
						disabled={!showWebview || capturingContext}
						onclick={() => void capturePageContext({ announce: true })}
						aria-label="Add page to chat context"
						title="Add page to next message"
					>
						<FileText
							size={16}
							class={announcingContextCapture ? 'context-capture-pulse' : ''}
						/>
					</button>
				</div>
			{/if}
			<div class="url-field">
				{#if panelMode === 'file' && displayedFilePath}
					<span class="page-title">
						{displayedFilePath.split(/[/\\]/).pop()}{#if dirty}<span
								class="dirty-dot"
								aria-label="Unsaved changes"
							>
								•</span
							>{/if}
					</span>
					<span class="file-path-display" title={displayedFilePath}
						>{displayedFilePath}</span
					>
				{:else}
					{#if pageTitle}
						<span class="page-title">{pageTitle}</span>
					{/if}
					<input
						use:trackAddressInput
						class="address-input"
						type="text"
						inputmode="search"
						role="combobox"
						spellcheck="false"
						autocapitalize="off"
						autocomplete="off"
						placeholder="Search web or open workspace file"
						bind:value={addressInput}
						oninput={onAddressInput}
						onfocus={onAddressFocus}
						onblur={onAddressBlur}
						onkeydown={onAddressKeydown}
						aria-label="Web panel address"
						aria-expanded={showAddressOptions}
						aria-controls="web-panel-address-options"
					/>
					{#if showAddressOptions}
						<div
							id="web-panel-address-options"
							class="address-options"
							role="listbox"
							tabindex="-1"
							onmousedown={keepAddressFocus}
						>
							<button
								type="button"
								class:highlighted={addressOptionHighlight === 0}
								class="address-option"
								role="option"
								aria-selected={addressOptionHighlight === 0}
								onmouseenter={() => (addressOptionHighlight = 0)}
								onclick={selectWebOption}
							>
								<span class="address-option-label">{webOptionVerb}</span>
								<span class="address-option-detail">{addressQuery}</span>
							</button>
							{#each fileOptions as filePath, index}
								<button
									type="button"
									class:highlighted={addressOptionHighlight === index + 1}
									class="address-option"
									role="option"
									aria-selected={addressOptionHighlight === index + 1}
									title={filePath}
									onmouseenter={() => (addressOptionHighlight = index + 1)}
									onclick={() => selectFileOption(filePath)}
								>
									<span class="address-option-label">Open file</span>
									<span class="address-option-detail">{filePath}</span>
								</button>
							{/each}
							{#if fileOptionsLoading && fileOptions.length === 0}
								<div class="address-option address-option-muted">
									Searching workspace files...
								</div>
							{/if}
						</div>
					{/if}
				{/if}
			</div>
			{#if panelMode === 'file' && editorState}
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
		{#if contextMessage}
			<div class="web-panel-context-status" role="status">{contextMessage}</div>
		{/if}
		<!-- svelte-ignore a11y_no_static_element_interactions -->
		<div class="web-panel-content" onmousedown={handlePanelMouseDown}>
			{#if showFilePreview && displayedFilePath}
				<FilePreview
					workspacePath={shellStore.workspacePath}
					filePath={displayedFilePath}
					onEditorState={(state) => (editorState = state)}
				/>
			{:else if showWebview}
				<!-- Electron webview tag; inert in plain browser dev without Electron. -->
				<webview bind:this={webviewEl} class="web-panel-view"></webview>
			{/if}
		</div>
	</div>
</div>

<style>
	.web-panel {
		flex: 0 0 auto;
		width: 0;
		min-width: 0;
		height: 100%;
		overflow: hidden;
		pointer-events: none;
		box-sizing: border-box;
		transition: width var(--duration-fast) var(--ease-smooth);
	}

	.web-panel.open {
		width: var(--web-panel-slot-width);
		pointer-events: auto;
	}

	.web-panel-inner {
		width: var(--web-panel-width);
		height: calc(100% - (2 * var(--content-panel-inset)));
		display: flex;
		flex-direction: column;
		margin: var(--content-panel-inset);
		margin-left: 0;
		box-sizing: border-box;
		overflow: hidden;
		transition:
			border-color var(--duration-fast) var(--ease-smooth),
			box-shadow var(--duration-fast) var(--ease-smooth);
	}

	.web-panel-toolbar {
		display: flex;
		align-items: center;
		gap: 8px;
		padding: 8px 10px;
		border-bottom: 1px solid var(--border-soft);
		background: rgba(250, 250, 249, 0.95);
		min-height: 44px;
	}

	.web-panel-context-status {
		padding: 6px 12px;
		border-bottom: 1px solid var(--border-soft);
		background: rgba(239, 246, 255, 0.78);
		color: #1d4ed8;
		font-size: 11px;
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

	.icon-button:disabled {
		opacity: 0.35;
		cursor: default;
	}

	.url-field {
		position: relative;
		flex: 1;
		min-width: 0;
		display: flex;
		flex-direction: column;
		gap: 1px;
		padding: 0 4px;
	}

	.page-title {
		font-size: 12px;
		font-weight: 600;
		color: var(--text-main);
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.dirty-dot {
		color: var(--accent, #2563eb);
		font-weight: 700;
	}

	.address-input {
		width: 100%;
		min-width: 0;
		border: none;
		background: transparent;
		font-size: 11px;
		color: var(--text-muted);
		padding: 0;
		outline: none;
	}

	.address-input:focus {
		color: var(--text-main);
	}

	.address-input::placeholder {
		color: var(--text-muted);
		opacity: 0.7;
	}

	.address-options {
		position: absolute;
		z-index: 5;
		top: calc(100% + 8px);
		left: 0;
		right: 0;
		display: flex;
		flex-direction: column;
		gap: 2px;
		max-height: min(320px, calc(100vh - 120px));
		overflow: auto;
		padding: 6px;
		border: 1px solid var(--border-soft);
		border-radius: 12px;
		background: rgba(255, 255, 255, 0.98);
		box-shadow: 0 18px 48px rgba(15, 23, 42, 0.18);
	}

	.address-option {
		display: grid;
		grid-template-columns: minmax(72px, auto) minmax(0, 1fr);
		align-items: center;
		gap: 8px;
		width: 100%;
		min-width: 0;
		padding: 7px 8px;
		border: none;
		border-radius: 8px;
		background: transparent;
		color: var(--text-main);
		text-align: left;
		cursor: pointer;
	}

	.address-option.highlighted,
	.address-option:hover {
		background: rgba(15, 23, 42, 0.06);
	}

	.address-option-label {
		font-size: 10px;
		font-weight: 700;
		text-transform: uppercase;
		letter-spacing: 0.04em;
		color: var(--text-muted);
		white-space: nowrap;
	}

	.address-option-detail {
		min-width: 0;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		font-size: 12px;
	}

	.address-option-muted {
		grid-template-columns: 1fr;
		font-size: 11px;
		color: var(--text-muted);
		cursor: default;
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

	.web-panel-content {
		flex: 1;
		min-height: 0;
		position: relative;
		background: #fff;
	}

	.web-panel-view {
		display: inline-flex;
		width: 100%;
		height: 100%;
		border: none;
	}

	:global(.spin) {
		animation: web-panel-spin 0.8s linear infinite;
	}

	:global(.context-capture-pulse) {
		transform-origin: center;
		animation: web-panel-context-capture 0.9s var(--ease-smooth) infinite;
	}

	@keyframes web-panel-spin {
		to {
			transform: rotate(360deg);
		}
	}

	@keyframes web-panel-context-capture {
		0%,
		100% {
			opacity: 1;
			transform: scale(1);
		}
		50% {
			opacity: 0.58;
			transform: scale(0.82);
		}
	}

	@media (prefers-reduced-motion: reduce) {
		:global(.context-capture-pulse) {
			animation: none;
		}

		.web-panel {
			transition: none;
		}

		.web-panel-inner {
			transition: none;
		}
	}

	@media (max-width: 900px) {
		.web-panel {
			position: fixed;
			inset: 0;
			z-index: 40;
			width: 100% !important;
			transition: none;
			pointer-events: none;
		}

		.web-panel.open {
			pointer-events: auto;
		}

		.web-panel-inner {
			width: 100%;
			height: 100%;
			margin: 0;
			border: none;
			border-radius: 0;
			box-shadow: none;
			transform: translateX(100%);
			transition: transform var(--duration-fast) var(--ease-smooth);
		}

		.web-panel.open .web-panel-inner {
			transform: translateX(0);
		}
	}
</style>
