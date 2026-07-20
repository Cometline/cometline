<script lang="ts">
	import { X } from '@lucide/svelte';
	import { flip } from 'svelte/animate';
	import type { Session } from '$lib/types';
	import { deleteSession, updateSession } from '$lib/client/cometmind';
	import { createMiniWindowSession } from '$lib/mini-window-session';
	import { chatStore } from '$lib/stores/chat.svelte';
	import { sessionStore } from '$lib/stores/session.svelte';
	import { shellStore } from '$lib/stores/shell.svelte';
	import {
		layoutSessionsForSidebar,
		PINNED_GROUP_KEY,
		DISCORD_GROUP_KEY,
		isDiscordSession
	} from '$lib/sessions/group-by-workspace';
	import SidebarSearch from '$lib/components/sidebar/SidebarSearch.svelte';
	import SessionRow from '$lib/components/sidebar/SessionRow.svelte';
	import Tooltip from '$lib/components/Tooltip.svelte';

	const WORKSPACE_GROUP_FLIP = { duration: 240 };

	let {
		onClose,
		onSelectSession,
		onNewChat
	}: {
		onClose: () => void;
		onSelectSession: (session: Session) => void;
		onNewChat: () => void;
	} = $props();

	let searchQuery = $state('');
	let searchInput = $state<HTMLInputElement | null>(null);
	let collapsedGroups = $state<Record<string, boolean>>({});
	let deletingID = $state<string | null>(null);
	let pinningID = $state<string | null>(null);
	let filteredSessions = $derived.by(() => {
		const query = searchQuery.trim().toLowerCase();
		if (!query) return sessionStore.sessions;
		return sessionStore.sessions.filter((session) =>
			(session.title || 'Untitled').toLowerCase().includes(query)
		);
	});
	let sidebarLayout = $derived(
		layoutSessionsForSidebar(
			filteredSessions,
			shellStore.sidebarOrderWorkspacePath,
			shellStore.sidebarOrderDiscordActive
		)
	);
	let totalSessions = $derived(filteredSessions.length);

	function isCollapsed(key: string) {
		if (searchQuery.trim()) return false;
		if (key in collapsedGroups) return Boolean(collapsedGroups[key]);
		return key === DISCORD_GROUP_KEY;
	}

	function toggleGroup(key: string) {
		collapsedGroups = { ...collapsedGroups, [key]: !isCollapsed(key) };
	}

	function selectSession(session: Session) {
		onSelectSession(session);
	}

	export function focusSearch() {
		searchInput?.focus();
		searchInput?.select();
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

	async function removeSession(session: Session) {
		if (!window.confirm(`Delete ${session.title || 'this chat'}?`)) return;
		deletingID = session.id;
		try {
			await deleteSession(session.id);
			const wasCurrent = sessionStore.current?.id === session.id;
			sessionStore.removeSession(session.id);
			if (wasCurrent) {
				chatStore.clear();
				onClose();
				await createMiniWindowSession();
			}
		} finally {
			deletingID = null;
		}
	}
</script>

<aside class="mini-sidebar" aria-label="Chats">
	<header class="mini-sidebar-header">
		<SidebarSearch bind:searchQuery bind:searchInput onNewChat={onNewChat} />
		<Tooltip label="Close chats">
			<button type="button" class="close-sidebar" onclick={onClose} aria-label="Close chats">
				<X size={16} stroke-width={2} />
			</button>
		</Tooltip>
	</header>

	<div class="session-list scrollbar-none">
		{#if sidebarLayout.pinnedSessions.length > 0}
			<section class="session-section pinned">
				<button class="section-header" aria-expanded={!isCollapsed(PINNED_GROUP_KEY)} onclick={() => toggleGroup(PINNED_GROUP_KEY)}>
					<span>Pinned</span><span>{sidebarLayout.pinnedSessions.length}</span>
				</button>
				{#if !isCollapsed(PINNED_GROUP_KEY)}
					{#each sidebarLayout.pinnedSessions as session (session.id)}
						<SessionRow {session} showWorkspaceLabel selected={sessionStore.current?.id === session.id} deleting={deletingID === session.id} pinning={pinningID === session.id} onSelect={() => selectSession(session)} onDelete={() => void removeSession(session)} onPin={() => void togglePinSession(session)} onContextMenu={() => {}} />
					{/each}
				{/if}
			</section>
		{/if}

		{#each sidebarLayout.workspaceGroups as group (group.workspacePath)}
			<div animate:flip={WORKSPACE_GROUP_FLIP}>
				<section class:active={group.workspacePath === shellStore.workspacePath} class="session-section">
					<button class="section-header" aria-expanded={!isCollapsed(group.workspacePath)} onclick={() => toggleGroup(group.workspacePath)} title={group.workspacePath}>
						<span>{group.label}</span><span>{group.sessions.length}</span>
					</button>
					{#if !isCollapsed(group.workspacePath)}
						{#each group.sessions as session (session.id)}
							<SessionRow {session} selected={sessionStore.current?.id === session.id} deleting={deletingID === session.id} pinning={pinningID === session.id} onSelect={() => selectSession(session)} onDelete={() => void removeSession(session)} onPin={() => void togglePinSession(session)} onContextMenu={() => {}} />
						{/each}
					{/if}
				</section>
			</div>
		{/each}

		{#if sidebarLayout.discordSessions.length > 0}
			<section class="session-section discord">
				<button class="section-header" aria-expanded={!isCollapsed(DISCORD_GROUP_KEY)} onclick={() => toggleGroup(DISCORD_GROUP_KEY)}>
					<span>Discord</span><span>{sidebarLayout.discordSessions.length}</span>
				</button>
				{#if !isCollapsed(DISCORD_GROUP_KEY)}
					{#each sidebarLayout.discordSessions as session (session.id)}
						<SessionRow {session} showGatewayLabel={isDiscordSession(session)} showPin={false} selected={sessionStore.current?.id === session.id} deleting={deletingID === session.id} onSelect={() => selectSession(session)} onDelete={() => void removeSession(session)} onPin={() => {}} onContextMenu={() => {}} />
					{/each}
				{/if}
			</section>
		{/if}

		{#if totalSessions === 0}
			<p class="session-empty">{searchQuery.trim() ? 'No chats match your search' : 'No chats yet'}</p>
		{/if}
	</div>
</aside>

<style>
	.mini-sidebar {
		display: flex;
		flex-direction: column;
		height: 100%;
		background: color-mix(in srgb, var(--panel-bg) 96%, var(--app-bg));
		border-right: 1px solid var(--border-soft);
		box-shadow: 18px 0 46px rgba(0, 0, 0, 0.18);
	}

	.mini-sidebar-header {
		display: flex;
		align-items: center;
		gap: 6px;
		padding: 10px;
		border-bottom: 1px solid var(--border-soft);
	}

	.close-sidebar {
		display: grid;
		place-items: center;
		width: 28px;
		height: 28px;
		flex: 0 0 auto;
		border: 1px solid var(--border-soft);
		border-radius: 8px;
		background: transparent;
		color: var(--text-muted);
		cursor: pointer;
	}

	.close-sidebar:hover {
		color: var(--text-main);
		background: color-mix(in srgb, var(--text-main) 6%, transparent);
	}

	.session-list {
		display: flex;
		flex: 1;
		flex-direction: column;
		gap: 8px;
		overflow-y: auto;
		padding: 10px;
	}

	.session-section {
		display: flex;
		flex-direction: column;
		gap: 2px;
		padding: 2px;
		border: 1px solid transparent;
		border-radius: 9px;
		background: color-mix(in srgb, var(--workspace-inactive-color, #9a9a9f) 8%, transparent);
	}

	.session-section.active {
		border-color: color-mix(in srgb, var(--hero-composer-glow-color, var(--accent)) 28%, transparent);
		background: color-mix(in srgb, var(--hero-composer-glow-color, var(--accent)) 12%, transparent);
	}

	.session-section.pinned {
		background: color-mix(in srgb, var(--pinned-group-color, #b45309) 10%, transparent);
	}

	.session-section.discord {
		background: color-mix(in srgb, var(--discord-group-color, #5865f2) 10%, transparent);
	}

	.section-header {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 8px;
		width: 100%;
		padding: 6px 8px;
		border: 0;
		border-radius: 7px;
		background: transparent;
		color: var(--text-muted);
		font-size: 10px;
		font-weight: 650;
		letter-spacing: 0.04em;
		text-align: left;
		text-transform: uppercase;
		cursor: pointer;
	}

	.section-header span:last-child {
		padding: 1px 5px;
		border-radius: 999px;
		background: color-mix(in srgb, var(--text-main) 7%, transparent);
		font-size: 9px;
	}

	.session-empty {
		margin: 18px 0;
		color: var(--text-muted);
		font-size: 12px;
		text-align: center;
	}
</style>
