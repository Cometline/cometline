<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { PanelLeftClose, PanelLeftOpen, PanelRightOpen } from '@lucide/svelte';
	import Sidebar from './Sidebar.svelte';
	import RuntimeOverlay from './RuntimeOverlay.svelte';
	import SettingsModal from './SettingsModal.svelte';
	import SetupWizard from './onboarding/SetupWizard.svelte';
	import UpdateButton from './UpdateButton.svelte';
	import MemoryToast from './MemoryToast.svelte';
	import AppToast from './AppToast.svelte';
	import ConfirmActionModal from './ConfirmActionModal.svelte';
	import FileSearchModal from './FileSearchModal.svelte';
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
	import {
		navigateSessionHistory,
		navigateToRecentSession
	} from '$lib/actions/navigate-session-history';
	import { navigateToSession } from '$lib/actions/navigate-to-session';
	import { sessionDisplayTitle } from '$lib/sessions/session-title';
	import { narrowViewportQuery, subscribeNarrowViewport } from '$lib/layout/narrow-viewport';
	import {
		clampWorkspacePanelWidth,
		resolveWorkspacePanelRatio,
		widthFromRatio,
		widthToRatio
	} from '$lib/layout/workspace-panel-width';
	import { shouldUseWorkspacePanelHistory } from '$lib/navigation/focus-nav';
	import {
		matchesShortcut,
		isReloadShortcut,
		type ShortcutAction
	} from '$lib/keyboard-shortcuts';
	import type { Session } from '$lib/types';

	const FALLBACK_SIDEBAR_DURATION = 360;

	let { children }: { children: import('svelte').Snippet } = $props();

	let sidebarRef = $state<{ focusSearch: () => void } | null>(null);
	let workspacePanelRef = $state<{
		navigateBack: () => void;
		navigateForward: () => void;
	} | null>(null);
	type WorkspacePanelModuleComponent = typeof import('./WorkspacePanel.svelte').default;
	let WorkspacePanelComponent = $state<WorkspacePanelModuleComponent | null>(null);
	let workspacePanelLoadPromise: Promise<WorkspacePanelModuleComponent | null> | null = null;
	let workspacePanelLoadFailed = $state(false);
	type IntroAnimationComponent = typeof import('./IntroAnimation.svelte').default;
	let IntroAnimation = $state<IntroAnimationComponent | null>(null);
	let introAnimationLoadPromise: Promise<IntroAnimationComponent | null> | null = null;
	type InboxDrawerComponent = typeof import('./inbox/InboxDrawer.svelte').default;
	let InboxDrawer = $state<InboxDrawerComponent | null>(null);
	let inboxDrawerLoadPromise: Promise<InboxDrawerComponent | null> | null = null;
	let inboxDrawerLoadFailed = $state(false);
	let contentRowRef = $state<HTMLDivElement | null>(null);
	let closeConfirmOpen = $state(false);
	let fileSearchOpen = $state(false);
	let reloadConfirmOpen = $state(false);
	/** True only after the confirm dialog has settled, so a duplicated Cmd+R delivery cannot open+confirm in one press. */
	let reloadConfirmArmed = $state(false);
	let pendingRename = $state<Session | null>(null);
	let renameTitle = $state('');

	function loadWorkspacePanel() {
		if (WorkspacePanelComponent) return Promise.resolve(WorkspacePanelComponent);
		if (!workspacePanelLoadPromise) {
			workspacePanelLoadFailed = false;
			workspacePanelLoadPromise = import('./WorkspacePanel.svelte')
				.then((module) => {
					WorkspacePanelComponent = module.default;
					return module.default;
				})
				.catch((error) => {
					workspacePanelLoadPromise = null;
					workspacePanelLoadFailed = true;
					console.error('Workspace panel failed to load', error);
					return null;
				});
		}
		return workspacePanelLoadPromise;
	}

	function loadIntroAnimation() {
		if (IntroAnimation) return Promise.resolve(IntroAnimation);
		if (!introAnimationLoadPromise) {
			introAnimationLoadPromise = import('./IntroAnimation.svelte')
				.then((module) => {
					IntroAnimation = module.default;
					return module.default;
				})
				.catch((error) => {
					introAnimationLoadPromise = null;
					console.error('Intro animation failed to load', error);
					shellStore.closeIntro();
					return null;
				});
		}
		return introAnimationLoadPromise;
	}

	function loadInboxDrawer() {
		if (InboxDrawer) return Promise.resolve(InboxDrawer);
		if (!inboxDrawerLoadPromise) {
			inboxDrawerLoadFailed = false;
			inboxDrawerLoadPromise = import('./inbox/InboxDrawer.svelte')
				.then((module) => {
					InboxDrawer = module.default;
					return module.default;
				})
				.catch((error) => {
					inboxDrawerLoadPromise = null;
					inboxDrawerLoadFailed = true;
					console.error('Inbox drawer failed to load', error);
					return null;
				});
		}
		return inboxDrawerLoadPromise;
	}

	let activeSessionId = $derived(sessionStore.current?.id ?? null);
	let titlebarSessionTitle = $derived.by(() => {
		const session = sessionStore.current;
		if (!session) return '';
		return sessionDisplayTitle(session.title);
	});

	function canFindInSession() {
		return Boolean(
			activeSessionId &&
			page.url.pathname.startsWith('/session/') &&
			!shellStore.settingsOpen &&
			!inboxStore.drawerOpen
		);
	}
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
	// via IPC forwarded from the webview guest (workspace panel focused).
	function runShortcutAction(action: ShortcutAction) {
		switch (action) {
			case 'toggleSidebar':
				shellStore.toggleSidebar();
				return;
			case 'toggleWorkspacePanel':
				shellStore.toggleWorkspacePanel();
				return;
			case 'openWebSearch':
				shellStore.openWebSearchPanel();
				return;
			case 'openGitPanel':
				shellStore.openGitChangesPanel();
				return;
			case 'openWikiPanel':
				shellStore.setWorkspacePanelBrowseSource('wiki');
				return;
			case 'openWorkspacePanel':
				shellStore.setWorkspacePanelBrowseSource('workspace');
				return;
			case 'openFileSearch':
				if (shellStore.settingsOpen) return;
				fileSearchOpen = true;
				return;
			case 'openTerminal':
				shellStore.requestTerminalFocus();
				return;
			case 'navigateBack':
				if (
					shouldUseWorkspacePanelHistory(
						shellStore.workspacePanelOpen,
						!!workspacePanelRef
					)
				) {
					workspacePanelRef?.navigateBack();
				} else {
					navigateSessionHistory('back');
				}
				return;
			case 'navigateForward':
				if (
					shouldUseWorkspacePanelHistory(
						shellStore.workspacePanelOpen,
						!!workspacePanelRef
					)
				) {
					workspacePanelRef?.navigateForward();
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
			case 'findInSession':
				if (!canFindInSession()) return;
				shellStore.requestSessionFind();
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
				void goto('/skills');
				return;
			case 'openGallery':
				if (shellStore.settingsOpen) shellStore.closeSettings();
				inboxStore.closeDrawer();
				void goto('/gallery');
				return;
			case 'openUsage':
				if (shellStore.settingsOpen) shellStore.closeSettings();
				inboxStore.closeDrawer();
				void goto('/usage');
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
		let panelPreloadHandle: number | ReturnType<typeof setTimeout> | null = null;
		let panelPreloadUsesIdleCallback = false;
		void terminalStore.initialize();

		if ('requestIdleCallback' in window) {
			panelPreloadUsesIdleCallback = true;
			panelPreloadHandle = window.requestIdleCallback(
				() => {
					void loadWorkspacePanel();
					void loadInboxDrawer();
				},
				{ timeout: 1500 }
			);
		} else {
			panelPreloadHandle = setTimeout(() => {
				void loadWorkspacePanel();
				void loadInboxDrawer();
			}, 300);
		}

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
			if (matchesShortcut(event, shortcuts.toggleWorkspacePanel)) {
				event.preventDefault();
				runShortcutAction('toggleWorkspacePanel');
				return;
			}
			if (matchesShortcut(event, shortcuts.openWebSearch)) {
				event.preventDefault();
				runShortcutAction('openWebSearch');
				return;
			}
			if (matchesShortcut(event, shortcuts.openGitPanel)) {
				event.preventDefault();
				runShortcutAction('openGitPanel');
				return;
			}
			if (matchesShortcut(event, shortcuts.openWikiPanel)) {
				event.preventDefault();
				runShortcutAction('openWikiPanel');
				return;
			}
			if (matchesShortcut(event, shortcuts.openWorkspacePanel)) {
				event.preventDefault();
				runShortcutAction('openWorkspacePanel');
				return;
			}
			if (matchesShortcut(event, shortcuts.openFileSearch)) {
				event.preventDefault();
				runShortcutAction('openFileSearch');
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
			if (matchesShortcut(event, shortcuts.cycleReasoningEffort)) {
				// Owned by the composer (cycles reasoning effort when the active
				// model supports it); must not fall through to newChat.
				return;
			}
			if (matchesShortcut(event, shortcuts.newChat)) {
				event.preventDefault();
				runShortcutAction('newChat');
				return;
			}
			if (matchesShortcut(event, shortcuts.findInSession)) {
				if (!canFindInSession()) return;
				event.preventDefault();
				runShortcutAction('findInSession');
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
			if (matchesShortcut(event, shortcuts.openGallery)) {
				event.preventDefault();
				runShortcutAction('openGallery');
				return;
			}
			if (matchesShortcut(event, shortcuts.openUsage)) {
				event.preventDefault();
				runShortcutAction('openUsage');
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

		const unsubscribeCloseWorkspacePanel = window.electronAPI?.onCloseWorkspacePanel?.(() => {
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

		const unsubscribeToggleWorkspacePanel = window.electronAPI?.onToggleWorkspacePanel?.(() => {
			if (shellStore.settingsOpen) return;
			shellStore.toggleWorkspacePanel();
		});

		const unsubscribeOpenWebSearch = window.electronAPI?.onOpenWebSearch?.(() => {
			if (shellStore.settingsOpen) return;
			shellStore.openWebSearchPanel();
		});

		// Shortcuts forwarded from the webview guest (workspace panel focused). Run the
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
			shellStore.setFullscreen(isFullScreen);
		}
		void window.electronAPI?.getFullScreen?.().then(updateFullScreen);
		const unsubscribeFullScreen = window.electronAPI?.onFullScreenChange?.(updateFullScreen);

		function onDomFullScreenChange() {
			updateFullScreen(Boolean(document.fullscreenElement));
		}
		document.addEventListener('fullscreenchange', onDomFullScreenChange);

		// Keep the preferred ratio applied as the content row changes (window
		// resize). Sidebar open/close writes the end width once and CSS-transitions
		// it — continuous RO rewrites during the 360ms anim cause lag + thrash.
		function applyPreferredRatioToLayout() {
			if (!shellStore.workspacePanelOpen || resizing || sidebarAnimating) return;
			if (ratioFrame) return;
			ratioFrame = requestAnimationFrame(() => {
				ratioFrame = 0;
				if (!shellStore.workspacePanelOpen || resizing || sidebarAnimating) return;
				applyWidthFromPreferredRatio();
			});
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
			unsubscribeCloseWorkspacePanel?.();
			unsubscribeCloseInbox?.();
			unsubscribeRequestCloseWindow?.();
			unsubscribeRequestReload?.();
			unsubscribeToggleWorkspacePanel?.();
			unsubscribeOpenWebSearch?.();
			unsubscribeShortcutAction?.();
			unsubscribeReplayIntro?.();
			unsubscribeRunSetupWizard?.();
			unsubscribeFullScreen?.();
			document.removeEventListener('fullscreenchange', onDomFullScreenChange);
			window.removeEventListener('resize', onWindowResize);
			if (panelPreloadHandle !== null) {
				if (panelPreloadUsesIdleCallback)
					window.cancelIdleCallback(panelPreloadHandle as number);
				else clearTimeout(panelPreloadHandle);
			}
			if (ratioFrame) cancelAnimationFrame(ratioFrame);
			ratioFrame = 0;
			resizeObserver?.disconnect();
		};
	});

	$effect(() => {
		if (!shellStore.workspacePanelOpen || WorkspacePanelComponent) return;
		void loadWorkspacePanel();
	});

	$effect(() => {
		if (!shellStore.introOpen || IntroAnimation) return;
		void loadIntroAnimation();
	});

	$effect(() => {
		if (!inboxStore.drawerOpen || InboxDrawer) return;
		void loadInboxDrawer();
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

	// Sidebar open/close: one target width write + CSS transition (same duration).
	// Avoids per-frame ResizeObserver ratio thrash while keeping the panel aligned.
	$effect(() => {
		void shellStore.sidebarOpen;
		if (typeof document === 'undefined') return;
		sidebarAnimating = true;
		document.body.classList.add('sidebar-animating');
		const duration = sidebarTransitionDuration();
		if (shellStore.workspacePanelOpen && !resizing) {
			applyWidthForSidebarEndState(shellStore.sidebarOpen);
		}
		const timeout = window.setTimeout(() => {
			sidebarAnimating = false;
			document.body.classList.remove('sidebar-animating');
			// Snap to the measured row once layout has settled.
			if (shellStore.workspacePanelOpen && !resizing) {
				applyWidthFromPreferredRatio();
			}
		}, duration);
		return () => {
			window.clearTimeout(timeout);
			sidebarAnimating = false;
			document.body.classList.remove('sidebar-animating');
		};
	});

	function handleMainMouseDown() {
		shellStore.setFocusedPane('chat');
	}

	const isUtilityPage = $derived(
		page.url.pathname === '/jobs' ||
			page.url.pathname === '/skills' ||
			page.url.pathname === '/skill-drafts' ||
			page.url.pathname === '/gallery' ||
			page.url.pathname === '/usage'
	);
	const showShellTitlebar = $derived(!shellStore.fullscreen);
	const titlebarLabel = $derived.by(() => {
		if (page.url.pathname === '/jobs') return 'Jobs';
		if (page.url.pathname === '/skills' || page.url.pathname === '/skill-drafts') return 'Skills';
		if (page.url.pathname === '/gallery') return 'Gallery';
		if (page.url.pathname === '/usage') return 'Usage';
		return titlebarSessionTitle;
	});
	const titlebarRenamable = $derived(Boolean(titlebarSessionTitle && !isUtilityPage));

	// --- Web/file panel resize ---------------------------------------------
	/** User's preferred share of the content row; survives temporary clamps. */
	let preferredRatio = $state(0.5);
	let resizing = $state(false);
	/** True while the left sidebar width transition is in flight. */
	let sidebarAnimating = $state(false);
	/** Coalesce ResizeObserver/window-resize ratio writes to one frame. */
	let ratioFrame = 0;
	let resizeStartX = 0;
	let resizeStartWidth = 0;

	function panelChrome() {
		return {
			// Utility pages need the normal usable main width when the panel is open,
			// even if the session sidebar is collapsed.
			sidebarOpen: shellStore.sidebarOpen || isUtilityPage,
			fullscreen: shellStore.fullscreen
		};
	}
	function contentRowWidth() {
		return contentRowRef?.clientWidth ?? window.innerWidth;
	}
	function sidebarWidthPx() {
		const raw = getComputedStyle(document.documentElement)
			.getPropertyValue('--sidebar-width')
			.trim();
		const px = Number.parseFloat(raw);
		return Number.isFinite(px) ? px : 250;
	}
	/** Content-row width after the sidebar width transition settles. */
	function contentRowWidthAfterSidebarToggle(open: boolean) {
		const shellW =
			contentRowRef?.parentElement?.clientWidth ??
			document.querySelector<HTMLElement>('.app-shell')?.clientWidth ??
			window.innerWidth;
		// Narrow layout uses an overlay sidebar that does not consume row width.
		const narrow = window.matchMedia('(max-width: 900px)').matches;
		const side = !narrow && open ? sidebarWidthPx() : 0;
		return Math.max(0, shellW - side);
	}
	function currentPanelWidth() {
		const raw = getComputedStyle(document.documentElement)
			.getPropertyValue('--workspace-panel-width')
			.trim();
		const px = Number.parseFloat(raw);
		if (raw.endsWith('px') && Number.isFinite(px)) return px;
		// Fallback for vw/default: measure the rendered panel inner element.
		const inner = document.querySelector<HTMLElement>('.workspace-panel-inner');
		if (inner) return inner.getBoundingClientRect().width;
		return Math.round(window.innerWidth * 0.5);
	}

	function applyWidthFromPreferredRatio() {
		const next = widthFromRatio(preferredRatio, contentRowWidth(), panelChrome());
		document.documentElement.style.setProperty('--workspace-panel-width', `${next}px`);
		return next;
	}

	function applyWidthForSidebarEndState(open: boolean) {
		const endRow = contentRowWidthAfterSidebarToggle(open);
		const chrome = {
			sidebarOpen: open || isUtilityPage,
			fullscreen: shellStore.fullscreen
		};
		const next = widthFromRatio(preferredRatio, endRow, chrome);
		document.documentElement.style.setProperty('--workspace-panel-width', `${next}px`);
		return next;
	}

	function setPanelWidthPx(width: number) {
		const display = clampWorkspacePanelWidth(width, contentRowWidth(), panelChrome());
		document.documentElement.style.setProperty('--workspace-panel-width', `${display}px`);
		return display;
	}

	// Keep preferredRatio in sync with persisted settings (not chrome changes).
	$effect(() => {
		const prefs = {
			workspacePanelRatio: settingsStore.settings.app.workspacePanelRatio,
			workspacePanelWidth: settingsStore.settings.app.workspacePanelWidth
		};
		preferredRatio = resolveWorkspacePanelRatio(prefs, contentRowWidth());
		// Migrate legacy absolute-only prefs to an explicit ratio once.
		if (prefs.workspacePanelRatio <= 0 && prefs.workspacePanelWidth > 0) {
			const width = widthFromRatio(preferredRatio, contentRowWidth(), panelChrome());
			void settingsStore.saveWorkspacePanelLayout(width, preferredRatio);
		}
	});

	// Re-apply the preferred ratio when non-sidebar chrome changes. Sidebar open/
	// close is owned by the sidebar-animating effect (end-state + CSS transition).
	$effect(() => {
		void shellStore.fullscreen;
		void shellStore.workspacePanelOpen;
		void preferredRatio;
		if (!shellStore.workspacePanelOpen) return;
		queueMicrotask(() => {
			if (!shellStore.workspacePanelOpen || resizing || sidebarAnimating) return;
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
		void settingsStore.saveWorkspacePanelLayout(width, preferredRatio);
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
		void settingsStore.saveWorkspacePanelLayout(width, preferredRatio);
	}
</script>

<div
	class="app-shell"
	class:sidebar-collapsed={!shellStore.sidebarOpen}
	class:is-fullscreen={shellStore.fullscreen}
	class:utility-page={isUtilityPage}
>
	<Sidebar bind:this={sidebarRef} collapsed={!shellStore.sidebarOpen} />
	<div class="content-row" bind:this={contentRowRef}>
		<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
		<main
			class="main content-panel-surface max-[900px]:shadow-none"
			class:utility-page={isUtilityPage}
			class:pane-focus-active={shellStore.focusedPane === 'chat' &&
				shellStore.workspacePanelOpen}
			onmousedown={handleMainMouseDown}
		>
			{#if showShellTitlebar}
				<header class="shell-titlebar" aria-label="Window title bar">
					<div class="shell-titlebar-start">
						<Tooltip
							label={shellStore.sidebarOpen ? 'Hide sidebar' : 'Show sidebar'}
							action="toggleSidebar"
						>
							<button
								type="button"
								class="shell-titlebar-btn"
								aria-label={shellStore.sidebarOpen ? 'Hide sidebar' : 'Show sidebar'}
								aria-pressed={shellStore.sidebarOpen}
								onclick={() => shellStore.toggleSidebar()}
							>
								{#if shellStore.sidebarOpen}
									<PanelLeftClose size={16} stroke-width={1.8} />
								{:else}
									<PanelLeftOpen size={16} stroke-width={1.8} />
								{/if}
							</button>
						</Tooltip>
					</div>
					{#if titlebarLabel}
						{#if titlebarRenamable}
							<button
								type="button"
								class="shell-titlebar-title"
								title={titlebarSessionTitleAttr}
								aria-label={`Rename session: ${titlebarLabel}`}
								ondblclick={startRenameFromTitlebar}
							>
								{titlebarLabel}
							</button>
						{:else}
							<span class="shell-titlebar-title">{titlebarLabel}</span>
						{/if}
					{/if}
					{#if !shellStore.workspacePanelOpen && !isUtilityPage}
						<div class="shell-titlebar-end">
							<Tooltip label="Show workspace panel" action="toggleWorkspacePanel">
								<button
									type="button"
									class="shell-titlebar-btn"
									aria-label="Show workspace panel"
									disabled={!activeSessionId}
									onclick={() => shellStore.toggleWorkspacePanel()}
								>
									<PanelRightOpen size={16} stroke-width={1.8} />
								</button>
							</Tooltip>
						</div>
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
				aria-label="Resize workspace panel"
				tabindex="0"
				onpointerdown={onResizePointerDown}
				onpointermove={onResizePointerMove}
				onpointerup={endResize}
				onpointercancel={endResize}
				onkeydown={onResizeKeydown}
			></div>
		{/if}
		{#if WorkspacePanelComponent}
			<WorkspacePanelComponent bind:this={workspacePanelRef} />
		{:else if shellStore.workspacePanelOpen}
			<aside
				class="workspace-panel-loading"
				aria-label="Loading workspace panel"
				aria-busy="true"
			>
				{#if workspacePanelLoadFailed}
					<button type="button" onclick={() => void loadWorkspacePanel()}
						>Retry workspace panel</button
					>
				{:else}
					<span>Loading workspace…</span>
				{/if}
			</aside>
		{/if}
	</div>
	<SettingsModal />
	{#if InboxDrawer}
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
	{:else if inboxStore.drawerOpen}
		<div class="inbox-loading-layer" role="dialog" aria-modal="true" aria-label="Inbox">
			<button
				type="button"
				class="inbox-loading-scrim"
				aria-label="Close inbox"
				onclick={() => inboxStore.closeDrawer()}
			></button>
			<div class="inbox-loading-card" aria-busy={!inboxDrawerLoadFailed}>
				{#if inboxDrawerLoadFailed}
					<p>Inbox failed to load.</p>
					<div class="inbox-loading-actions">
						<button type="button" onclick={() => inboxStore.closeDrawer()}>Close</button
						>
						<button type="button" class="primary" onclick={() => void loadInboxDrawer()}
							>Retry</button
						>
					</div>
				{:else}
					<p>Loading inbox…</p>
				{/if}
			</div>
		</div>
	{/if}
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
		inputPlaceholder="New Chat"
		inputMaxLength={200}
		onCancel={cancelRename}
		onConfirm={() => void confirmRename()}
	/>
	<FileSearchModal open={fileSearchOpen} onClose={() => (fileSearchOpen = false)} />
	{#if shellStore.introOpen}
		{#if IntroAnimation}
			<IntroAnimation />
		{:else}
			<div class="intro-loading" aria-label="Loading introduction"></div>
		{/if}
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

	.workspace-panel-loading {
		display: grid;
		flex: 0 1 auto;
		width: var(--workspace-panel-slot-width);
		max-width: 100%;
		min-width: 0;
		height: 100%;
		place-items: center;
		box-sizing: border-box;
		border-left: 1px solid var(--border-soft);
		background: var(--panel-bg);
		color: var(--text-muted);
		font-size: 13px;
	}

	.workspace-panel-loading button {
		border: 0;
		background: transparent;
		color: var(--accent);
		font: inherit;
		cursor: pointer;
	}

	.intro-loading {
		position: fixed;
		inset: 0;
		z-index: 90;
		background: var(--intro-bg, #fafafa);
	}

	.inbox-loading-layer {
		position: fixed;
		inset: 0;
		z-index: 75;
		display: grid;
		place-items: center;
		padding: 24px;
	}

	.inbox-loading-scrim {
		position: fixed;
		inset: 0;
		border: 0;
		background: rgba(17, 24, 39, 0.28);
		backdrop-filter: blur(10px);
	}

	.inbox-loading-card {
		position: relative;
		display: grid;
		min-width: 260px;
		gap: 16px;
		place-items: center;
		padding: 28px;
		border: 1px solid var(--border-soft);
		border-radius: 18px;
		background: var(--panel-bg);
		box-shadow: 0 22px 70px rgba(15, 23, 42, 0.18);
		color: var(--text-muted);
	}

	.inbox-loading-card p {
		margin: 0;
	}

	.inbox-loading-actions {
		display: flex;
		gap: 8px;
	}

	.inbox-loading-actions button {
		padding: 7px 12px;
		border: 1px solid var(--border-soft);
		border-radius: 8px;
		background: var(--panel-bg);
		color: var(--text-main);
		font: inherit;
		cursor: pointer;
	}

	.inbox-loading-actions button.primary {
		border-color: var(--accent);
		background: var(--accent);
		color: #ffffff;
	}

	.app-shell.sidebar-collapsed {
		--active-sidebar-width: 0px;
	}

	.app-shell.is-fullscreen {
		--traffic-light-gutter: 0px;
	}

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

	.shell-titlebar-start,
	.shell-titlebar-end {
		position: relative;
		z-index: 1;
		display: flex;
		align-items: center;
		-webkit-app-region: no-drag;
	}

	.shell-titlebar-end {
		margin-left: auto;
	}

	.shell-titlebar-title {
		position: absolute;
		left: 50%;
		top: 50%;
		transform: translate(-50%, -50%);
		min-width: 0;
		max-width: min(calc(100% - 88px), 14rem);
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
			max-width: min(calc(100% - 88px), 22rem);
		}
	}

	@media (min-width: 1280px) {
		.shell-titlebar-title {
			max-width: min(calc(100% - 88px), 32rem);
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

	.shell-titlebar-btn:hover:not(:disabled) {
		background: rgba(0, 0, 0, 0.04);
		color: var(--text-main);
	}

	.shell-titlebar-btn:active:not(:disabled) {
		background: rgba(0, 0, 0, 0.07);
	}

	.shell-titlebar-btn:disabled {
		opacity: 0.4;
		cursor: default;
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
		/* Chat metrics respond to this pane's width when the workspace panel is open. */
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

	/* Keep a slim main strip for the collapsed titlebar when the workspace panel is wide. */
	.app-shell.sidebar-collapsed:not(.is-fullscreen) .main {
		min-width: 72px;
	}

	/* Utility pages have their own page header and do not render the session
	   titlebar, while still keeping a usable main pane beside the workspace panel. */
	.app-shell.utility-page:not(.is-fullscreen) .main {
		min-width: 400px;
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
		/* background: var(--border-subtle, rgba(148, 163, 184, 0.4)); */
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

		.app-shell.utility-page .main {
			min-width: 0;
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
