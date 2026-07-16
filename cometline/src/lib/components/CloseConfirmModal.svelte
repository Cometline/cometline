<script lang="ts">
	import { fade, scale } from 'svelte/transition';

	let {
		open = false,
		onCancel,
		onClose,
		onAlwaysClose
	}: {
		open?: boolean;
		onCancel: () => void;
		onClose: () => void;
		onAlwaysClose: () => void;
	} = $props();

	function onKeydown(event: KeyboardEvent) {
		if (!open) return;
		if (event.key === 'Escape') {
			event.preventDefault();
			event.stopPropagation();
			onCancel();
			return;
		}
		if (event.key === 'Enter' && !event.metaKey && !event.ctrlKey && !event.altKey) {
			event.preventDefault();
			event.stopPropagation();
			onClose();
			return;
		}
		if (
			event.metaKey &&
			!event.ctrlKey &&
			!event.altKey &&
			!event.shiftKey &&
			event.key.toLowerCase() === 'w'
		) {
			event.preventDefault();
			event.stopPropagation();
			onClose();
		}
	}
</script>

<svelte:window onkeydown={onKeydown} />

{#if open}
	<div class="close-layer" transition:fade={{ duration: 120 }}>
		<button type="button" class="close-scrim" aria-label="Cancel close" onclick={onCancel}
		></button>
		<div
			class="close-modal"
			role="alertdialog"
			aria-modal="true"
			aria-labelledby="close-confirm-title"
			aria-describedby="close-confirm-desc"
			transition:scale={{ start: 0.96, duration: 140 }}
		>
			<img
				class="app-icon"
				src="/project_avatar_96.png"
				alt=""
				width="56"
				height="56"
			/>
			<h2 id="close-confirm-title">Are you sure you want to close Cometline?</h2>
			<p id="close-confirm-desc">
				The window will hide to the menu bar. You can reopen it anytime.
			</p>
			<div class="actions">
				<button type="button" class="btn muted" onclick={onAlwaysClose}>Always close</button>
				<button type="button" class="btn muted" onclick={onCancel}>
					Cancel
					<span class="key-hint" aria-hidden="true">esc</span>
				</button>
				<button type="button" class="btn danger" onclick={onClose}>
					Close
					<span class="key-hint key-hint-light" aria-hidden="true">↵</span>
				</button>
			</div>
		</div>
	</div>
{/if}

<style>
	.close-layer {
		position: fixed;
		inset: 0;
		z-index: 120;
		display: grid;
		place-items: center;
		padding: 24px;
		pointer-events: none;
	}

	.close-scrim {
		position: fixed;
		inset: 0;
		border: none;
		background: rgba(17, 24, 39, 0.32);
		backdrop-filter: blur(8px);
		pointer-events: auto;
		cursor: default;
	}

	.close-modal {
		position: relative;
		z-index: 1;
		width: min(440px, calc(100vw - 48px));
		padding: 22px 22px 16px;
		border-radius: 18px;
		background: rgba(250, 250, 249, 0.98);
		border: 1px solid color-mix(in srgb, var(--border-soft) 80%, transparent);
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
		font-size: 16px;
		font-weight: 650;
		line-height: 1.3;
		color: var(--text-main);
	}

	p {
		margin: 8px 0 0;
		font-size: 13px;
		line-height: 1.45;
		color: var(--text-muted);
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
