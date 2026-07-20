<script lang="ts">
	let {
		open = false,
		title,
		description,
		confirmLabel,
		onCancel,
		onConfirm
	}: {
		open?: boolean;
		title: string;
		description: string;
		confirmLabel: string;
		onCancel: () => void;
		onConfirm: () => void;
		} = $props();

	function openModal(dialog: HTMLDialogElement) {
		dialog.showModal();
		return () => dialog.close();
	}

	function cancel(event: Event) {
		event.preventDefault();
		onCancel();
	}

	function cancelOnBackdrop(event: MouseEvent) {
		if (event.target === event.currentTarget) onCancel();
	}
</script>

{#if open}
	<dialog
		{@attach openModal}
		class="confirm-modal"
		oncancel={cancel}
		onclick={cancelOnBackdrop}
		aria-labelledby="confirm-modal-title"
		aria-describedby="confirm-modal-description"
	>
		<img class="app-icon" src="/project_avatar_96.png" alt="" width="56" height="56" />
		<h2 id="confirm-modal-title">{title}</h2>
		<p id="confirm-modal-description">{description}</p>
		<div class="actions">
			<button type="button" class="btn muted" onclick={onCancel}
				>Cancel <span class="key-hint">esc</span></button
			>
			<button type="button" class="btn danger" onclick={onConfirm}>{confirmLabel}</button>
		</div>
	</dialog>
{/if}

<style>
	.confirm-modal {
		position: fixed;
		top: 50%;
		left: 50%;
		margin: 0;
		transform: translate(-50%, -50%);
		width: min(440px, calc(100vw - 48px));
		padding: 22px 22px 16px;
		border: 1px solid color-mix(in srgb, var(--border-soft) 80%, transparent);
		border-radius: 18px;
		background: rgba(250, 250, 249, 0.98);
		box-shadow: 0 18px 48px rgba(15, 23, 42, 0.18);
		text-align: left;
	}
	.confirm-modal::backdrop {
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
	.actions {
		display: flex;
		gap: 8px;
		margin-top: 18px;
	}
	.btn {
		flex: 1 1 0;
		min-height: 34px;
		border: none;
		border-radius: 10px;
		font-size: 13px;
		font-weight: 600;
		cursor: pointer;
	}
	.btn.muted {
		background: rgba(15, 23, 42, 0.06);
		color: var(--text-main);
	}
	.btn.danger {
		background: #e11d48;
		color: #fff;
	}
	.key-hint {
		margin-left: 6px;
		padding: 1px 4px;
		border: 1px solid rgba(15, 23, 42, 0.14);
		border-radius: 5px;
		color: var(--text-muted);
		font-size: 10px;
	}
</style>
