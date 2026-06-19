<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { onMount } from 'svelte';
	import { flip } from 'svelte/animate';
	import { fade, fly, slide } from 'svelte/transition';
	import {
		ChevronDown,
		ChevronRight,
		Folder,
		Settings,
		Search,
		SquarePen,
		Trash2
	} from '@lucide/svelte';
	import type { Session } from '$lib/types';
	import { sessionStore } from '$lib/stores/session.svelte';
	import { deleteSession } from '$lib/client/cometmind';
	import { startNewChat } from '$lib/actions/new-chat';
	import { navigateToSession } from '$lib/actions/navigate-to-session';
	import { chatStore } from '$lib/stores/chat.svelte';
	import { shellStore } from '$lib/stores/shell.svelte';
	import { isNarrowViewport } from '$lib/layout/narrow-viewport';
	import { groupSessionsByWorkspace } from '$lib/sessions/group-by-workspace';

	const WORKSPACE_SESSIONS_SLIDE = { duration: 180 };
	const WORKSPACE_GROUP_FLIP = { duration: 240 };

	let { collapsed = false }: { collapsed?: boolean } = $props();
	let orderWorkspacePath = $derived(shellStore.sidebarOrderWorkspacePath);
	let highlightWorkspacePath = $derived(
		sessionStore.current?.workspace_path ?? shellStore.sidebarOrderWorkspacePath
	);
	let deletingID = $state<string | null>(null);
	let pendingDelete = $state<Session | null>(null);
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
	let groupedSessions = $derived(groupSessionsByWorkspace(filteredSessions, orderWorkspacePath));
	let showWorkspaceDivider = $derived(
		groupedSessions.length > 1 && groupedSessions[0].workspacePath === orderWorkspacePath
	);
	let totalSessions = $derived(filteredSessions.length);

	function toggleGroup(path: string) {
		collapsedGroups = { ...collapsedGroups, [path]: !collapsedGroups[path] };
	}

	function isGroupCollapsed(path: string): boolean {
		// While searching, force all groups open so matches are always visible.
		if (searchQuery.trim()) return false;
		return Boolean(collapsedGroups[path]);
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
			<div class="search-field-wrap no-drag">
				<div
					class="search-composite flex h-7 min-w-0 items-stretch overflow-hidden rounded-lg border border-border-soft bg-white/70 text-text-soft focus-within:border-slate-900/15 focus-within:bg-white/95 focus-within:text-text-muted"
				>
					<label class="search-field flex min-w-0 flex-1 items-center gap-2 px-2.5">
						<Search size={14} stroke-width={2} aria-hidden="true" class="shrink-0" />
						<input
							type="search"
							class="min-w-0 flex-1 border-0 bg-transparent p-0 text-xs text-text-main outline-none placeholder:text-text-soft"
							placeholder="Search chats"
							bind:value={searchQuery}
							bind:this={searchInput}
							spellcheck="false"
							autocomplete="off"
							aria-label="Search chats by title"
						/>
					</label>
					<div class="search-divider" aria-hidden="true"></div>
					<button
						class="new-chat-button grid w-7 shrink-0 place-items-center border-0 bg-transparent text-text-muted hover:text-text-main"
						onclick={newChat}
						aria-label="New chat"
						title="New chat"
					>
						<SquarePen size={16} stroke-width={1.8} />
					</button>
				</div>
			</div>
		</div>

		<div class="session-list">
			{#each groupedSessions as group, index (group.workspacePath)}
				{@const collapsed = isGroupCollapsed(group.workspacePath)}
				{@const isActive = group.workspacePath === highlightWorkspacePath}
				<div class="workspace-entry" animate:flip={WORKSPACE_GROUP_FLIP}>
					<div class="workspace-group" class:active={isActive}>
						<button
							class="workspace-header"
							aria-expanded={!collapsed}
							aria-current={isActive ? 'true' : undefined}
							onclick={() => toggleGroup(group.workspacePath)}
							title={group.workspacePath}
						>
							<span class="workspace-chevron">
								{#if collapsed}
									<ChevronRight size={13} stroke-width={2} />
								{:else}
									<ChevronDown size={13} stroke-width={2} />
								{/if}
							</span>
							<Folder size={13} stroke-width={1.8} class="workspace-folder" />
							<span class="workspace-label">{group.label}</span>
							<span class="workspace-count">{group.sessions.length}</span>
						</button>

						{#if !collapsed}
							<div
								class="workspace-sessions"
								transition:slide={WORKSPACE_SESSIONS_SLIDE}
							>
								{#each group.sessions as session (session.id)}
									<div
										class="session-row-wrap"
										class:selected={currentSessionId === session.id}
									>
										<button
											class="session-row"
											onclick={() => selectSession(session)}
										>
											<span class="session-title"
												>{session.title || 'Untitled'}</span
											>
										</button>
										<button
											class="delete-session"
											disabled={deletingID === session.id}
											onclick={() => removeSession(session)}
											aria-label={`Delete ${session.title || 'Untitled'}`}
											title="Delete session"
										>
											<Trash2 size={13} stroke-width={1.9} />
										</button>
									</div>
								{/each}
							</div>
						{/if}
					</div>
					{#if index === 0 && showWorkspaceDivider}
						<div class="workspace-divider" role="separator" aria-hidden="true"></div>
					{/if}
				</div>
			{/each}
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
		<div class="delete-confirm" transition:fly={{ y: 8, duration: 140 }}>
			<div class="delete-copy">
				<strong>Delete “{pendingDelete.title || 'Untitled'}”?</strong>
				<span>This cannot be undone.</span>
			</div>
			<label class="delete-check">
				<input type="checkbox" bind:checked={rememberDeleteChoice} />
				<span>Don’t ask again</span>
			</label>
			<div class="delete-actions">
				<button class="cancel-delete" onclick={() => (pendingDelete = null)}>Cancel</button>
				<button
					class="confirm-delete"
					onclick={confirmDelete}
					disabled={deletingID === pendingDelete.id}
				>
					Delete
				</button>
			</div>
		</div>
		<button
			class="delete-scrim"
			aria-label="Cancel delete"
			onclick={() => (pendingDelete = null)}
			transition:fade={{ duration: 100 }}
		></button>
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

	.search-field-wrap {
		flex: 1;
		min-width: 0;
	}

	.search-composite {
		width: 100%;
	}

	.search-field input {
		overflow: hidden;
		text-overflow: ellipsis;
	}

	.search-field input::-webkit-search-decoration,
	.search-field input::-webkit-search-cancel-button {
		-webkit-appearance: none;
		appearance: none;
	}

	.search-divider {
		width: 2px;
		align-self: center;
		height: 16px;
		background: var(--border-soft);
		flex-shrink: 0;
		border-radius: 1px;
	}

	.new-chat-button {
		border-radius: 0;
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

	.workspace-entry {
		display: flex;
		flex-direction: column;
		gap: 2px;
	}

	.workspace-group {
		display: flex;
		flex-direction: column;
		gap: 4px;
		border-radius: 8px;
		padding: 2px;
		border-left: 2px solid transparent;
		transition:
			background var(--duration-fast) var(--ease-smooth),
			border-color var(--duration-fast) var(--ease-smooth);
	}

	.workspace-group:not(.active) {
		border-left: 3px solid var(--workspace-inactive-color);
		padding-left: 4px;
		margin-left: -6px;
		margin-right: -6px;
		background: color-mix(in srgb, var(--workspace-inactive-color) 15%, transparent);
	}

	.workspace-group:not(.active):hover {
		background: color-mix(in srgb, var(--workspace-inactive-color) 30%, transparent);
	}

	.workspace-group.active {
		border-left: 3px solid var(--hero-composer-glow-color, var(--accent));
		padding-left: 4px;
		margin-left: -6px;
		margin-right: -6px;
		background: color-mix(
			in srgb,
			var(--hero-composer-glow-color, var(--accent)) 30%,
			transparent
		);
	}

	.workspace-group.active:hover {
		background: color-mix(
			in srgb,
			var(--hero-composer-glow-color, var(--accent)) 40%,
			transparent
		);
	}

	.workspace-group.active .workspace-label {
		color: var(--text-main);
	}

	.workspace-group.active .workspace-chevron,
	.workspace-group.active :global(.workspace-folder) {
		color: var(--hero-composer-glow-color, var(--accent));
	}

	.workspace-divider {
		height: 2px;
		margin: 8px 6px 6px;
		background: rgba(15, 23, 42, 0.16);
		border-radius: 1px;
		flex-shrink: 0;
	}

	.workspace-header {
		display: flex;
		align-items: center;
		gap: 6px;
		width: 100%;
		padding: 6px 8px;
		border: none;
		border-radius: 7px;
		background: transparent;
		color: var(--workspace-inactive-color);
		font-size: 11px;
		font-weight: 600;
		text-transform: uppercase;
		letter-spacing: 0.02em;
		cursor: pointer;
		text-align: left;
	}

	.workspace-group:hover .workspace-header {
		color: var(--text-muted);
	}

	.workspace-chevron {
		display: grid;
		place-items: center;
		flex-shrink: 0;
		color: var(--workspace-inactive-color);
		transition: color var(--duration-fast) var(--ease-smooth);
	}

	.workspace-header :global(.workspace-folder) {
		flex-shrink: 0;
		color: var(--workspace-inactive-color);
		transition: color var(--duration-fast) var(--ease-smooth);
	}

	.workspace-label {
		min-width: 0;
		flex: 1;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		transition: color var(--duration-fast) var(--ease-smooth);
	}

	.workspace-count {
		flex-shrink: 0;
		font-size: 10px;
		font-weight: 600;
		color: var(--text-soft);
		background: rgba(15, 23, 42, 0.06);
		border-radius: 999px;
		padding: 1px 6px;
	}

	.workspace-sessions {
		display: flex;
		flex-direction: column;
		gap: 2px;
		padding-left: 6px;
	}

	.session-row {
		width: 100%;
		text-align: left;
		padding: 7px 10px;
		border-radius: 8px;
		border: none;
		background: transparent;
		color: var(--text-main);
		font-size: 13px;
		line-height: 1.35;
		font-weight: 450;
		display: flex;
		flex-direction: column;
		gap: 2px;
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}

	.session-row-wrap {
		position: relative;
		display: flex;
		align-items: stretch;
		border-radius: 8px;
	}

	.session-row-wrap .session-row {
		padding-right: 34px;
	}

	.delete-session {
		position: absolute;
		right: 5px;
		top: 50%;
		transform: translateY(-50%);
		width: 24px;
		height: 24px;
		border: none;
		border-radius: 6px;
		background: transparent;
		color: var(--text-soft);
		display: grid;
		place-items: center;
		opacity: 0;
	}

	.session-title {
		overflow: hidden;
		text-overflow: ellipsis;
	}

	.session-empty {
		padding: 10px;
		font-size: 12px;
		line-height: 1.4;
		color: var(--text-soft);
		text-align: center;
	}

	.session-row-wrap:hover {
		background: rgba(0, 0, 0, 0.08);
	}

	.session-row-wrap:hover .delete-session,
	.session-row-wrap:focus-within .delete-session {
		opacity: 1;
	}

	.delete-session:hover:not(:disabled) {
		background: rgba(180, 35, 24, 0.08);
		color: #b42318;
	}

	.delete-session:disabled {
		opacity: 0.35;
	}

	.session-row-wrap.selected {
		background: rgba(0, 0, 0, 0.06);
	}

	.sidebar-footer {
		margin-top: auto;
		margin-right: 10px;
		margin-left: 10px;
		padding-top: 8px;
		border-top: 1px solid var(--border-soft);
	}

	.delete-scrim {
		position: fixed;
		inset: 0;
		z-index: 39;
		border: none;
		background: transparent;
	}

	.delete-confirm {
		position: absolute;
		left: 12px;
		right: 12px;
		bottom: 46px;
		z-index: 40;
		border: 1px solid rgba(229, 231, 235, 0.95);
		border-radius: 14px;
		background: rgba(255, 255, 255, 0.98);
		box-shadow: 0 18px 48px rgba(15, 23, 42, 0.16);
		padding: 12px;
	}

	.delete-copy {
		display: grid;
		gap: 3px;
		margin-bottom: 10px;
	}

	.delete-copy strong {
		font-size: 13px;
		font-weight: 650;
		color: var(--text-main);
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.delete-copy span,
	.delete-check {
		font-size: 12px;
		color: var(--text-muted);
	}

	.delete-check {
		display: flex;
		align-items: center;
		gap: 7px;
		margin-bottom: 12px;
	}

	.delete-check input {
		width: 14px;
		height: 14px;
		accent-color: var(--text-main);
	}

	.delete-actions {
		display: flex;
		justify-content: flex-end;
		gap: 7px;
	}

	.cancel-delete,
	.confirm-delete {
		border: none;
		border-radius: 9px;
		padding: 7px 10px;
		font: inherit;
		font-size: 12px;
		font-weight: 600;
	}

	.cancel-delete {
		background: rgba(15, 23, 42, 0.05);
		color: var(--text-main);
	}

	.confirm-delete {
		background: #b42318;
		color: white;
	}

	@media (max-width: 900px) {
		.sidebar:not(.collapsed) {
			background: var(--sidebar-overlay-bg);
		}
	}
</style>
