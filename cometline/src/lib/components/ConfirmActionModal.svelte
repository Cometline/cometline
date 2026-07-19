<script lang="ts">
	import { fade, scale } from 'svelte/transition';

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

	function onKeydown(event: KeyboardEvent) {
		if (!open) return;
		if (event.key === 'Escape') {
			event.preventDefault();
			onCancel();
		}
	}
</script>

<svelte:window onkeydown={onKeydown} />

{#if open}
	<div class="confirm-layer" transition:fade={{ duration: 120 }}>
		<button type="button" class="confirm-scrim" aria-label="Cancel" onclick={onCancel}></button>
		<div
			class="confirm-modal"
			role="alertdialog"
			aria-modal="true"
			transition:scale={{ start: 0.96, duration: 140 }}
		>
			<img class="app-icon" src="/project_avatar_96.png" alt="" width="56" height="56" />
			<h2>{title}</h2>
			<p>{description}</p>
			<div class="actions">
				<button type="button" class="btn muted" onclick={onCancel}
					>Cancel <span class="key-hint">esc</span></button
				>
				<button type="button" class="btn danger" onclick={onConfirm}>{confirmLabel}</button>
			</div>
		</div>
	</div>
{/if}

<style>
	.confirm-layer {
		position: fixed;
		inset: 0;
		z-index: 120;
		display: grid;
		place-items: center;
		padding: 24px;
		pointer-events: none;
	}
	.confirm-scrim {
		position: fixed;
		inset: 0;
		border: none;
		background: rgba(17, 24, 39, 0.32);
		backdrop-filter: blur(8px);
		pointer-events: auto;
		cursor: default;
	}
	.confirm-modal {
		position: relative;
		z-index: 1;
		width: min(440px, calc(100vw - 48px));
		padding: 22px 22px 16px;
		border: 1px solid color-mix(in srgb, var(--border-soft) 80%, transparent);
		border-radius: 18px;
		background: rgba(250, 250, 249, 0.98);
		box-shadow: 0 18px 48px rgba(15, 23, 42, 0.18);
		text-align: left;
		pointer-events: auto;
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
