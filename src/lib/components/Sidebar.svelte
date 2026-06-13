<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { onMount } from 'svelte';
	import { fade, fly } from 'svelte/transition';
	import { Settings, SquarePen, Trash2 } from '@lucide/svelte';
	import type { Session } from '$lib/types';
	import { sessionStore } from '$lib/stores/session.svelte';
	import { deleteSession } from '$lib/client/cometmind';
	import { chatStore } from '$lib/stores/chat.svelte';
	import { modelStore } from '$lib/stores/model.svelte';
	import { shellStore } from '$lib/stores/shell.svelte';

	let { workspacePath = '/', collapsed = false }: { workspacePath?: string; collapsed?: boolean } =
		$props();
	let deletingID = $state<string | null>(null);
	let pendingDelete = $state<Session | null>(null);
	let skipDeleteConfirm = $state(false);
	let rememberDeleteChoice = $state(false);

	onMount(() => {
		skipDeleteConfirm = localStorage.getItem('cometline.skipDeleteConfirm') === 'true';
	});

	function newChat() {
		sessionStore.selectSession(null);
		chatStore.clear();
		shellStore.centerComposer();
		void goto('/');
	}

	function selectSession(session: Session) {
		sessionStore.selectSession(session);
		modelStore.selectFromSession(session);
		void goto(`/session/${session.id}`);
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
</script>

<aside class="sidebar" class:collapsed aria-hidden={collapsed}>
	<div class="sidebar-content">
		<div class="sidebar-header">
		<div class="traffic-spacer" aria-hidden="true"></div>
		<div class="sidebar-actions">
			<button onclick={newChat} aria-label="New chat" title="New chat">
				<SquarePen size={16} stroke-width={1.8} />
			</button>
		</div>
		</div>

		<div class="session-list">
		<button class="new-chat-row" onclick={newChat}>New Chat</button>
		{#each sessionStore.sessions as session (session.id)}
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
				<button class="confirm-delete" onclick={confirmDelete} disabled={deletingID === pendingDelete.id}>
					Delete
				</button>
			</div>
		</div>
		<button class="delete-scrim" aria-label="Cancel delete" onclick={() => (pendingDelete = null)} transition:fade={{ duration: 100 }}></button>
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
		border-right: 1px solid var(--border-soft);
		padding: 12px 10px 10px;
		overflow: hidden;
		transition:
			width var(--duration-fast) var(--ease-smooth),
			padding var(--duration-fast) var(--ease-smooth),
			border-color 180ms ease;
		view-transition-name: sidebar;
	}

	.sidebar.collapsed {
		padding-left: 0;
		padding-right: 0;
		border-right-color: transparent;
	}

	.sidebar-content {
		width: calc(var(--sidebar-width) - 20px);
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

	.sidebar-header {
		display: flex;
		align-items: center;
		justify-content: space-between;
		margin-bottom: 10px;
	}

	.traffic-spacer {
		width: 64px;
	}

	.sidebar-actions {
		display: flex;
		gap: 4px;
	}

	.sidebar-actions button,
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

	.sidebar-actions button:hover,
	.sidebar-footer button:hover {
		background: rgba(0, 0, 0, 0.04);
		color: var(--text-main);
	}

	.sidebar-actions button:active,
	.sidebar-footer button:active {
		background: rgba(0, 0, 0, 0.07);
	}

	.session-list {
		flex: 1;
		overflow-y: auto;
		display: flex;
		flex-direction: column;
		gap: 2px;
	}

	.new-chat-row,
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

	.new-chat-row:hover,
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
</style>
