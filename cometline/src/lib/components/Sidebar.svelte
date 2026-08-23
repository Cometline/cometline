<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { flip } from 'svelte/animate';
	import { Settings, Briefcase, Sparkles, Bell, Images, CircleDollarSign } from '@lucide/svelte';
	import type { Session } from '$lib/types';
	import { sessionStore } from '$lib/stores/session.svelte';
	import { deleteSession, updateSession } from '$lib/client/cometmind';
	import { startNewChat } from '$lib/actions/new-chat';
	import { navigateToSession } from '$lib/actions/navigate-to-session';
	import { sessionDisplayTitle } from '$lib/sessions/session-title';
	import {
		activateAfterSessionDeleted,
		sessionsSnapshot
	} from '$lib/actions/activate-after-session-deleted';
	import { shellStore } from '$lib/stores/shell.svelte';
	import { inboxStore } from '$lib/stores/inbox.svelte';
	import { jobsIndicatorStore } from '$lib/stores/jobs-indicator.svelte';
	import { skillDraftsStore } from '$lib/stores/skill-drafts.svelte';
	import { openSettings } from '$lib/actions/open-settings';
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
	import ConfirmActionModal from '$lib/components/ConfirmActionModal.svelte';
	import SessionContextMenu from '$lib/components/sidebar/SessionContextMenu.svelte';
	import Tooltip from '$lib/components/Tooltip.svelte';
	import { terminalStore } from '$lib/stores/terminal.svelte';
	import { settingsStore } from '$lib/stores/settings.svelte';

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
	let deletingID = $state<string | null>(null);
	let pinningID = $state<string | null>(null);
	let contextMenu = $state<{ session: Session; x: number; y: number } | null>(null);
	let pendingDelete = $state<Session | null>(null);
	let terminalDeleteSession = $state<Session | null>(null);
	let pendingRename = $state<Session | null>(null);
	let renameTitle = $state('');
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

	function newChat() {
		startNewChat();
		closeSidebarIfNarrow();
	}

	function selectSession(session: Session) {
		navigateToSession(session);
		closeSidebarIfNarrow();
	}

	async function removeSession(session: Session) {
		if (terminalStore.isRunning(session.id)) {
			terminalDeleteSession = session;
			return;
		}
		if (terminalStore.hasTerminal(session.id)) await terminalStore.remove(session.id);
		if (settingsStore.settings.app.confirmBeforeDeletingChats) {
			pendingDelete = session;
			return;
		}
		await deleteSelectedSession(session);
	}

	async function confirmTerminalDelete() {
		const session = terminalDeleteSession;
		if (!session) return;
		terminalDeleteSession = null;
		await terminalStore.remove(session.id);
		await deleteSelectedSession(session);
	}

	async function confirmDelete() {
		if (!pendingDelete) return;
		const session = pendingDelete;
		pendingDelete = null;
		await deleteSelectedSession(session);
	}

	async function alwaysDeleteWithoutConfirm() {
		// Persist preference in the background — don't block delete on settings IPC.
		void settingsStore.saveConfirmBeforeDeletingChats(false).catch(() => {});
		await confirmDelete();
	}

	async function deleteSelectedSession(session: Session) {
		deletingID = session.id;
		try {
			const wasCurrent = sessionStore.current?.id === session.id;
			const before = sessionsSnapshot();
			await deleteSession(session.id);
			shellStore.clearWorkspacePanelForSession(session.id);
			sessionStore.removeSession(session.id);
			if (wasCurrent) {
				await activateAfterSessionDeleted(session.id, before);
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
	class:modal-open={Boolean(pendingDelete || terminalDeleteSession || pendingRename)}
	class:context-menu-open={Boolean(contextMenu)}
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
					{deletingID}
					{pinningID}
					onToggle={() => toggleGroup(PINNED_GROUP_KEY)}
					onSelectSession={selectSession}
					onDeleteSession={removeSession}
					onPinSession={togglePinSession}
					onSessionContextMenu={openSessionContextMenu}
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
					{deletingID}
					onToggle={() => toggleGroup(DISCORD_GROUP_KEY)}
					onSelectSession={selectSession}
					onDeleteSession={removeSession}
					onSessionContextMenu={openSessionContextMenu}
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
						{deletingID}
						{pinningID}
						onToggle={() => toggleGroup(group.workspacePath)}
						onSelectSession={selectSession}
						onDeleteSession={removeSession}
						onPinSession={togglePinSession}
						onRenameSession={startRenameSession}
						onSessionContextMenu={openSessionContextMenu}
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

		<div class="sidebar-footer p-2">
			<Tooltip label="Settings" action="openSettings">
				<button aria-label="Settings" onclick={openSettings}>
					<Settings size={16} stroke-width={1.8} />
				</button>
			</Tooltip>
			<Tooltip label="Jobs" action="openJobs">
				<button
					aria-label={jobsIndicatorStore.hasOngoing
						? `Jobs (${jobsIndicatorStore.ongoingCount} ongoing)`
						: 'Jobs'}
					class="nav-badge"
					class:has-badge={jobsIndicatorStore.hasOngoing}
					class:active={page.url.pathname === '/jobs'}
					onclick={() => goto('/jobs')}
				>
					<Briefcase size={16} stroke-width={1.8} />
				</button>
			</Tooltip>
			<Tooltip label="Skills" action="openSkillDrafts">
				<button
					aria-label="Skills"
					class="nav-badge"
					class:has-badge={skillDraftsStore.hasDrafts}
					class:active={page.url.pathname === '/skills' || page.url.pathname === '/skill-drafts'}
					onclick={() => goto('/skills')}
				>
					<Sparkles size={16} stroke-width={1.8} />
				</button>
			</Tooltip>
			<Tooltip label="Gallery" action="openGallery">
				<button
					aria-label="Gallery"
					class="nav-badge"
					class:active={page.url.pathname === '/gallery'}
					onclick={() => goto('/gallery')}
				>
					<Images size={16} stroke-width={1.8} />
				</button>
			</Tooltip>
			<Tooltip label="Usage" action="openUsage">
				<button
					aria-label="Usage"
					class="nav-badge"
					class:active={page.url.pathname === '/usage'}
					onclick={() => goto('/usage')}
				>
					<CircleDollarSign size={16} stroke-width={1.8} />
				</button>
			</Tooltip>
			<Tooltip label="Inbox" action="openInbox">
				<button
					aria-label="Inbox"
					class="nav-badge"
					class:has-badge={inboxStore.openCount > 0}
					class:active={inboxStore.drawerOpen}
					onclick={() => inboxStore.toggleDrawer()}
				>
					<Bell size={16} stroke-width={1.8} />
				</button>
			</Tooltip>
		</div>
	</div>

	<ConfirmActionModal
		open={Boolean(pendingDelete)}
		title={`Delete "${pendingDelete ? sessionDisplayTitle(pendingDelete.title) : ''}"?`}
		description="This cannot be undone."
		confirmLabel="Delete"
		secondaryLabel="Don't ask again"
		onSecondary={() => void alwaysDeleteWithoutConfirm()}
		onCancel={() => (pendingDelete = null)}
		onConfirm={() => void confirmDelete()}
	/>

	<ConfirmActionModal
		open={Boolean(terminalDeleteSession)}
		title="Delete chat?"
		description="This will terminate this chat's terminal and every program started from it. This cannot be undone."
		confirmLabel="Delete chat"
		onCancel={() => (terminalDeleteSession = null)}
		onConfirm={() => void confirmTerminalDelete()}
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
		--workspace-inactive-color: #9a9a9f;
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

	.sidebar-footer button.active {
		background: rgba(0, 0, 0, 0.1);
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

	.sidebar-footer {
		margin-top: auto;
		margin-right: 10px;
		margin-left: 10px;
		padding-top: 8px;
		border-top: 1px solid var(--border-soft);
		display: flex;
		flex-direction: row;
		gap: 4px;
	}

	.sidebar-footer .nav-badge {
		position: relative;
	}

	.sidebar-footer .nav-badge.has-badge::after {
		content: '';
		position: absolute;
		top: 4px;
		right: 4px;
		width: 5px;
		height: 5px;
		border-radius: 999px;
		background: var(--accent);
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
