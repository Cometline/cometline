<script lang="ts">
	import { page } from '$app/state';
	import { flip } from 'svelte/animate';
	import type { Session } from '#lib/types.js';
	import { sessionStore } from '#lib/stores/session.svelte.js';
	import { startNewChat } from '#lib/actions/new-chat.js';
	import { navigateToSession } from '#lib/actions/navigate-to-session.js';
	import { sessionDisplayTitle } from '#lib/sessions/session-title.js';
	import { shellStore } from '#lib/stores/shell.svelte.js';
	import { isNarrowViewport } from '#lib/layout/narrow-viewport.js';
	import {
		layoutSessionsForSidebar,
		PINNED_GROUP_KEY,
		DISCORD_GROUP_KEY
	} from '#lib/sessions/group-by-workspace.js';
	import SidebarSearch from '#lib/features/sidebar/components/SidebarSearch.svelte';
	import PinnedGroup from '#lib/features/sidebar/components/PinnedGroup.svelte';
	import DiscordGroup from '#lib/features/sidebar/components/DiscordGroup.svelte';
	import WorkspaceGroup from '#lib/features/sidebar/components/WorkspaceGroup.svelte';
	import ConfirmActionModal from '#lib/components/ConfirmActionModal.svelte';
	import SessionContextMenu from '#lib/features/sidebar/components/SessionContextMenu.svelte';
	import SidebarFooter from '#lib/features/sidebar/components/sidebar/SidebarFooter.svelte';
	import { createSidebarSessionActions } from '#lib/features/sidebar/sidebar-session-actions.svelte.js';

	const WORKSPACE_GROUP_FLIP = { duration: 240 };

	let { collapsed = false }: { collapsed?: boolean } = $props();
	let orderWorkspacePath = $derived(shellStore.sidebarOrderWorkspacePath);
	let orderDiscordActive = $derived(shellStore.sidebarOrderDiscordActive);
	let highlightWorkspacePath = $derived.by(() => {
		const current = sessionStore.current;
		if (current?.pinned) {
			return shellStore.sidebarOrderWorkspacePath;
		}
		return current?.workspace_path ?? shellStore.sidebarOrderWorkspacePath;
	});
	const actions = createSidebarSessionActions();
	let searchQuery = $state('');
	let searchInput = $state<HTMLInputElement | null>(null);

	export function focusSearch() {
		searchInput?.focus();
		searchInput?.select();
	}

	function closeSidebarIfNarrow() {
		if (isNarrowViewport()) {
			shellStore.closeSidebar();
		}
	}

	function newChat(workspacePath?: string) {
		void startNewChat(workspacePath);
		closeSidebarIfNarrow();
	}

	function selectSession(session: Session) {
		navigateToSession(session);
		closeSidebarIfNarrow();
	}

	let currentSessionId = $derived(page.params.id ?? null);
	// Groups the user has explicitly collapsed. Groups default to expanded.
	let collapsedGroups = $state<Record<string, boolean>>({});
	let filteredSessions = $derived.by(() => {
		const query = searchQuery.trim().toLowerCase();
		if (!query) return sessionStore.sessions;
		return sessionStore.sessions.filter((session) =>
			sessionDisplayTitle(session.title).toLowerCase().includes(query)
		);
	});
	let sidebarLayout = $derived(
		layoutSessionsForSidebar(filteredSessions, orderWorkspacePath, orderDiscordActive)
	);
	let pinnedSessions = $derived(sidebarLayout.pinnedSessions);
	let groupedSessions = $derived(sidebarLayout.workspaceGroups);
	let discordSessions = $derived(sidebarLayout.discordSessions);
	let discordFirst = $derived(sidebarLayout.discordFirst);
	let hasPinnedSection = $derived(pinnedSessions.length > 0);
	let hasWorkspaceSection = $derived(groupedSessions.length > 0);
	let hasDiscordSection = $derived(discordSessions.length > 0);
	let showDividerAfterPinned = $derived(
		hasPinnedSection && (hasWorkspaceSection || hasDiscordSection)
	);
	let showDividerAfterDiscordFirst = $derived(
		discordFirst && hasDiscordSection && hasWorkspaceSection
	);
	let showDividerBeforeDiscordLast = $derived(
		!discordFirst && hasDiscordSection && hasWorkspaceSection
	);
	let totalSessions = $derived(filteredSessions.length);

	function toggleGroup(path: string) {
		collapsedGroups = { ...collapsedGroups, [path]: !isGroupCollapsed(path) };
	}

	function isGroupCollapsed(path: string): boolean {
		// While searching, force all groups open so matches are always visible.
		if (searchQuery.trim()) return false;
		if (path in collapsedGroups) {
			return Boolean(collapsedGroups[path]);
		}
		// Discord gateway sessions stay folded until explicitly expanded.
		return path === DISCORD_GROUP_KEY;
	}
</script>

<aside
	class="sidebar"
	class:collapsed
	class:modal-open={Boolean(
		actions.pendingDelete || actions.terminalDeleteSession || actions.pendingRename
	)}
	class:context-menu-open={Boolean(actions.contextMenu)}
	aria-hidden={collapsed}
	data-workspace-path={orderWorkspacePath}
>
	<div class="sidebar-content">
		<div class="sidebar-titlebar-row">
			<SidebarSearch bind:searchQuery bind:searchInput onNewChat={newChat} />
		</div>

		<div class="session-list scrollbar-none">
			{#if pinnedSessions.length > 0}
				<PinnedGroup
					sessions={pinnedSessions}
					collapsed={isGroupCollapsed(PINNED_GROUP_KEY)}
					{currentSessionId}
					deletingID={actions.deletingID}
					pinningID={actions.pinningID}
					onToggle={() => toggleGroup(PINNED_GROUP_KEY)}
					onSelectSession={selectSession}
					onDeleteSession={actions.removeSession}
					onPinSession={actions.togglePinSession}
					onSessionContextMenu={actions.openSessionContextMenu}
				/>
			{/if}
			{#if showDividerAfterPinned}
				<div class="sidebar-section-divider" role="separator" aria-hidden="true"></div>
			{/if}
			{#if discordFirst && discordSessions.length > 0}
				<DiscordGroup
					sessions={discordSessions}
					collapsed={isGroupCollapsed(DISCORD_GROUP_KEY)}
					active
					{currentSessionId}
					deletingID={actions.deletingID}
					onToggle={() => toggleGroup(DISCORD_GROUP_KEY)}
					onSelectSession={selectSession}
					onDeleteSession={actions.removeSession}
					onSessionContextMenu={actions.openSessionContextMenu}
				/>
			{/if}
			{#if showDividerAfterDiscordFirst}
				<div class="sidebar-section-divider" role="separator" aria-hidden="true"></div>
			{/if}
			{#each groupedSessions as group (group.workspacePath)}
				<div animate:flip={WORKSPACE_GROUP_FLIP}>
					<WorkspaceGroup
						label={group.label}
						workspacePath={group.workspacePath}
						sessions={group.sessions}
						collapsed={isGroupCollapsed(group.workspacePath)}
						active={group.workspacePath === highlightWorkspacePath}
						searchActive={!!searchQuery.trim()}
						{currentSessionId}
						deletingID={actions.deletingID}
						pinningID={actions.pinningID}
						onToggle={() => toggleGroup(group.workspacePath)}
						onNewSession={newChat}
						onSelectSession={selectSession}
						onDeleteSession={actions.removeSession}
						onPinSession={actions.togglePinSession}
						onRenameSession={actions.startRenameSession}
						onSessionContextMenu={actions.openSessionContextMenu}
					/>
				</div>
			{/each}
			{#if showDividerBeforeDiscordLast}
				<div class="sidebar-section-divider" role="separator" aria-hidden="true"></div>
			{/if}
			{#if !discordFirst && discordSessions.length > 0}
				<DiscordGroup
					sessions={discordSessions}
					collapsed={isGroupCollapsed(DISCORD_GROUP_KEY)}
					{currentSessionId}
					deletingID={actions.deletingID}
					onToggle={() => toggleGroup(DISCORD_GROUP_KEY)}
					onSelectSession={selectSession}
					onDeleteSession={actions.removeSession}
					onSessionContextMenu={actions.openSessionContextMenu}
				/>
			{/if}
			{#if totalSessions === 0}
				<p class="session-empty">
					{searchQuery.trim() ? 'No chats match your search' : 'No chats yet'}
				</p>
			{/if}
		</div>

		<SidebarFooter />
	</div>

	<ConfirmActionModal
		open={Boolean(actions.pendingDelete)}
		title={`Delete "${actions.pendingDelete ? sessionDisplayTitle(actions.pendingDelete.title) : ''}"?`}
		description="This cannot be undone."
		confirmLabel="Delete"
		secondaryLabel="Don't ask again"
		onSecondary={() => void actions.alwaysDeleteWithoutConfirm()}
		onCancel={() => (actions.pendingDelete = null)}
		onConfirm={() => void actions.confirmDelete()}
	/>

	<ConfirmActionModal
		open={Boolean(actions.terminalDeleteSession)}
		title="Delete chat?"
		description="This will terminate this chat's terminal and every program started from it. This cannot be undone."
		confirmLabel="Delete chat"
		onCancel={() => (actions.terminalDeleteSession = null)}
		onConfirm={() => void actions.confirmTerminalDelete()}
	/>

	<ConfirmActionModal
		open={Boolean(actions.pendingRename)}
		title="Rename session"
		description="Choose a name for this chat."
		confirmLabel="Save"
		confirmTone="accent"
		showInput
		bind:inputValue={actions.renameTitle}
		inputPlaceholder="New Chat"
		inputMaxLength={200}
		onCancel={actions.cancelRename}
		onConfirm={() => void actions.confirmRename()}
	/>

	{#if actions.contextMenu}
		{@const menu = actions.contextMenu}
		<SessionContextMenu
			session={menu.session}
			x={menu.x}
			y={menu.y}
			onPin={() => actions.togglePinSession(menu.session)}
			onRename={() => actions.startRenameSession(menu.session)}
			onClose={actions.closeSessionContextMenu}
		/>
	{/if}
</aside>

<style>
	.sidebar {
		position: relative;
		width: var(--active-sidebar-width, var(--sidebar-width));
		flex-shrink: 0;
		display: flex;
		flex-direction: column;
		background: var(--sidebar-bg);
		border-right: none;
		padding: 0;
		overflow: hidden;
		transition: width var(--duration-sidebar) var(--ease-smooth);
		view-transition-name: sidebar;
		--workspace-inactive-color: var(--workspace-group-color);
	}

	.sidebar.modal-open {
		overflow: visible;
	}

	.sidebar.context-menu-open {
		z-index: 2;
		overflow: visible;
	}

	.sidebar-content {
		position: relative;
		z-index: 1;
		width: 100%;
		min-width: 0;
		height: 100%;
		display: flex;
		flex-direction: column;
		opacity: 1;
		transform: translateX(0);
		transition:
			opacity var(--duration-sidebar) var(--ease-smooth),
			transform var(--duration-sidebar) var(--ease-smooth);
	}

	.sidebar.collapsed .sidebar-content {
		opacity: 0;
		transform: translateX(-28px);
		pointer-events: none;
	}

	.sidebar-titlebar-row {
		height: var(--titlebar-height);
		width: 100%;
		flex-shrink: 0;
		display: flex;
		align-items: center;
		padding: 10px 8px;
		padding-left: calc(8px + var(--traffic-light-gutter));
		transition: padding-left var(--duration-fast) var(--ease-smooth);
		-webkit-app-region: drag;
	}

	.session-list {
		flex: 1;
		overflow-y: auto;
		display: flex;
		flex-direction: column;
		gap: 2px;
		padding: 0 8px 12px 8px;
	}

	.session-empty {
		padding: 10px;
		font-size: 12px;
		line-height: 1.4;
		color: var(--text-soft);
		text-align: center;
	}

	.sidebar-section-divider {
		height: 2px;
		margin: 8px 0 6px;
		background: rgba(15, 23, 42, 0.16);
		border-radius: 1px;
		flex-shrink: 0;
	}

	@media (prefers-reduced-motion: reduce) {
		.sidebar,
		.sidebar-content {
			transition: none;
		}
	}

	@media (max-width: 900px) {
		.sidebar:not(.collapsed) {
			background: var(--sidebar-overlay-bg);
		}
	}
</style>
