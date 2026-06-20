<script lang="ts">
	import { Pin, PinOff, Trash2 } from '@lucide/svelte';
	import type { Session } from '$lib/types';
	import { workspaceLabel, gatewaySessionLabel } from '$lib/sessions/group-by-workspace';

	let {
		session,
		selected = false,
		deleting = false,
		pinning = false,
		showWorkspaceLabel = false,
		showGatewayLabel = false,
		showPin = true,
		onSelect,
		onDelete,
		onPin,
		onContextMenu
	}: {
		session: Session;
		selected?: boolean;
		deleting?: boolean;
		pinning?: boolean;
		showWorkspaceLabel?: boolean;
		showGatewayLabel?: boolean;
		showPin?: boolean;
		onSelect: () => void;
		onDelete: () => void;
		onPin: () => void;
		onContextMenu: (event: MouseEvent) => void;
	} = $props();

	function handleContextMenu(event: MouseEvent) {
		event.preventDefault();
		onContextMenu(event);
	}
</script>

<div
	class="session-row-wrap"
	class:selected
	role="group"
	oncontextmenu={handleContextMenu}
>
	<button class="session-row" onclick={onSelect}>
		<span class="session-title">{session.title || 'Untitled'}</span>
		{#if showGatewayLabel}
			<span class="session-workspace session-gateway">{gatewaySessionLabel(session)}</span>
		{:else if showWorkspaceLabel}
			<span class="session-workspace">{workspaceLabel(session.workspace_path)}</span>
		{/if}
	</button>
	<div class="session-actions">
		{#if showPin}
			<button
			class="pin-session"
			class:active={session.pinned}
			disabled={pinning}
			onclick={onPin}
			aria-label={session.pinned ? `Unpin ${session.title || 'Untitled'}` : `Pin ${session.title || 'Untitled'}`}
			title={session.pinned ? 'Unpin session' : 'Pin session'}
		>
			{#if session.pinned}
				<Pin size={13} stroke-width={2} />
			{:else}
				<PinOff size={13} stroke-width={1.9} />
			{/if}
		</button>
		{/if}
		<button
			class="delete-session"
			disabled={deleting}
			onclick={onDelete}
			aria-label={`Delete ${session.title || 'Untitled'}`}
			title="Delete session"
		>
			<Trash2 size={13} stroke-width={1.9} />
		</button>
	</div>
</div>

<style>
	.session-row-wrap {
		position: relative;
		display: flex;
		align-items: stretch;
		border-radius: 8px;
	}

	.session-row-wrap:hover {
		background: rgba(0, 0, 0, 0.08);
	}

	.session-row-wrap.selected {
		background: rgba(0, 0, 0, 0.06);
	}

	.session-row {
		width: 100%;
		text-align: left;
		padding: 7px 10px;
		padding-right: 58px;
		border-radius: 8px;
		border: none;
		background: transparent;
		color: var(--text-main);
		font-size: 13px;
		line-height: 1.35;
		font-weight: 450;
		cursor: pointer;
	}

	.session-title {
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		display: block;
	}

	.session-workspace {
		display: block;
		margin-top: 1px;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		font-size: 10px;
		font-weight: 500;
		line-height: 1.3;
		color: var(--text-soft);
	}

	.session-actions {
		position: absolute;
		right: 5px;
		top: 50%;
		transform: translateY(-50%);
		display: flex;
		align-items: center;
		gap: 2px;
	}

	.pin-session,
	.delete-session {
		width: 24px;
		height: 24px;
		border: none;
		border-radius: 6px;
		background: transparent;
		color: var(--text-soft);
		display: grid;
		place-items: center;
		opacity: 0;
		cursor: pointer;
	}

	.session-row-wrap:hover .session-actions button,
	.session-row-wrap:focus-within .session-actions button {
		opacity: 1;
	}

	.pin-session.active {
		color: var(--pinned-group-color, #b45309);
	}

	.pin-session:hover:not(:disabled),
	.delete-session:hover:not(:disabled) {
		background: rgba(0, 0, 0, 0.06);
		color: var(--text-main);
	}

	.pin-session.active:hover:not(:disabled) {
		color: var(--pinned-group-color, #b45309);
	}

	.delete-session:hover:not(:disabled) {
		background: rgba(180, 35, 24, 0.08);
		color: #b42318;
	}

	.pin-session:disabled,
	.delete-session:disabled {
		opacity: 0.35;
	}
</style>
