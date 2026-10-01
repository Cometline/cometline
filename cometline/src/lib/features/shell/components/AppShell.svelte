<script lang="ts">
	import { onMount, untrack } from 'svelte';
	import { page } from '$app/state';
	import Sidebar from '$lib/features/sidebar/components/Sidebar.svelte';
	import RuntimeOverlay from './RuntimeOverlay.svelte';
	import SettingsModal from '$lib/features/settings/components/SettingsModal.svelte';
	import SetupWizard from '$lib/features/onboarding/components/SetupWizard.svelte';
	import UpdateButton from './UpdateButton.svelte';
	import MemoryToast from './MemoryToast.svelte';
	import AppToast from './AppToast.svelte';
	import ConfirmActionModal from '$lib/components/ConfirmActionModal.svelte';
	import FileSearchModal from '$lib/features/workspace/components/FileSearchModal.svelte';
	import { shellStore } from '$lib/stores/shell.svelte';
	import { sessionStore } from '$lib/stores/session.svelte';
	import { inboxStore } from '$lib/stores/inbox.svelte';
	import { terminalStore } from '$lib/stores/terminal.svelte';
	import { sessionDisplayTitle } from '$lib/sessions/session-title';
	import { narrowViewportQuery, subscribeNarrowViewport } from '$lib/layout/narrow-viewport';
	import { shouldClaimChatPaneFromMainPointer } from '$lib/features/workspace/workspace-pane-focus';
	import { createAppShellLayout } from '$lib/features/shell/app-shell-layout.svelte';
	import { createAppShellLazyPanels } from '$lib/features/shell/app-shell-lazy.svelte';
	import { createAppShellShortcuts } from '$lib/features/shell/app-shell-shortcuts.svelte';
	import ShellInboxHost from './ShellInboxHost.svelte';
	import ShellPanelResizer from './ShellPanelResizer.svelte';
	import ShellTitlebar from './ShellTitlebar.svelte';
	import WorkspacePanelLoading from './WorkspacePanelLoading.svelte';

	let { children }: { children: import('svelte').Snippet } = $props();

	let sidebarRef = $state<{ focusSearch: () => void } | null>(null);
	let workspacePanelRef = $state<{
		navigateBack: () => void;
		navigateForward: () => void;
	} | null>(null);
	let contentRowRef = $state<HTMLDivElement | null>(null);
	const panels = createAppShellLazyPanels();

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
		if (page.url.pathname === '/skills' || page.url.pathname === '/skill-drafts')
			return 'Skills';
		if (page.url.pathname === '/gallery') return 'Gallery';
		if (page.url.pathname === '/usage') return 'Usage';
		return titlebarSessionTitle;
	});
	const titlebarRenamable = $derived(Boolean(titlebarSessionTitle && !isUtilityPage));

	const shortcuts = createAppShellShortcuts({
		getSidebarRef: () => sidebarRef,
		getWorkspacePanelRef: () => workspacePanelRef,
		canFindInSession
	});
	const layout = createAppShellLayout({
		getContentRow: () => contentRowRef,
		getIsUtilityPage: () => isUtilityPage
	});

	$effect(() => {
		window.electronAPI?.setSessionNavigationSuspended?.(shellStore.settingsOpen);
	});

	$effect(() => {
		void activeSessionId;
		untrack(() => shellStore.onActiveSessionChange());
	});

	onMount(() => {
		let panelPreloadHandle: number | ReturnType<typeof setTimeout> | null = null;
		let panelPreloadUsesIdleCallback = false;
		void terminalStore.initialize();

		if ('requestIdleCallback' in window) {
			panelPreloadUsesIdleCallback = true;
			panelPreloadHandle = window.requestIdleCallback(
				() => {
					void panels.loadWorkspacePanel();
					void panels.loadInboxDrawer();
				},
				{ timeout: 1500 }
			);
		} else {
			panelPreloadHandle = setTimeout(() => {
				void panels.loadWorkspacePanel();
				void panels.loadInboxDrawer();
			}, 300);
		}

		if (narrowViewportQuery().matches) {
			shellStore.closeSidebar();
		}
		const unsubscribeNarrowViewport = subscribeNarrowViewport((narrow) => {
			if (narrow) shellStore.closeSidebar();
		});
		const stopShortcuts = shortcuts.install();
		const stopLayout = layout.install();

		return () => {
			unsubscribeNarrowViewport();
			stopShortcuts();
			stopLayout();
			if (panelPreloadHandle !== null) {
				if (panelPreloadUsesIdleCallback)
					window.cancelIdleCallback(panelPreloadHandle as number);
				else clearTimeout(panelPreloadHandle);
			}
		};
	});

	$effect(() => {
		if (!shellStore.workspacePanelOpen || panels.WorkspacePanel) return;
		void panels.loadWorkspacePanel();
	});

	$effect(() => {
		if (!shellStore.introOpen || panels.Intro) return;
		void panels.loadIntroAnimation();
	});

	$effect(() => {
		if (!inboxStore.drawerOpen || panels.Inbox) return;
		void panels.loadInboxDrawer();
	});

	function handleMainMouseDown(event: MouseEvent) {
		if (!shouldClaimChatPaneFromMainPointer(event.target)) return;
		shellStore.setFocusedPane('chat');
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
				<ShellTitlebar
					label={titlebarLabel}
					renamable={titlebarRenamable}
					titleAttr={titlebarSessionTitleAttr}
					utilityPage={isUtilityPage}
					{activeSessionId}
					onRename={shortcuts.startRenameFromTitlebar}
				/>
			{/if}
			{@render children()}
			<RuntimeOverlay />
		</main>
		{#if shellStore.workspacePanelOpen}
			<ShellPanelResizer
				resizing={layout.resizing}
				onPointerDown={layout.onResizePointerDown}
				onPointerMove={layout.onResizePointerMove}
				onPointerUp={layout.endResize}
				onKeydown={layout.onResizeKeydown}
			/>
		{/if}
		{#if panels.WorkspacePanel}
			<panels.WorkspacePanel bind:this={workspacePanelRef} />
		{:else if shellStore.workspacePanelOpen}
			<WorkspacePanelLoading
				failed={panels.workspacePanelLoadFailed}
				onRetry={() => void panels.loadWorkspacePanel()}
			/>
		{/if}
	</div>
	<SettingsModal />
	<ShellInboxHost
		drawer={panels.Inbox}
		loadFailed={panels.inboxLoadFailed}
		onRetry={() => void panels.loadInboxDrawer()}
	/>
	<UpdateButton />
	<MemoryToast />
	<AppToast />
	<ConfirmActionModal
		open={shortcuts.closeConfirmOpen}
		title="Are you sure you want to close Cometline?"
		description="The window will hide to the menu bar. You can reopen it anytime."
		confirmLabel="Close"
		secondaryLabel="Always close"
		onSecondary={() => void shortcuts.alwaysCloseWithoutConfirm()}
		onCancel={() => (shortcuts.closeConfirmOpen = false)}
		onConfirm={shortcuts.hideMainWindow}
	/>
	<ConfirmActionModal
		open={shortcuts.reloadConfirmOpen}
		title="Are you sure you want to refresh?"
		description="Refreshing reloads the app and can interrupt the main panel and any open terminal sessions."
		confirmLabel="Refresh"
		onCancel={shortcuts.cancelReload}
		onConfirm={shortcuts.confirmReload}
	/>
	<ConfirmActionModal
		open={Boolean(shortcuts.pendingRename)}
		title="Rename session"
		description="Choose a name for this chat."
		confirmLabel="Save"
		confirmTone="accent"
		showInput
		bind:inputValue={shortcuts.renameTitle}
		inputPlaceholder="New Chat"
		inputMaxLength={200}
		onCancel={shortcuts.cancelRename}
		onConfirm={() => void shortcuts.confirmRename()}
	/>
	<FileSearchModal
		open={shortcuts.fileSearchOpen}
		onClose={() => (shortcuts.fileSearchOpen = false)}
	/>
	{#if shellStore.introOpen}
		{#if panels.Intro}
			<panels.Intro />
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
		background: var(--sidebar-bg);
		box-sizing: border-box;
	}

	.intro-loading {
		position: fixed;
		inset: 0;
		z-index: 90;
		background: var(--intro-bg, var(--color-fafafa));
	}

	.app-shell.sidebar-collapsed {
		--active-sidebar-width: 0px;
	}

	.app-shell.is-fullscreen {
		--traffic-light-gutter: 0px;
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
