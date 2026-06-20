<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { onMount } from 'svelte';
	import { flip } from 'svelte/animate';
	import { Settings } from '@lucide/svelte';
	import type { Session } from '$lib/types';
	import { sessionStore } from '$lib/stores/session.svelte';
	import { deleteSession, updateSession } from '$lib/client/cometmind';
	import { startNewChat } from '$lib/actions/new-chat';
	import { navigateToSession } from '$lib/actions/navigate-to-session';
	import { chatStore } from '$lib/stores/chat.svelte';
	import { shellStore } from '$lib/stores/shell.svelte';
	import { isNarrowViewport } from '$lib/layout/narrow-viewport';
	import {
		layoutSessionsForSidebar,
		PINNED_GROUP_KEY,
		DISCORD_GROUP_KEY
	} from '$lib/sessions/group-by-workspace';
	import SidebarSearch from '$lib/components/sidebar/SidebarSearch.svelte';
	import PinnedGroup from '$lib/components/sidebar/PinnedGroup.svelte';
	import DiscordGroup from '$lib/components/sidebar/DiscordGroup.svelte';
	import WorkspaceGroup from '$lib/components/sidebar/WorkspaceGroup.svelte';
	import DeleteConfirmDialog from '$lib/components/sidebar/DeleteConfirmDialog.svelte';
	import RenameSessionDialog from '$lib/components/sidebar/RenameSessionDialog.svelte';
	import SessionContextMenu from '$lib/components/sidebar/SessionContextMenu.svelte';

	const WORKSPACE_GROUP_FLIP = { duration: 240 };

	let { collapsed = false }: { collapsed?: boolean } = $props();
	let orderWorkspacePath = $derived(shellStore.sidebarOrderWorkspacePath);
	let orderDiscordActive = $derived(shellStore.sidebarOrderDiscordActive);
	let highlightWorkspacePath = $derived(
		sessionStore.current?.workspace_path ?? shellStore.sidebarOrderWorkspacePath
	);
	let deletingID = $state<string | null>(null);
	let pinningID = $state<string | null>(null);
	let renamingID = $state<string | null>(null);
	let contextMenu = $state<{ session: Session; x: number; y: number } | null>(null);
	let pendingDelete = $state<Session | null>(null);
	let pendingRename = $state<Session | null>(null);
	let skipDeleteConfirm = $state(false);
	let rememberDeleteChoice = $state(false);
	let searchQuery = $state('');
	let searchInput = $state<HTMLInputElement | null>(null);

	export function focusSearch() {
		searchInput?.focus();
		searchInput?.select();
	}

	onMount(() => {
		skipDeleteConfirm = localStorage.getItem('cometline.skipDeleteConfirm') === 'true';
	});

	function closeSidebarIfNarrow() {
		if (isNarrowViewport()) {
			shellStore.closeSidebar();
		}
	}

	function newChat() {
		startNewChat();
		closeSidebarIfNarrow();
	}

	function selectSession(session: Session) {
		navigateToSession(session);
		closeSidebarIfNarrow();
	}

	async function removeSession(session: Session) {
		if (!skipDeleteConfirm) {
			pendingDelete = session;
			rememberDeleteChoice = false;
			return;
		}
		await deleteSelectedSession(session);
	}

	async function confirmDelete() {
		if (!pendingDelete) return;
		if (rememberDeleteChoice) {
			skipDeleteConfirm = true;
			localStorage.setItem('cometline.skipDeleteConfirm', 'true');
		}
		const session = pendingDelete;
		pendingDelete = null;
		await deleteSelectedSession(session);
	}

	async function deleteSelectedSession(session: Session) {
		deletingID = session.id;
		try {
			await deleteSession(session.id);
			shellStore.clearWebPanelForSession(session.id);
			const wasCurrent = sessionStore.current?.id === session.id;
			sessionStore.removeSession(session.id);
			if (wasCurrent) {
				chatStore.clear();
				await goto('/');
			}
		} finally {
			deletingID = null;
		}
	}

	async function togglePinSession(session: Session) {
		pinningID = session.id;
		try {
			const updated = await updateSession(session.id, { pinned: !session.pinned });
			sessionStore.updateSession(updated);
		} finally {
			pinningID = null;
		}
	}

	function openSessionContextMenu(session: Session, event: MouseEvent) {
		contextMenu = { session, x: event.clientX, y: event.clientY };
	}

	function closeSessionContextMenu() {
		contextMenu = null;
	}

	function startRenameSession(session: Session) {
		pendingRename = session;
	}

	function cancelRename() {
		pendingRename = null;
	}

	async function confirmRename(title: string) {
		if (!pendingRename) return;
		renamingID = pendingRename.id;
		try {
			const updated = await updateSession(pendingRename.id, { title });
			sessionStore.updateSession(updated);
			pendingRename = null;
		} finally {
			renamingID = null;
		}
	}

	let currentSessionId = $derived(page.params.id ?? null);
	// Groups the user has explicitly collapsed. Groups default to expanded.
	let collapsedGroups = $state<Record<string, boolean>>({});
	let filteredSessions = $derived.by(() => {
		const query = searchQuery.trim().toLowerCase();
		if (!query) return sessionStore.sessions;
		return sessionStore.sessions.filter((session) =>
			(session.title || 'Untitled').toLowerCase().includes(query)
		);
	});
	let sidebarLayout = $derived(
		layoutSessionsForSidebar(filteredSessions, orderWorkspacePath, orderDiscordActive)
	);
	let pinnedSessions = $derived(sidebarLayout.pinnedSessions);
	let groupedSessions = $derived(sidebarLayout.workspaceGroups);
	let discordSessions = $derived(sidebarLayout.discordSessions);
	let discordFirst = $derived(sidebarLayout.discordFirst);
	let showActiveWorkspaceDivider = $derived(
		groupedSessions.length > 0 && groupedSessions[0].workspacePath === orderWorkspacePath
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
	aria-hidden={collapsed}
	data-workspace-path={orderWorkspacePath}
>
	<div class="sidebar-content">
		<div class="sidebar-titlebar-row">
			<SidebarSearch bind:searchQuery bind:searchInput onNewChat={newChat} />
		</div>

		<div class="session-list">
			{#if pinnedSessions.length > 0}
				<PinnedGroup
					sessions={pinnedSessions}
					collapsed={isGroupCollapsed(PINNED_GROUP_KEY)}
					{currentSessionId}
					{deletingID}
					{pinningID}
					onToggle={() => toggleGroup(PINNED_GROUP_KEY)}
					onSelectSession={selectSession}
					onDeleteSession={removeSession}
					onPinSession={togglePinSession}
					onSessionContextMenu={openSessionContextMenu}
				/>
			{/if}
			{#if discordFirst && discordSessions.length > 0}
				<DiscordGroup
					sessions={discordSessions}
					collapsed={isGroupCollapsed(DISCORD_GROUP_KEY)}
					active
					showDivider
					{currentSessionId}
					{deletingID}
					onToggle={() => toggleGroup(DISCORD_GROUP_KEY)}
					onSelectSession={selectSession}
					onDeleteSession={removeSession}
					onSessionContextMenu={openSessionContextMenu}
				/>
			{/if}
			{#each groupedSessions as group, index (group.workspacePath)}
				<div animate:flip={WORKSPACE_GROUP_FLIP}>
					<WorkspaceGroup
						label={group.label}
						workspacePath={group.workspacePath}
						sessions={group.sessions}
						collapsed={isGroupCollapsed(group.workspacePath)}
						active={group.workspacePath === highlightWorkspacePath}
						showDivider={index === 0 && showActiveWorkspaceDivider}
						{currentSessionId}
						{deletingID}
						{pinningID}
						onToggle={() => toggleGroup(group.workspacePath)}
						onSelectSession={selectSession}
						onDeleteSession={removeSession}
						onPinSession={togglePinSession}
						onSessionContextMenu={openSessionContextMenu}
					/>
				</div>
			{/each}
			{#if !discordFirst && discordSessions.length > 0}
				<DiscordGroup
					sessions={discordSessions}
					collapsed={isGroupCollapsed(DISCORD_GROUP_KEY)}
					{currentSessionId}
					{deletingID}
					onToggle={() => toggleGroup(DISCORD_GROUP_KEY)}
					onSelectSession={selectSession}
					onDeleteSession={removeSession}
					onSessionContextMenu={openSessionContextMenu}
				/>
			{/if}
			{#if totalSessions === 0}
				<p class="session-empty">
					{searchQuery.trim() ? 'No chats match your search' : 'No chats yet'}
				</p>
			{/if}
		</div>

		<div class="sidebar-footer">
			<button aria-label="Settings" title="Settings" onclick={shellStore.openSettings}>
				<Settings size={16} stroke-width={1.8} />
			</button>
		</div>
	</div>

	{#if pendingDelete}
		<DeleteConfirmDialog
			session={pendingDelete}
			deleting={deletingID === pendingDelete.id}
			bind:rememberDeleteChoice
			onCancel={() => (pendingDelete = null)}
			onConfirm={confirmDelete}
		/>
	{/if}

	{#if pendingRename}
		<RenameSessionDialog
			session={pendingRename}
			renaming={renamingID === pendingRename.id}
			onCancel={cancelRename}
			onConfirm={confirmRename}
		/>
	{/if}

	{#if contextMenu}
		{@const menu = contextMenu}
		<SessionContextMenu
			session={menu.session}
			x={menu.x}
			y={menu.y}
			onPin={() => togglePinSession(menu.session)}
			onRename={() => startRenameSession(menu.session)}
			onClose={closeSessionContextMenu}
		/>
	{/if}
</aside>

<style>
	.sidebar {
		position: relative;
		z-index: 0;
		width: var(--active-sidebar-width, var(--sidebar-width));
		flex-shrink: 0;
		display: flex;
		flex-direction: column;
		background: transparent;
		border-right: none;
		padding: 0;
		overflow: hidden;
		transition: width var(--duration-fast) var(--ease-smooth);
		view-transition-name: sidebar;
		--workspace-inactive-color: #9a9a9f;
	}

	.sidebar-content {
		position: relative;
		z-index: 1;
		width: 100%;
		min-width: 0;
		height: 100%;
		display: flex;
		flex-direction: column;
		transition:
			opacity 150ms ease,
			transform var(--duration-fast) var(--ease-smooth);
	}

	.sidebar.collapsed .sidebar-content {
		opacity: 0;
		transform: translateX(-14px);
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

	.sidebar-footer button {
		width: 28px;
		height: 28px;
		border: none;
		background: transparent;
		border-radius: 6px;
		color: var(--text-muted);
		display: grid;
		place-items: center;
	}

	.sidebar-footer button:hover {
		background: rgba(0, 0, 0, 0.04);
		color: var(--text-main);
	}

	.sidebar-footer button:active {
		background: rgba(0, 0, 0, 0.07);
	}

	.session-list {
		flex: 1;
		overflow-y: auto;
		scrollbar-gutter: stable;
		display: flex;
		flex-direction: column;
		gap: 2px;
		padding: 0 12px 12px 12px;
	}

	.session-empty {
		padding: 10px;
		font-size: 12px;
		line-height: 1.4;
		color: var(--text-soft);
		text-align: center;
	}

	.sidebar-footer {
		margin-top: auto;
		margin-right: 10px;
		margin-left: 10px;
		padding-top: 8px;
		border-top: 1px solid var(--border-soft);
	}

	@media (max-width: 900px) {
		.sidebar:not(.collapsed) {
			background: var(--sidebar-overlay-bg);
		}
	}
</style>
