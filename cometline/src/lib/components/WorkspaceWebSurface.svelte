<script lang="ts">
	import type { WebContext } from '$lib/actions/start-chat';

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

	type CachedPageContext = {
		sessionKey: string;
		url: string;
		title: string;
		content: string;
		capturedAt: number;
	};

	type NavigationState = {
		url: string;
		title: string;
		canGoBack: boolean;
		canGoForward: boolean;
		loading: boolean;
	};

	let {
		url,
		sessionKey,
		onNavigationState,
		onFocus,
		onNewWindow,
		onCapturingChange
	}: {
		url: string | null;
		sessionKey: string | null;
		onNavigationState: (state: NavigationState) => void;
		onFocus: () => void;
		onNewWindow: (url: string) => void;
		onCapturingChange: (capturing: boolean) => void;
	} = $props();

	const PAGE_CONTEXT_CACHE_TTL_MS = 10_000;

	let webviewEl = $state<WebviewElement | null>(null);
	let loadedUrl: string | null = null;
	let loadedSessionKey: string | null = null;
	let loading = $state(false);
	let title = $state('');
	let cachedContext = $state<CachedPageContext | null>(null);
	let captureRun = 0;
	let capturing = false;

	function currentUrl() {
		const fallback = url ?? '';
		try {
			return String(webviewEl?.getURL() || fallback).trim();
		} catch {
			return fallback;
		}
	}

	function publishNavigationState() {
		const el = webviewEl;
		if (!el) return;
		try {
			title = el.getTitle() || '';
		} catch {
			title = '';
		}
		onNavigationState({
			url: currentUrl(),
			title,
			canGoBack: el.canGoBack(),
			canGoForward: el.canGoForward(),
			loading
		});
	}

	function attachWebview(el: WebviewElement) {
		el.setAttribute('sandbox', 'allow-scripts allow-same-origin allow-popups allow-forms');
		const onNavigate = () => publishNavigationState();
		const onInPageNavigate = () => {
			loading = false;
			publishNavigationState();
		};
		const onStartLoading = (event: Event & { isMainFrame?: boolean }) => {
			if (event.isMainFrame === false) return;
			loading = true;
			publishNavigationState();
		};
		const onStopLoading = () => {
			loading = false;
			publishNavigationState();
		};
		const onFrameFinishLoad = (event: Event & { isMainFrame?: boolean }) => {
			if (event.isMainFrame === false) return;
			loading = false;
			publishNavigationState();
		};
		const onFailLoad = () => {
			loading = false;
			publishNavigationState();
		};
		const onTitleUpdated = (event: Event & { title?: string }) => {
			title = event.title ?? '';
			publishNavigationState();
		};
		const handleNewWindow = (event: Event & { url?: string; preventDefault?: () => void }) => {
			event.preventDefault?.();
			if (event.url) onNewWindow(event.url);
		};

		el.addEventListener('did-navigate', onNavigate);
		el.addEventListener('did-navigate-in-page', onInPageNavigate);
		el.addEventListener('did-start-loading', onStartLoading);
		el.addEventListener('did-stop-loading', onStopLoading);
		el.addEventListener('did-frame-finish-load', onFrameFinishLoad);
		el.addEventListener('did-fail-load', onFailLoad);
		el.addEventListener('page-title-updated', onTitleUpdated);
		el.addEventListener('new-window', handleNewWindow);
		el.addEventListener('focus', onFocus);

		return () => {
			el.removeEventListener('did-navigate', onNavigate);
			el.removeEventListener('did-navigate-in-page', onInPageNavigate);
			el.removeEventListener('did-start-loading', onStartLoading);
			el.removeEventListener('did-stop-loading', onStopLoading);
			el.removeEventListener('did-frame-finish-load', onFrameFinishLoad);
			el.removeEventListener('did-fail-load', onFailLoad);
			el.removeEventListener('page-title-updated', onTitleUpdated);
			el.removeEventListener('new-window', handleNewWindow);
			el.removeEventListener('focus', onFocus);
			try {
				el.stop();
			} catch {
				// Ignore teardown errors from a guest that has already exited.
			}
		};
	}

	export function navigateBack(): boolean {
		if (!webviewEl?.canGoBack()) return false;
		webviewEl.goBack();
		return true;
	}

	export function navigateForward(): boolean {
		if (!webviewEl?.canGoForward()) return false;
		webviewEl.goForward();
		return true;
	}

	export function reload() {
		webviewEl?.reload();
	}

	export async function captureContext(source?: string): Promise<WebContext | null> {
		const el = webviewEl;
		const capturedSessionKey = sessionKey;
		const expectedUrl = source ?? currentUrl();
		if (!el || !capturedSessionKey || !expectedUrl || capturing) return null;
		if (source && currentUrl() !== expectedUrl) return null;

		const cached = cachedContext;
		if (
			cached &&
			cached.sessionKey === capturedSessionKey &&
			cached.url === expectedUrl &&
			Date.now() - cached.capturedAt < PAGE_CONTEXT_CACHE_TTL_MS
		) {
			return { kind: 'page', title: cached.title, source: cached.url, content: cached.content };
		}

		const run = ++captureRun;
		capturing = true;
		onCapturingChange(true);
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
			const pageUrl = String(page?.url || currentUrl()).trim();
			const content = String(page?.content || '').trim();
			if (run !== captureRun || pageUrl !== expectedUrl || !pageUrl.startsWith('http')) return null;
			if (!content) return null;
			const context = {
				kind: 'page' as const,
				title: String(page?.title || title).trim(),
				source: pageUrl,
				content
			};
			cachedContext = { sessionKey: capturedSessionKey, url: pageUrl, ...context, capturedAt: Date.now() };
			return context;
		} finally {
			capturing = false;
			onCapturingChange(false);
		}
	}

	$effect(() => {
		const el = webviewEl;
		if (!el) return;
		return attachWebview(el);
	});

	$effect(() => {
		const el = webviewEl;
		if (!el || !url || !sessionKey) return;
		if (loadedSessionKey === sessionKey && loadedUrl === url) return;
		el.src = url;
		loadedSessionKey = sessionKey;
		loadedUrl = url;
	});
</script>

{#if url}
	<!-- Electron webview tag; inert in plain browser development without Electron. -->
	<webview bind:this={webviewEl} class="workspace-panel-view"></webview>
{/if}

<style>
	.workspace-panel-view {
		display: inline-flex;
		width: 100%;
		height: 100%;
		border: none;
	}
</style>
