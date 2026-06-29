<script lang="ts">
	import { ArrowLeft, ArrowRight, RotateCcw, RotateCw, Save, X } from '@lucide/svelte';
	import FilePreview from '$lib/components/FilePreview.svelte';
	import { shellStore } from '$lib/stores/shell.svelte';
	import { isWebPanelUrl, normalizeUserUrl, openLink } from '$lib/open-link';
	import { openExternalLink } from '$lib/external-link';

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
	};

	type FileEditorState = {
		dirty: boolean;
		saving: boolean;
		saveError: string | null;
		save: () => Promise<void>;
		revert: () => void;
	};

	let webviewEl = $state<WebviewElement | null>(null);
	let addressInputEl = $state<HTMLInputElement | null>(null);
	let canGoBack = $state(false);
	let canGoForward = $state(false);
	let loading = $state(false);
	let addressInput = $state('');
	let pageTitle = $state('');
	let webviewSessionId = $state<string | null>(null);
	let webviewLoadedUrl = $state<string | null>(null);
	let addressEditing = $state(false);
	let editorState = $state<FileEditorState | null>(null);
	let displayedFilePath = $state<string | null>(null);

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

	function submitAddress() {
		const normalized = normalizeUserUrl(addressInput);
		if (!normalized) return;
		addressEditing = false;
		shellStore.navigateWebPanel(normalized);
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

	function onAddressFocus() {
		addressEditing = true;
		shellStore.setFocusedPane('web');
	}

	function onAddressBlur() {
		addressEditing = false;
		syncAddressFromNavigation();
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
		const onStartLoading = () => {
			loading = true;
		};
		const onStopLoading = () => {
			loading = false;
			updateNavigationState();
		};
		const onTitleUpdated = (event: Event & { title?: string }) => {
			pageTitle = event.title ?? '';
		};
		const onFocus = () => {
			shellStore.setFocusedPane('web');
		};

		el.addEventListener('did-navigate', onNavigate);
		el.addEventListener('did-navigate-in-page', onNavigate);
		el.addEventListener('did-start-loading', onStartLoading);
		el.addEventListener('did-stop-loading', onStopLoading);
		el.addEventListener('page-title-updated', onTitleUpdated);
		el.addEventListener('new-window', onNewWindow);
		el.addEventListener('focus', onFocus);

		return () => {
			el.removeEventListener('did-navigate', onNavigate);
			el.removeEventListener('did-navigate-in-page', onNavigate);
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
		const el = webviewEl;
		if (!el) return;
		return attachWebview(el);
	});

	$effect(() => {
		// Re-sync the address bar whenever the panel URL changes.
		void panelUrl;
		if (!addressEditing) {
			syncAddressFromNavigation();
		}
	});

	$effect(() => {
		if (!shellStore.hasWebPanelForSession) {
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
		const requestId = shellStore.addressBarFocusRequestId;
		if (!requestId || !panelOpen) return;
		queueMicrotask(() => {
			shellStore.setFocusedPane('web');
			addressInputEl?.focus();
			addressInputEl?.select();
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
						bind:this={addressInputEl}
						class="address-input"
						type="text"
						inputmode="url"
						spellcheck="false"
						autocapitalize="off"
						autocomplete="off"
						placeholder="Enter a URL"
						bind:value={addressInput}
						onfocus={onAddressFocus}
						onblur={onAddressBlur}
						onkeydown={onAddressKeydown}
						aria-label="Web panel address"
					/>
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

	@keyframes web-panel-spin {
		to {
			transform: rotate(360deg);
		}
	}

	@media (prefers-reduced-motion: reduce) {
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
