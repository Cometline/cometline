<script lang="ts">
	import { untrack } from 'svelte';
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
		isLoadingMainFrame(): boolean;
		isCurrentlyAudible(): boolean;
		isAudioMuted(): boolean;
		setAudioMuted(muted: boolean): void;
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
		onNewWindow
	}: {
		url: string | null;
		sessionKey: string | null;
		onNavigationState: (state: NavigationState) => void;
		onFocus: () => void;
		onNewWindow: (url: string) => void;
	} = $props();

	const PAGE_CONTEXT_CACHE_TTL_MS = 10_000;

	let webviewEl = $state<WebviewElement | null>(null);
	let loadedUrl: string | null = null;
	let loadedSessionKey: string | null = null;
	let cachedContext = $state<CachedPageContext | null>(null);
	let captureRun = 0;
	let loadingTimer: ReturnType<typeof setTimeout> | null = null;
	export const pageState = $state({
		url: '',
		title: '',
		canGoBack: false,
		canGoForward: false,
		loading: false,
		showLoading: false,
		loadError: null as string | null,
		ready: false,
		mediaPlaying: false,
		audible: false,
		muted: false,
		capturing: false
	});

	function setLoading(loading: boolean) {
		pageState.loading = loading;
		if (loading) {
			pageState.loadError = null;
			if (!loadingTimer && !pageState.showLoading) {
				loadingTimer = setTimeout(() => {
					loadingTimer = null;
					pageState.showLoading = true;
				}, 150);
			}
		} else {
			if (loadingTimer) clearTimeout(loadingTimer);
			loadingTimer = null;
			pageState.showLoading = false;
		}
	}

	function syncAudio() {
		if (!webviewEl || !pageState.ready) return;
		try {
			pageState.muted = webviewEl.isAudioMuted();
			pageState.audible = !pageState.muted && webviewEl.isCurrentlyAudible();
		} catch {
			pageState.audible = false;
		}
	}

	export function toggleAudioMuted() {
		if (!webviewEl || !pageState.ready) return;
		try {
			webviewEl.setAudioMuted(!webviewEl.isAudioMuted());
			syncAudio();
		} catch {
			// The guest may have exited between the pointer event and the native call.
		}
	}

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
		if (!el || !pageState.ready) return;
		try {
			Object.assign(pageState, {
				title: el.getTitle() || '',
				url: currentUrl(),
				canGoBack: el.canGoBack(),
				canGoForward: el.canGoForward()
			});
		} catch {
			return;
		}
		// Loading events can still report the old document after a new src was requested.
		if (pageState.url === loadedUrl) onNavigationState(pageState);
	}

	function attachWebview(el: WebviewElement) {
		el.setAttribute('sandbox', 'allow-scripts allow-same-origin allow-popups allow-forms');
		let audioTimer: ReturnType<typeof setInterval> | null = null;
		const onDomReady = () => {
			pageState.ready = true;
			syncAudio();
			// Web Audio and mute changes do not always emit media playback events.
			if (!audioTimer) audioTimer = setInterval(syncAudio, 750);
			publishNavigationState();
		};
		const onMediaStarted = () => {
			pageState.mediaPlaying = true;
			syncAudio();
		};
		const onMediaPaused = () => {
			pageState.mediaPlaying = false;
			syncAudio();
		};
		const onStartNavigation = (
			event: Event & { isMainFrame?: boolean; isInPlace?: boolean }
		) => {
			if (event.isMainFrame && !event.isInPlace) setLoading(true);
		};
		const onRenderProcessGone = () => {
			captureRun += 1;
			pageState.ready = false;
			pageState.audible = false;
			pageState.mediaPlaying = false;
			setLoading(false);
			pageState.loadError = 'Page stopped unexpectedly. Reload to try again.';
			if (audioTimer) clearInterval(audioTimer);
			audioTimer = null;
		};
		const handleFocus = () => onFocus();
		const rememberGuestLocation = (event: Event & { url?: string }) => {
			loadedUrl = event.url || currentUrl();
			loadedSessionKey = sessionKey;
		};
		const onNavigate = (event: Event & { url?: string }) => {
			rememberGuestLocation(event);
			publishNavigationState();
		};
		const onInPageNavigate = (event: Event & { url?: string; isMainFrame?: boolean }) => {
			if (event.isMainFrame === false) return;
			rememberGuestLocation(event);
			publishNavigationState();
		};
		const onStartLoading = () => {
			if (!pageState.ready) return;
			try {
				if (el.isLoadingMainFrame()) setLoading(true);
			} catch {
				return;
			}
			publishNavigationState();
		};
		const onStopLoading = () => {
			setLoading(false);
			publishNavigationState();
		};
		const onFrameFinishLoad = (event: Event & { isMainFrame?: boolean }) => {
			if (!event.isMainFrame) return;
			setLoading(false);
			publishNavigationState();
		};
		const onFailLoad = (
			event: Event & { isMainFrame?: boolean; errorCode?: number; errorDescription?: string }
		) => {
			if (!event.isMainFrame || event.errorCode === -3) return;
			setLoading(false);
			pageState.loadError =
				event.errorDescription || 'Could not load this page. Reload to try again.';
			publishNavigationState();
		};
		const onTitleUpdated = (event: Event & { title?: string }) => {
			pageState.title = event.title ?? '';
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
		el.addEventListener('focus', handleFocus);
		el.addEventListener('dom-ready', onDomReady);
		el.addEventListener('did-start-navigation', onStartNavigation);
		el.addEventListener('media-started-playing', onMediaStarted);
		el.addEventListener('media-paused', onMediaPaused);
		el.addEventListener('render-process-gone', onRenderProcessGone);

		return () => {
			captureRun += 1;
			pageState.ready = false;
			pageState.audible = false;
			setLoading(false);
			if (audioTimer) clearInterval(audioTimer);
			el.removeEventListener('did-navigate', onNavigate);
			el.removeEventListener('did-navigate-in-page', onInPageNavigate);
			el.removeEventListener('did-start-loading', onStartLoading);
			el.removeEventListener('did-stop-loading', onStopLoading);
			el.removeEventListener('did-frame-finish-load', onFrameFinishLoad);
			el.removeEventListener('did-fail-load', onFailLoad);
			el.removeEventListener('page-title-updated', onTitleUpdated);
			el.removeEventListener('new-window', handleNewWindow);
			el.removeEventListener('focus', handleFocus);
			el.removeEventListener('dom-ready', onDomReady);
			el.removeEventListener('did-start-navigation', onStartNavigation);
			el.removeEventListener('media-started-playing', onMediaStarted);
			el.removeEventListener('media-paused', onMediaPaused);
			el.removeEventListener('render-process-gone', onRenderProcessGone);
			try {
				el.stop();
			} catch {
				// Ignore teardown errors from a guest that has already exited.
			}
		};
	}

	export function navigateBack(): boolean {
		try {
			if (!pageState.ready || !webviewEl?.canGoBack()) return false;
			webviewEl.goBack();
			return true;
		} catch {
			return false;
		}
	}

	export function navigateForward(): boolean {
		try {
			if (!pageState.ready || !webviewEl?.canGoForward()) return false;
			webviewEl.goForward();
			return true;
		} catch {
			return false;
		}
	}

	export function reload() {
		try {
			setLoading(true);
			webviewEl?.reload();
		} catch {
			setLoading(false);
			pageState.loadError = 'Could not reload this page. Try again.';
		}
	}

	export function focus() {
		webviewEl?.focus();
		onFocus();
	}

	export async function captureContext(source?: string): Promise<WebContext | null> {
		const el = webviewEl;
		const capturedSessionKey = sessionKey;
		const expectedUrl = source ?? currentUrl();
		if (!el || !pageState.ready || !capturedSessionKey || !expectedUrl || pageState.capturing)
			return null;
		if (source && currentUrl() !== expectedUrl) return null;

		const cached = cachedContext;
		if (
			cached &&
			cached.sessionKey === capturedSessionKey &&
			cached.url === expectedUrl &&
			Date.now() - cached.capturedAt < PAGE_CONTEXT_CACHE_TTL_MS
		) {
			return {
				kind: 'page',
				title: cached.title,
				source: cached.url,
				content: cached.content
			};
		}

		const run = ++captureRun;
		pageState.capturing = true;
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
			if (run !== captureRun || pageUrl !== expectedUrl || !pageUrl.startsWith('http'))
				return null;
			if (!content) return null;
			const context = {
				kind: 'page' as const,
				title: String(page?.title || pageState.title).trim(),
				source: pageUrl,
				content
			};
			cachedContext = {
				sessionKey: capturedSessionKey,
				url: pageUrl,
				...context,
				capturedAt: Date.now()
			};
			return context;
		} catch (error) {
			if (run !== captureRun) return null;
			throw error;
		} finally {
			pageState.capturing = false;
		}
	}

	$effect(() => {
		const el = webviewEl;
		if (!el) return;
		// Callback updates must not tear down a live guest's event subscriptions.
		return untrack(() => attachWebview(el));
	});

	$effect(() => {
		const el = webviewEl;
		if (!el || !url || !sessionKey) return;
		if (loadedSessionKey === sessionKey && loadedUrl === url) return;
		untrack(() => setLoading(true));
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
