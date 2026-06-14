<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { onMount } from 'svelte';
	import { fade, fly } from 'svelte/transition';
	import { Settings, Search, SquarePen, Trash2 } from '@lucide/svelte';
	import type { Session } from '$lib/types';
	import { sessionStore } from '$lib/stores/session.svelte';
	import { deleteSession } from '$lib/client/cometmind';
	import { startNewChat } from '$lib/actions/new-chat';
	import { chatStore } from '$lib/stores/chat.svelte';
	import { modelStore } from '$lib/stores/model.svelte';
	import { shellStore } from '$lib/stores/shell.svelte';
	import { isNarrowViewport } from '$lib/layout/narrow-viewport';

	let {
		workspacePath = '/',
		collapsed = false
	}: { workspacePath?: string; collapsed?: boolean } = $props();
	let deletingID = $state<string | null>(null);
	let pendingDelete = $state<Session | null>(null);
	let skipDeleteConfirm = $state(false);
	let rememberDeleteChoice = $state(false);
	let searchQuery = $state('');

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
		sessionStore.selectSession(session);
		modelStore.selectFromSession(session);
		void goto(`/session/${session.id}`);
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
	let filteredSessions = $derived.by(() => {
		const query = searchQuery.trim().toLowerCase();
		if (!query) return sessionStore.sessions;
		return sessionStore.sessions.filter((session) =>
			(session.title || 'Untitled').toLowerCase().includes(query)
		);
	});
</script>

<aside class="sidebar" class:collapsed aria-hidden={collapsed} data-workspace-path={workspacePath}>
	<div class="sidebar-content">
		<div
			class="mb-2.5 grid h-11 w-full shrink-0 grid-cols-[minmax(0,1fr)_auto] items-center gap-2 pl-[72px]"
		>
			<label
				class="flex h-7 min-w-0 items-center gap-1.5 rounded-lg border border-border-soft bg-white/70 px-2.5 text-text-soft focus-within:border-slate-900/15 focus-within:bg-white/95 focus-within:text-text-muted"
			>
				<Search size={14} stroke-width={2} aria-hidden="true" />
				<input
					type="search"
					class="min-w-0 flex-1 border-0 bg-transparent p-0 text-xs text-text-main outline-none placeholder:text-text-soft"
					placeholder="Search chats"
					bind:value={searchQuery}
					spellcheck="false"
					autocomplete="off"
					aria-label="Search chats by title"
				/>
			</label>
			<button
				class="grid h-7 w-7 shrink-0 place-items-center rounded-md border-0 bg-transparent text-text-muted hover:bg-black/4 hover:text-text-main active:bg-black/7"
				onclick={newChat}
				aria-label="New chat"
				title="New chat"
			>
				<SquarePen size={16} stroke-width={1.8} />
			</button>
		</div>

		<div class="session-list">
			{#each filteredSessions as session (session.id)}
				<div class="session-row-wrap" class:selected={currentSessionId === session.id}>
					<button class="session-row" onclick={() => selectSession(session)}>
						<span class="session-title">{session.title || 'Untitled'}</span>
						<span class="session-meta">{session.model_id}</span>
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
			{#if filteredSessions.length === 0}
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
		padding: 12px 10px 10px;
		overflow: hidden;
		transition:
			width var(--duration-fast) var(--ease-smooth),
			padding var(--duration-fast) var(--ease-smooth);
		view-transition-name: sidebar;
	}

	.sidebar.collapsed {
		padding-left: 0;
		padding-right: 0;
	}

	.sidebar-content {
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

	.session-title,
	.session-meta {
		overflow: hidden;
		text-overflow: ellipsis;
	}

	.session-meta {
		font-size: 11px;
		font-weight: 400;
		color: var(--text-soft);
	}

	.session-empty {
		padding: 10px;
		font-size: 12px;
		line-height: 1.4;
		color: var(--text-soft);
		text-align: center;
	}

	.session-row-wrap:hover {
		background: rgba(0, 0, 0, 0.04);
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
