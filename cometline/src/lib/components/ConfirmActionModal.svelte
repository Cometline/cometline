<script lang="ts">
	let {
		open = false,
		title,
		description,
		confirmLabel,
		secondaryLabel,
		confirmTone = 'danger',
		showInput = false,
		inputValue = $bindable(''),
		inputPlaceholder,
		inputMaxLength = 200,
		onCancel,
		onConfirm,
		onSecondary
	}: {
		open?: boolean;
		title: string;
		description: string;
		confirmLabel: string;
		secondaryLabel?: string;
		confirmTone?: 'danger' | 'accent';
		showInput?: boolean;
		inputValue?: string;
		inputPlaceholder?: string;
		inputMaxLength?: number;
		onCancel: () => void;
		onConfirm: () => void;
		onSecondary?: () => void;
	} = $props();

	function openModal(dialog: HTMLDialogElement) {
		dialog.showModal();
		// Native dialog focuses the first tabbable control. Secondary actions render first
		// in the action row, so move initial focus to the primary confirm button.
		if (!showInput) {
			queueMicrotask(() => {
				dialog.querySelector<HTMLButtonElement>('[data-confirm-action]')?.focus();
			});
		}
		return () => dialog.close();
	}

	function focusInput(input: HTMLInputElement) {
		input.focus();
		input.select();
	}

	function cancel(event: Event) {
		event.preventDefault();
		onCancel();
	}

	function cancelOnBackdrop(event: MouseEvent) {
		if (event.target === event.currentTarget) onCancel();
	}

	function onKeydown(event: KeyboardEvent) {
		if (!open) return;
		if (event.key === 'Enter' && !event.metaKey && !event.ctrlKey && !event.altKey) {
			event.preventDefault();
			event.stopPropagation();
			onConfirm();
		}
	}
</script>

<svelte:window onkeydown={onKeydown} />

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
		{#if showInput}
			<input
				{@attach focusInput}
				class="confirm-input"
				type="text"
				bind:value={inputValue}
				placeholder={inputPlaceholder}
				maxlength={inputMaxLength}
				aria-label={title}
			/>
		{/if}
		<div class="actions">
			{#if secondaryLabel}
				<button type="button" class="btn muted" onclick={() => onSecondary?.()}
					>{secondaryLabel}</button
				>
			{/if}
			<button type="button" class="btn muted" onclick={onCancel}>
				Cancel
				<span class="key-hint" aria-hidden="true">esc</span>
			</button>
			<button
				type="button"
				class="btn {confirmTone}"
				data-confirm-action
				onclick={onConfirm}
			>
				{confirmLabel}
				<span class="key-hint key-hint-light" aria-hidden="true">↵</span>
			</button>
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
	.confirm-input {
		display: block;
		width: 100%;
		box-sizing: border-box;
		margin: 14px 0 0;
		padding: 9px 11px;
		border: 1px solid var(--border-soft);
		border-radius: 10px;
		background: var(--panel-bg, #fff);
		color: var(--text-main);
		font: inherit;
		font-size: 13px;
		line-height: 1.35;
	}
	.confirm-input:focus {
		outline: none;
		border-color: var(--accent);
		box-shadow: 0 0 0 2px color-mix(in srgb, var(--accent) 18%, transparent);
	}
	.actions {
		display: flex;
		align-items: center;
		justify-content: stretch;
		gap: 8px;
		margin-top: 18px;
	}
	.btn {
		flex: 1 1 0;
		display: inline-flex;
		align-items: center;
		justify-content: center;
		gap: 6px;
		min-height: 34px;
		padding: 0 12px;
		border: none;
		border-radius: 10px;
		font-size: 13px;
		font-weight: 600;
		white-space: nowrap;
		cursor: pointer;
	}
	.btn.muted {
		background: rgba(15, 23, 42, 0.06);
		color: var(--text-main);
	}
	.btn.muted:hover {
		background: rgba(15, 23, 42, 0.1);
	}
	.btn.danger {
		background: #e11d48;
		color: #fff;
	}
	.btn.danger:hover {
		background: #be123c;
	}
	.btn.accent {
		background: var(--accent);
		color: #fff;
	}
	.btn.accent:hover {
		filter: brightness(0.92);
	}
	.key-hint {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		min-width: 1.4rem;
		height: 1.15rem;
		padding: 0 4px;
		border: 1px solid rgba(15, 23, 42, 0.14);
		border-radius: 5px;
		font-size: 10px;
		font-weight: 600;
		line-height: 1;
		color: var(--text-muted);
		text-transform: lowercase;
	}
	.key-hint-light {
		border-color: rgba(255, 255, 255, 0.35);
		color: rgba(255, 255, 255, 0.92);
	}
</style>
