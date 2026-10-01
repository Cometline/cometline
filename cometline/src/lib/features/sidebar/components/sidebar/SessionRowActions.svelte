<script lang="ts">
	import { Pin, PinOff, Trash2 } from '@lucide/svelte';
	import type { Session } from '$lib/types';
	import { sessionDisplayTitle } from '$lib/sessions/session-title';

	let {
		session,
		deleting = false,
		pinning = false,
		showPin = true,
		onDelete,
		onPin
	}: {
		session: Session;
		deleting?: boolean;
		pinning?: boolean;
		showPin?: boolean;
		onDelete: () => void;
		onPin: () => void;
	} = $props();
</script>

<div class="session-actions">
	{#if showPin}
		<button
			class="pin-session"
			class:active={session.pinned}
			disabled={pinning}
			onclick={onPin}
			aria-label={session.pinned
				? `Unpin ${sessionDisplayTitle(session.title)}`
				: `Pin ${sessionDisplayTitle(session.title)}`}
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
		aria-label={`Delete ${sessionDisplayTitle(session.title)}`}
		title="Delete session"
	>
		<Trash2 size={13} stroke-width={1.9} />
	</button>
</div>

<style>
	.session-actions {
		position: absolute;
		right: 4px;
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

	:global(.session-row-wrap:hover) .session-actions button,
	:global(.session-row-wrap:focus-within) .session-actions button {
		opacity: 1;
	}

	.pin-session.active {
		color: var(--pinned-group-color, var(--pinned-group-color));
	}

	.pin-session:hover:not(:disabled),
	.delete-session:hover:not(:disabled) {
		background: rgba(0, 0, 0, 0.06);
		color: var(--text-main);
	}

	.pin-session.active:hover:not(:disabled) {
		color: var(--pinned-group-color, var(--pinned-group-color));
	}

	.delete-session:hover:not(:disabled) {
		background: rgba(180, 35, 24, 0.08);
		color: var(--status-error);
	}

	.pin-session:disabled,
	.delete-session:disabled {
		opacity: 0.35;
	}
</style>
