<script lang="ts">
	import { onMount } from 'svelte';
	import type { Session } from '$lib/types';

	let {
		session,
		deleting = false,
		rememberDeleteChoice = $bindable(false),
		onCancel,
		onConfirm
	}: {
		session: Session;
		deleting?: boolean;
		rememberDeleteChoice?: boolean;
		onCancel: () => void;
		onConfirm: () => void;
	} = $props();

	let dialog = $state<HTMLDialogElement | null>(null);

	onMount(() => {
		dialog?.showModal();
		return () => dialog?.close();
	});

	function cancel(event?: Event) {
		event?.preventDefault();
		onCancel();
	}

	function cancelOnBackdrop(event: MouseEvent) {
		if (event.target === dialog) onCancel();
	}
</script>

<dialog
	bind:this={dialog}
	class="delete-confirm"
	oncancel={cancel}
	onclick={cancelOnBackdrop}
	aria-labelledby="delete-confirm-title"
	aria-describedby="delete-confirm-description"
>
	<img class="app-icon" src="/project_avatar_96.png" alt="" width="56" height="56" />
	<div class="delete-copy">
		<h2 id="delete-confirm-title">Delete “{session.title || 'Untitled'}”?</h2>
		<p id="delete-confirm-description">This cannot be undone.</p>
	</div>
	<label class="delete-check">
		<input type="checkbox" bind:checked={rememberDeleteChoice} />
		<span>Don’t ask again</span>
	</label>
	<div class="delete-actions">
		<button class="cancel-delete" onclick={onCancel}>Cancel</button>
		<button class="confirm-delete" onclick={onConfirm} disabled={deleting}>Delete</button>
	</div>
</dialog>

<style>
	.delete-confirm {
		display: grid;
		gap: 0;
		width: min(440px, calc(100vw - 48px));
		margin: auto;
		padding: 22px 22px 16px;
		border: 1px solid color-mix(in srgb, var(--border-soft) 80%, transparent);
		border-radius: 18px;
		background: rgba(250, 250, 249, 0.98);
		box-shadow: 0 18px 48px rgba(15, 23, 42, 0.18);
	}

	.delete-confirm::backdrop {
		background: rgba(17, 24, 39, 0.32);
		backdrop-filter: blur(8px);
	}

	.app-icon {
		display: block;
		width: 56px;
		height: 56px;
		margin: 0 0 14px;
		border-radius: 14px;
		object-fit: cover;
	}

	.delete-copy {
		display: grid;
		gap: 4px;
	}

	h2 {
		margin: 0;
		color: var(--text-main);
		font-size: 16px;
		font-weight: 650;
		line-height: 1.3;
	}

	p {
		margin: 8px 0 0;
		color: var(--text-muted);
		font-size: 13px;
		line-height: 1.45;
	}

	.delete-check {
		display: flex;
		align-items: center;
		gap: 8px;
		margin-top: 14px;
		font-size: 11px;
		color: var(--text-muted);
	}

	.delete-actions {
		display: flex;
		gap: 8px;
		margin-top: 18px;
	}

	.cancel-delete,
	.confirm-delete {
		flex: 1 1 0;
		min-height: 34px;
		border: none;
		border-radius: 8px;
		padding: 0 10px;
		font: inherit;
		font-size: 12px;
		font-weight: 600;
		cursor: pointer;
	}

	.cancel-delete {
		background: rgba(15, 23, 42, 0.06);
		color: var(--text-main);
	}

	.confirm-delete {
		background: #e11d48;
		color: white;
	}

	.confirm-delete:disabled {
		opacity: 0.5;
		cursor: not-allowed;
	}

</style>
