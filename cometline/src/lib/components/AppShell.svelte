<script lang="ts">
	import { onMount } from 'svelte';
	import { slide } from 'svelte/transition';
	import { goto } from '$app/navigation';
	import { PanelLeft } from '@lucide/svelte';
	import Sidebar from './Sidebar.svelte';
	import RuntimeOverlay from './RuntimeOverlay.svelte';
	import SettingsModal from './SettingsModal.svelte';
	import InboxDrawer from './inbox/InboxDrawer.svelte';
	import IntroAnimation from './IntroAnimation.svelte';
	import SetupWizard from './onboarding/SetupWizard.svelte';
	import UpdateButton from './UpdateButton.svelte';
	import MemoryToast from './MemoryToast.svelte';
	import AppToast from './AppToast.svelte';
	import ConfirmActionModal from './ConfirmActionModal.svelte';
	import WebPanel from './WebPanel.svelte';
	import TerminalPanel from './TerminalPanel.svelte';
	import Tooltip from './Tooltip.svelte';
	import { getSession, updateSession } from '$lib/client/cometmind';
	import { shellStore } from '$lib/stores/shell.svelte';
	import { sessionStore } from '$lib/stores/session.svelte';
	import { settingsStore } from '$lib/stores/settings.svelte';
	import { inboxStore } from '$lib/stores/inbox.svelte';
	import { terminalStore } from '$lib/stores/terminal.svelte';
	import { startNewChat } from '$lib/actions/new-chat';
	import { openSettings } from '$lib/actions/open-settings';
	import { navigateAdjacentSession } from '$lib/actions/navigate-adjacent-session';
	import { navigateSessionHistory, navigateToRecentSession } from '$lib/actions/navigate-session-history';
	import { navigateToSession } from '$lib/actions/navigate-to-session';
	import { narrowViewportQuery, subscribeNarrowViewport } from '$lib/layout/narrow-viewport';
	import {
		clampWebPanelWidth,
		resolveWebPanelRatio,
		widthFromRatio,
		widthToRatio
	} from '$lib/layout/web-panel-width';
	import { shouldUseWebPanelHistory } from '$lib/navigation/focus-nav';
	import { matchesShortcut, isReloadShortcut, type ShortcutAction } from '$lib/keyboard-shortcuts';
	import type { Session } from '$lib/types';

	const FALLBACK_SIDEBAR_DURATION = 360;

	let { children }: { children: import('svelte').Snippet } = $props();

	let sidebarRef = $state<{ focusSearch: () => void } | null>(null);
	let webPanelRef = $state<{ navigateBack: () => void; navigateForward: () => void } | null>(
		null
	);
	let contentRowRef = $state<HTMLDivElement | null>(null);
	let closeConfirmOpen = $state(false);
	let reloadConfirmOpen = $state(false);
	/** True only after the confirm dialog has settled, so a duplicated Cmd+R delivery cannot open+confirm in one press. */
	let reloadConfirmArmed = $state(false);
	let pendingRename = $state<Session | null>(null);
	let renameTitle = $state('');

	let activeSessionId = $derived(sessionStore.current?.id ?? null);
	let titlebarSessionTitle = $derived.by(() => {
		const session = sessionStore.current;
		if (!session) return '';
		const title = session.title?.trim();
		return title || 'Untitled';
	});
	let titlebarSessionTitleAttr = $derived(
		titlebarSessionTitle ? `${titlebarSessionTitle} — Double-click to rename` : ''
	);

	function startRenameFromTitlebar() {
		const session = sessionStore.current;
		if (!session) return;
		renameTitle = session.title || '';
		pendingRename = session;
	}

	function cancelRename() {
		pendingRename = null;
	}

	async function confirmRename() {
		if (!pendingRename) return;
		const updated = await updateSession(pendingRename.id, { title: renameTitle.trim() });
		sessionStore.updateSession(updated);
		pendingRename = null;
	}

	$effect(() => {
		window.electronAPI?.setSessionNavigationSuspended?.(shellStore.settingsOpen);
	});

	$effect(() => {
		void activeSessionId;
		shellStore.onActiveSessionChange();
	});

	function isCmdW(event: KeyboardEvent) {
		return (
			event.metaKey &&
			!event.ctrlKey &&
			!event.altKey &&
			!event.shiftKey &&
			event.key.toLowerCase() === 'w'
		);
	}

	function hideMainWindow() {
		closeConfirmOpen = false;
		window.electronAPI?.confirmCloseWindow?.();
	}

	function handleRequestCloseWindow() {
		if (closeConfirmOpen) {
			hideMainWindow();
			return;
		}
		if (inboxStore.drawerOpen) {
			inboxStore.closeDrawer();
			return;
		}
		if (shellStore.workspacePanelOpen) {
			shellStore.closeWorkspacePanel();
			return;
		}
		if (settingsStore.settings.app.confirmCloseOnCmdW === false) {
			hideMainWindow();
			return;
		}
		closeConfirmOpen = true;
	}

	function confirmReload() {
		reloadConfirmOpen = false;
		reloadConfirmArmed = false;
		window.location.reload();
	}

	function handleRequestReload() {
		if (reloadConfirmOpen) {
			if (!reloadConfirmArmed) return;
			confirmReload();
			return;
		}
		reloadConfirmOpen = true;
		reloadConfirmArmed = false;
		queueMicrotask(() => {
			if (reloadConfirmOpen) reloadConfirmArmed = true;
		});
	}

	async function alwaysCloseWithoutConfirm() {
		// Persist preference in the background — don't block hiding the window on IPC.
		void settingsStore.saveConfirmCloseOnCmdW(false).catch(() => {});
		hideMainWindow();
	}

	// Single source of truth for what each global shortcut does, so it behaves
	// identically whether the key arrives via DOM keydown (renderer focused) or
	// via IPC forwarded from the webview guest (web panel focused).
	function runShortcutAction(action: ShortcutAction) {
		switch (action) {
			case 'toggleSidebar':
				shellStore.toggleSidebar();
				return;
			case 'toggleWebPanel':
				shellStore.toggleWebPanel();
				return;
			case 'openWebPanel':
				shellStore.openWebPanelFromShortcut();
				return;
			case 'openGitPanel':
				shellStore.openGitChangesPanel();
				return;
			case 'openTerminal':
				shellStore.requestTerminalFocus();
				return;
			case 'navigateBack':
				if (shouldUseWebPanelHistory(shellStore.webPanelOpen)) {
					webPanelRef?.navigateBack();
				} else {
					navigateSessionHistory('back');
				}
				return;
			case 'navigateForward':
				if (shouldUseWebPanelHistory(shellStore.webPanelOpen)) {
					webPanelRef?.navigateForward();
				} else {
					navigateSessionHistory('forward');
				}
				return;
			case 'openSettings':
				openSettings();
				return;
			case 'newChat':
				startNewChat();
				return;
			case 'focusSearch':
				shellStore.openSidebar();
				sidebarRef?.focusSearch();
				return;
			case 'previousSession':
				if (shellStore.settingsOpen) return;
				navigateAdjacentSession('prev');
				return;
			case 'nextSession':
				if (shellStore.settingsOpen) return;
				navigateAdjacentSession('next');
				return;
			case 'openJobs':
				if (shellStore.settingsOpen) shellStore.closeSettings();
				inboxStore.closeDrawer();
				void goto('/jobs');
				return;
			case 'openSkillDrafts':
				if (shellStore.settingsOpen) shellStore.closeSettings();
				inboxStore.closeDrawer();
				void goto('/skill-drafts');
				return;
			case 'openInbox':
				if (shellStore.settingsOpen) shellStore.closeSettings();
				inboxStore.toggleDrawer();
				return;
			case 'recentSession':
				if (shellStore.settingsOpen) shellStore.closeSettings();
				inboxStore.closeDrawer();
				navigateToRecentSession();
				return;
		}
	}

	onMount(() => {
		let resizeObserver: ResizeObserver | null = null;
		void terminalStore.initialize();

		if (narrowViewportQuery().matches) {
			shellStore.closeSidebar();
		}

		const unsubscribeNarrowViewport = subscribeNarrowViewport((narrow) => {
			if (narrow) {
				shellStore.closeSidebar();
			}
		});

		function onKeydown(event: KeyboardEvent) {
			const shortcuts = settingsStore.settings.shortcuts;

			if (closeConfirmOpen && event.key === 'Escape') {
				event.preventDefault();
				closeConfirmOpen = false;
				return;
			}
			if (reloadConfirmOpen && event.key === 'Escape') {
				event.preventDefault();
				reloadConfirmOpen = false;
				reloadConfirmArmed = false;
				return;
			}
			if (isCmdW(event)) {
				event.preventDefault();
				handleRequestCloseWindow();
				return;
			}
			if (
				isReloadShortcut({
					key: event.key,
					code: event.code,
					meta: event.metaKey,
					control: event.ctrlKey,
					alt: event.altKey,
					shift: event.shiftKey,
					isComposing: event.isComposing
				})
			) {
				event.preventDefault();
				handleRequestReload();
				return;
			}
			if (
				shellStore.focusedPane === 'terminal' &&
				shellStore.terminalPanelOpen &&
				!shellStore.settingsOpen &&
				!inboxStore.drawerOpen &&
				event.key === 'Escape'
			) {
				// Let terminal applications own Escape, but keep app shortcuts available.
				return;
			}
			if (matchesShortcut(event, shortcuts.closeSettings) && shellStore.settingsOpen) {
				event.preventDefault();
				shellStore.closeSettings();
				return;
			}
			if (matchesShortcut(event, shortcuts.toggleSidebar)) {
				event.preventDefault();
				runShortcutAction('toggleSidebar');
				return;
			}
			if (matchesShortcut(event, shortcuts.toggleWebPanel)) {
				event.preventDefault();
				runShortcutAction('toggleWebPanel');
				return;
			}
			if (matchesShortcut(event, shortcuts.openWebPanel)) {
				event.preventDefault();
				runShortcutAction('openWebPanel');
				return;
			}
			if (matchesShortcut(event, shortcuts.openGitPanel)) {
				event.preventDefault();
				runShortcutAction('openGitPanel');
				return;
			}
			if (matchesShortcut(event, shortcuts.openTerminal)) {
				event.preventDefault();
				runShortcutAction('openTerminal');
				return;
			}
			if (matchesShortcut(event, shortcuts.navigateBack)) {
				event.preventDefault();
				runShortcutAction('navigateBack');
				return;
			}
			if (matchesShortcut(event, shortcuts.navigateForward)) {
				event.preventDefault();
				runShortcutAction('navigateForward');
				return;
			}
			if (matchesShortcut(event, shortcuts.openSettings)) {
				event.preventDefault();
				runShortcutAction('openSettings');
				return;
			}
			if (matchesShortcut(event, shortcuts.newChat)) {
				event.preventDefault();
				runShortcutAction('newChat');
				return;
			}
			if (matchesShortcut(event, shortcuts.focusSearch)) {
				event.preventDefault();
				runShortcutAction('focusSearch');
				return;
			}
			if (matchesShortcut(event, shortcuts.openJobs)) {
				event.preventDefault();
				runShortcutAction('openJobs');
				return;
			}
			if (matchesShortcut(event, shortcuts.openSkillDrafts)) {
				event.preventDefault();
				runShortcutAction('openSkillDrafts');
				return;
			}
			if (matchesShortcut(event, shortcuts.openInbox)) {
				event.preventDefault();
				runShortcutAction('openInbox');
				return;
			}
			if (matchesShortcut(event, shortcuts.recentSession)) {
				event.preventDefault();
				runShortcutAction('recentSession');
				return;
			}
			if (shellStore.settingsOpen) return;
			if (matchesShortcut(event, shortcuts.previousSession)) {
				event.preventDefault();
				runShortcutAction('previousSession');
				return;
			}
			if (matchesShortcut(event, shortcuts.nextSession)) {
				event.preventDefault();
				runShortcutAction('nextSession');
				return;
			}
		}

		window.addEventListener('keydown', onKeydown, true);

		const unsubscribeNavigate = window.electronAPI?.onNavigateSession?.((direction) => {
			if (shellStore.settingsOpen) return;
			navigateAdjacentSession(direction);
		});

		const unsubscribeCloseWebPanel = window.electronAPI?.onCloseWebPanel?.(() => {
			shellStore.closeWorkspacePanel();
		});

		const unsubscribeCloseInbox = window.electronAPI?.onCloseInbox?.(() => {
			inboxStore.closeDrawer();
		});

		const unsubscribeRequestCloseWindow = window.electronAPI?.onRequestCloseWindow?.(() => {
			handleRequestCloseWindow();
		});

		const unsubscribeRequestReload = window.electronAPI?.onRequestReload?.(() => {
			handleRequestReload();
		});

		const unsubscribeToggleWebPanel = window.electronAPI?.onToggleWebPanel?.(() => {
			if (shellStore.settingsOpen) return;
			shellStore.toggleWebPanel();
		});

		const unsubscribeOpenWebPanel = window.electronAPI?.onOpenWebPanel?.(() => {
			if (shellStore.settingsOpen) return;
			shellStore.openWebPanelFromShortcut();
		});

		// Shortcuts forwarded from the webview guest (web panel focused). Run the
		// same effects as the DOM keydown dispatcher above.
		const unsubscribeShortcutAction = window.electronAPI?.onShortcutAction?.((action) => {
			runShortcutAction(action);
		});

		// Onboarding surfaces replayed from the separate Settings window. They can
		// only render here (inside AppShell), so the Settings window forwards the
		// request over IPC and we open them against this window's shell store.
		const unsubscribeReplayIntro = window.electronAPI?.onReplayIntro?.(() => {
			shellStore.openIntro();
		});
		const unsubscribeRunSetupWizard = window.electronAPI?.onRunSetupWizard?.(() => {
			shellStore.openSetup();
		});

		function updateFullScreen(isFullScreen: boolean) {
			if (import.meta.env.DEV) {
				console.log('[AppShell] fullscreen state:', isFullScreen);
			}
			shellStore.setFullscreen(isFullScreen);
		}
		void window.electronAPI?.getFullScreen?.().then(updateFullScreen);
		const unsubscribeFullScreen = window.electronAPI?.onFullScreenChange?.(updateFullScreen);

		function onDomFullScreenChange() {
			updateFullScreen(Boolean(document.fullscreenElement));
		}
		document.addEventListener('fullscreenchange', onDomFullScreenChange);

		// Keep the preferred ratio applied as the content row changes (window
		// resize or sidebar open/close animation) so the web panel tracks
		// proportionally instead of locking to a stale absolute width.
		function applyPreferredRatioToLayout() {
			if (!shellStore.workspacePanelOpen || resizing) return;
			applyWidthFromPreferredRatio();
		}

		function onWindowResize() {
			applyPreferredRatioToLayout();
		}
		window.addEventListener('resize', onWindowResize);

		if (contentRowRef) {
			resizeObserver = new ResizeObserver(() => {
				applyPreferredRatioToLayout();
			});
			resizeObserver.observe(contentRowRef);
		}

		return () => {
			unsubscribeNarrowViewport();
			window.removeEventListener('keydown', onKeydown, true);
			unsubscribeNavigate?.();
			unsubscribeCloseWebPanel?.();
			unsubscribeCloseInbox?.();
			unsubscribeRequestCloseWindow?.();
			unsubscribeRequestReload?.();
			unsubscribeToggleWebPanel?.();
			unsubscribeOpenWebPanel?.();
			unsubscribeShortcutAction?.();
			unsubscribeReplayIntro?.();
			unsubscribeRunSetupWizard?.();
			unsubscribeFullScreen?.();
			document.removeEventListener('fullscreenchange', onDomFullScreenChange);
			window.removeEventListener('resize', onWindowResize);
			resizeObserver?.disconnect();
		};
	});

	function parseDuration(value: string) {
		const trimmed = value.trim();
		if (!trimmed) return FALLBACK_SIDEBAR_DURATION;
		if (trimmed.endsWith('ms'))
			return Number(trimmed.slice(0, -2)) || FALLBACK_SIDEBAR_DURATION;
		if (trimmed.endsWith('s')) return (Number(trimmed.slice(0, -1)) || 0) * 1000;
		return Number(trimmed) || FALLBACK_SIDEBAR_DURATION;
	}

	function sidebarTransitionDuration() {
		if (window.matchMedia('(prefers-reduced-motion: reduce)').matches) return 0;
		return parseDuration(
			getComputedStyle(document.documentElement).getPropertyValue('--duration-sidebar')
		);
	}

	$effect(() => {
		window.electronAPI?.setSidebarOpen?.({
			open: shellStore.sidebarOpen,
			duration: sidebarTransitionDuration()
		});
	});

	// Match web-panel ratio updates to the sidebar width animation so the
	// panel tracks the content row without a lagged CSS width transition.
	$effect(() => {
		void shellStore.sidebarOpen;
		if (typeof document === 'undefined') return;
		document.body.classList.add('sidebar-animating');
		const timeout = window.setTimeout(
			() => document.body.classList.remove('sidebar-animating'),
			sidebarTransitionDuration()
		);
		return () => {
			window.clearTimeout(timeout);
			document.body.classList.remove('sidebar-animating');
		};
	});

	function handleMainMouseDown() {
		shellStore.setFocusedPane('chat');
	}

	function titlebarSlideParams() {
		return {
			duration: sidebarTransitionDuration(),
			axis: 'y' as const
		};
	}

	const showShellTitlebar = $derived(!shellStore.sidebarOpen && !shellStore.fullscreen);

	// --- Web/file panel resize ---------------------------------------------
	/** User's preferred share of the content row; survives temporary clamps. */
	let preferredRatio = $state(0.5);
	let resizing = $state(false);
	let resizeStartX = 0;
	let resizeStartWidth = 0;

	function panelChrome() {
		return {
			sidebarOpen: shellStore.sidebarOpen,
			fullscreen: shellStore.fullscreen
		};
	}
	function contentRowWidth() {
		return contentRowRef?.clientWidth ?? window.innerWidth;
	}
	function currentPanelWidth() {
		const raw = getComputedStyle(document.documentElement)
			.getPropertyValue('--web-panel-width')
			.trim();
		const px = Number.parseFloat(raw);
		if (raw.endsWith('px') && Number.isFinite(px)) return px;
		// Fallback for vw/default: measure the rendered panel inner element.
		const inner = document.querySelector<HTMLElement>(
			'.web-panel-inner, .terminal-panel-inner'
		);
		if (inner) return inner.getBoundingClientRect().width;
		return Math.round(window.innerWidth * 0.5);
	}

	function applyWidthFromPreferredRatio() {
		const next = widthFromRatio(preferredRatio, contentRowWidth(), panelChrome());
		document.documentElement.style.setProperty('--web-panel-width', `${next}px`);
		return next;
	}

	function setPanelWidthPx(width: number) {
		const display = clampWebPanelWidth(width, contentRowWidth(), panelChrome());
		document.documentElement.style.setProperty('--web-panel-width', `${display}px`);
		return display;
	}

	// Keep preferredRatio in sync with persisted settings (not chrome changes).
	$effect(() => {
		const prefs = {
			webPanelRatio: settingsStore.settings.app.webPanelRatio,
			webPanelWidth: settingsStore.settings.app.webPanelWidth
		};
		preferredRatio = resolveWebPanelRatio(prefs, contentRowWidth());
		// Migrate legacy absolute-only prefs to an explicit ratio once.
		if (prefs.webPanelRatio <= 0 && prefs.webPanelWidth > 0) {
			const width = widthFromRatio(preferredRatio, contentRowWidth(), panelChrome());
			void settingsStore.saveWebPanelLayout(width, preferredRatio);
		}
	});

	// Re-apply the preferred ratio when chrome changes. Opening the sidebar
	// tightens the main-pane floor; closing it restores toward the ratio.
	$effect(() => {
		void shellStore.sidebarOpen;
		void shellStore.fullscreen;
		void shellStore.workspacePanelOpen;
		void preferredRatio;
		if (!shellStore.workspacePanelOpen) return;
		queueMicrotask(() => {
			if (!shellStore.workspacePanelOpen || resizing) return;
			applyWidthFromPreferredRatio();
		});
	});

	function onResizePointerDown(event: PointerEvent) {
		if (event.button !== 0) return;
		event.preventDefault();
		resizing = true;
		resizeStartX = event.clientX;
		resizeStartWidth = currentPanelWidth();
		(event.currentTarget as HTMLElement).setPointerCapture(event.pointerId);
		document.body.classList.add('panel-resizing');
	}

	function onResizePointerMove(event: PointerEvent) {
		if (!resizing) return;
		// Panel is on the right: dragging left (negative delta) grows it.
		const raw = resizeStartWidth - (event.clientX - resizeStartX);
		setPanelWidthPx(raw);
	}

	function endResize(event: PointerEvent) {
		if (!resizing) return;
		resizing = false;
		const target = event.currentTarget as HTMLElement;
		if (target.hasPointerCapture(event.pointerId)) {
			target.releasePointerCapture(event.pointerId);
		}
		document.body.classList.remove('panel-resizing');
		const width = currentPanelWidth();
		preferredRatio = widthToRatio(width, contentRowWidth());
		void settingsStore.saveWebPanelLayout(width, preferredRatio);
	}

	function onResizeKeydown(event: KeyboardEvent) {
		const step = event.shiftKey ? 64 : 16;
		let next: number | null = null;
		if (event.key === 'ArrowLeft') next = currentPanelWidth() + step;
		else if (event.key === 'ArrowRight') next = currentPanelWidth() - step;
		if (next === null) return;
		event.preventDefault();
		const width = setPanelWidthPx(next);
		preferredRatio = widthToRatio(width, contentRowWidth());
		void settingsStore.saveWebPanelLayout(width, preferredRatio);
	}
</script>

<div
	class="app-shell"
	class:sidebar-collapsed={!shellStore.sidebarOpen}
	class:is-fullscreen={shellStore.fullscreen}
>
	<Sidebar bind:this={sidebarRef} collapsed={!shellStore.sidebarOpen} />
	<div class="content-row" bind:this={contentRowRef}>
		<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
		<main
			class="main content-panel-surface max-[900px]:shadow-none"
			class:pane-focus-active={shellStore.focusedPane === 'chat' &&
				shellStore.workspacePanelOpen}
			onmousedown={handleMainMouseDown}
		>
			{#if showShellTitlebar}
				<header
					class="shell-titlebar"
					aria-label="Window title bar"
					transition:slide={titlebarSlideParams()}
				>
					<Tooltip label="Show sidebar" action="toggleSidebar">
						<button
							type="button"
							class="shell-titlebar-btn"
							aria-label="Show sidebar"
							onclick={() => shellStore.openSidebar()}
						>
							<PanelLeft size={16} stroke-width={1.8} />
						</button>
					</Tooltip>
					{#if titlebarSessionTitle}
						<button
							type="button"
							class="shell-titlebar-title"
							title={titlebarSessionTitleAttr}
							aria-label={`Rename session: ${titlebarSessionTitle}`}
							ondblclick={startRenameFromTitlebar}
						>
							{titlebarSessionTitle}
						</button>
					{/if}
				</header>
			{/if}
			{@render children()}
			<RuntimeOverlay />
		</main>
		{#if shellStore.workspacePanelOpen}
			<!-- svelte-ignore a11y_no_noninteractive_tabindex -->
			<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
			<div
				class="panel-resizer"
				class:resizing
				role="separator"
				aria-orientation="vertical"
				aria-label="Resize web panel"
				tabindex="0"
				onpointerdown={onResizePointerDown}
				onpointermove={onResizePointerMove}
				onpointerup={endResize}
				onpointercancel={endResize}
				onkeydown={onResizeKeydown}
			></div>
		{/if}
		<WebPanel bind:this={webPanelRef} />
		<TerminalPanel />
	</div>
	<SettingsModal />
	<InboxDrawer
		open={inboxStore.drawerOpen}
		messages={inboxStore.messages}
		busyId={inboxStore.busyId}
		error={inboxStore.error}
		onClose={() => inboxStore.closeDrawer()}
		onReply={(id, content) => inboxStore.reply(id, content)}
		onDismiss={(id) => inboxStore.dismiss(id)}
		onOpenJob={(jobId) => {
			inboxStore.closeDrawer();
			void goto(`/jobs?job=${encodeURIComponent(jobId)}`);
		}}
		onOpenSession={(sessionId) => {
			inboxStore.closeDrawer();
			void getSession(sessionId)
				.then((session) => navigateToSession(session))
				.catch(() => {
					/* session may already be purged */
				});
		}}
	/>
	<UpdateButton />
	<MemoryToast />
	<AppToast />
	<ConfirmActionModal
		open={closeConfirmOpen}
		title="Are you sure you want to close Cometline?"
		description="The window will hide to the menu bar. You can reopen it anytime."
		confirmLabel="Close"
		secondaryLabel="Always close"
		onSecondary={() => void alwaysCloseWithoutConfirm()}
		onCancel={() => (closeConfirmOpen = false)}
		onConfirm={hideMainWindow}
	/>
	<ConfirmActionModal
		open={reloadConfirmOpen}
		title="Are you sure you want to refresh?"
		description="Refreshing reloads the app and can interrupt the main panel and any open terminal sessions."
		confirmLabel="Refresh"
		onCancel={() => {
			reloadConfirmOpen = false;
			reloadConfirmArmed = false;
		}}
		onConfirm={confirmReload}
	/>
	<ConfirmActionModal
		open={Boolean(pendingRename)}
		title="Rename session"
		description="Choose a name for this chat."
		confirmLabel="Save"
		confirmTone="accent"
		showInput
		bind:inputValue={renameTitle}
		inputPlaceholder="Untitled"
		inputMaxLength={200}
		onCancel={cancelRename}
		onConfirm={() => void confirmRename()}
	/>
	{#if shellStore.introOpen}
		<IntroAnimation />
	{/if}
	{#if shellStore.setupOpen}
		<SetupWizard />
	{/if}
</div>

<style>
	.app-shell {
		--active-sidebar-width: var(--sidebar-width);
		display: flex;
		width: 100vw;
		height: 100vh;
		background: var(--shell-canvas-bg);
		box-sizing: border-box;
	}

	.app-shell.sidebar-collapsed {
		--active-sidebar-width: 0px;
	}

	.app-shell.is-fullscreen {
		--traffic-light-gutter: 0px;
	}

	/* Internal chrome of the main card. Traffic lights are hidden while the
	   sidebar is collapsed, so no gutter reserved for native window buttons. */
	.shell-titlebar {
		position: relative;
		flex-shrink: 0;
		height: var(--panel-header-height);
		z-index: 40;
		display: flex;
		align-items: center;
		box-sizing: border-box;
		padding: 0 10px 0 12px;
		border-bottom: 1px solid color-mix(in srgb, var(--border-soft) 80%, transparent);
		background: transparent;
		-webkit-app-region: drag;
	}

	.shell-titlebar-title {
		position: absolute;
		left: 50%;
		top: 50%;
		transform: translate(-50%, -50%);
		min-width: 0;
		max-width: min(36vw, 14rem);
		margin: 0;
		padding: 0;
		border: none;
		background: transparent;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		color: var(--text-muted);
		font: inherit;
		font-size: 12px;
		font-weight: 600;
		letter-spacing: 0.01em;
		line-height: 1;
		user-select: none;
		text-align: center;
		cursor: default;
		-webkit-app-region: no-drag;
	}

	@media (min-width: 900px) {
		.shell-titlebar-title {
			max-width: min(42vw, 22rem);
		}
	}

	@media (min-width: 1280px) {
		.shell-titlebar-title {
			max-width: min(48vw, 32rem);
		}
	}

	.shell-titlebar-btn {
		width: 26px;
		height: 26px;
		padding: 0;
		border: none;
		border-radius: 6px;
		background: transparent;
		color: var(--text-muted);
		display: grid;
		place-items: center;
		cursor: pointer;
		flex-shrink: 0;
		-webkit-app-region: no-drag;
	}

	.shell-titlebar-btn:hover {
		background: rgba(0, 0, 0, 0.04);
		color: var(--text-main);
	}

	.shell-titlebar-btn:active {
		background: rgba(0, 0, 0, 0.07);
	}

	.content-row {
		flex: 1;
		min-width: 0;
		display: flex;
		position: relative;
	}

	.main {
		flex: 1 1 0;
		min-width: 0;
		display: flex;
		flex-direction: column;
		position: relative;
		z-index: 1;
		margin: var(--content-panel-inset);
		margin-left: calc(-1 * var(--content-panel-overlap));
		overflow: hidden;
		/* Chat metrics respond to this pane's width when the web panel is open. */
		container-type: inline-size;
		container-name: main-pane;
		transition:
			margin-left var(--duration-sidebar) var(--ease-smooth),
			border-color var(--duration-sidebar) var(--ease-smooth),
			box-shadow var(--duration-sidebar) var(--ease-smooth);
	}

	.app-shell.sidebar-collapsed .main {
		/* Keep the floating rounded content card under the thin titlebar. */
		margin: var(--content-panel-inset);
	}

	/* Keep a slim main strip for the collapsed titlebar when the web panel is wide. */
	.app-shell.sidebar-collapsed:not(.is-fullscreen) .main {
		min-width: 72px;
	}

	.panel-resizer {
		flex: 0 0 auto;
		width: 12px;
		margin: 0 -3px 0 -9px;
		z-index: 2;
		cursor: col-resize;
		align-self: stretch;
		background: transparent;
		position: relative;
	}

	.panel-resizer::before {
		content: '';
		position: absolute;
		top: 50%;
		left: 50%;
		transform: translate(-50%, -50%);
		width: 4px;
		height: 36px;
		border-radius: 999px;
		background: var(--border-subtle, rgba(148, 163, 184, 0.4));
		opacity: 0;
		transition: opacity var(--duration-fast) var(--ease-smooth);
	}

	.panel-resizer:hover::before,
	.panel-resizer:focus-visible::before,
	.panel-resizer.resizing::before {
		opacity: 1;
	}

	.panel-resizer:focus-visible {
		outline: none;
	}

	@media (prefers-reduced-motion: reduce) {
		.main {
			transition: none;
		}
	}

	@media (max-width: 900px) {
		.app-shell {
			--active-sidebar-width: 0px;
			background: var(--app-bg);
		}

		.content-row {
			display: flex;
		}

		.main {
			margin: 0;
			border: none;
			border-radius: 0;
			background: transparent;
			box-shadow: none;
		}

		.panel-resizer {
			display: none;
		}

		.app-shell:not(.sidebar-collapsed) :global(.sidebar:not(.collapsed)) {
			position: fixed;
			inset: 0;
			width: 100vw;
			height: 100vh;
			z-index: 50;
			flex-shrink: 0;
			border-right: none;
		}
	}
</style>
