<script lang="ts">
	import { CircleAlert, CircleCheck, TriangleAlert, X } from '@lucide/svelte';
	import { appToastStore, type AppToastTone } from '#lib/stores/app-toasts.svelte.js';

	const icons = {
		success: CircleCheck,
		warning: TriangleAlert,
		error: CircleAlert
	} satisfies Record<AppToastTone, typeof CircleCheck>;

	function open(id: string, onOpen?: () => void) {
		onOpen?.();
		appToastStore.dismiss(id);
	}
</script>

{#if appToastStore.toasts.length > 0}
	<div class="toast-container" aria-live="polite" aria-label="Notifications">
		{#each appToastStore.toasts as toast (toast.id)}
			{@const Icon = icons[toast.tone]}
			<div class="toast" class:actionable={!!toast.onOpen} data-tone={toast.tone}>
				<button
					class="toast-open"
					type="button"
					disabled={!toast.onOpen}
					onclick={() => open(toast.id, toast.onOpen)}
				>
					<span class="toast-icon">
						<Icon size={17} strokeWidth={2} aria-hidden="true" />
					</span>
					<span class="toast-body">
						<span class="toast-label">{toast.label}</span>
						{#if toast.detail}
							<span class="toast-detail">{toast.detail}</span>
						{/if}
					</span>
				</button>
				<button
					class="toast-dismiss"
					type="button"
					aria-label="Dismiss notification"
					onclick={() => appToastStore.dismiss(toast.id)}
				>
					<X size={14} strokeWidth={2} aria-hidden="true" />
				</button>
			</div>
		{/each}
	</div>
{/if}

<style>
	.toast-container {
		position: fixed;
		right: 1.25rem;
		bottom: 1.25rem;
		z-index: 9999;
		display: flex;
		flex-direction: column;
		gap: 0.5rem;
		width: min(320px, calc(100vw - 2.5rem));
		pointer-events: none;
	}

	.toast {
		display: flex;
		align-items: center;
		gap: 0.25rem;
		min-width: 0;
		padding: 0.375rem 0.375rem 0.375rem 0.25rem;
		background: var(--panel-bg);
		border: 1px solid var(--border-soft);
		border-radius: var(--radius-card);
		box-shadow: var(--shadow-card);
		pointer-events: auto;
		animation: toast-in var(--duration-fast) var(--ease-smooth) both;
	}

	.toast-open {
		display: flex;
		min-width: 0;
		flex: 1;
		align-items: center;
		gap: 0.625rem;
		padding: 0.25rem 0.375rem 0.25rem 0.625rem;
		border: 0;
		border-radius: 8px;
		background: transparent;
		text-align: left;
		color: inherit;
	}

	.toast.actionable .toast-open {
		cursor: pointer;
	}

	.toast.actionable .toast-open:hover {
		background: rgba(15, 23, 42, 0.04);
	}

	.toast-open:disabled {
		cursor: default;
	}

	.toast-icon {
		flex: 0 0 auto;
		color: var(--status-success, var(--status-success));
	}

	.toast[data-tone='warning'] .toast-icon {
		color: var(--status-warning);
	}

	.toast[data-tone='error'] .toast-icon {
		color: var(--status-error);
	}

	.toast-body {
		display: flex;
		min-width: 0;
		flex: 1;
		flex-direction: column;
		gap: 0.125rem;
	}

	.toast-label,
	.toast-detail {
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.toast-label {
		font-size: 0.8125rem;
		font-weight: 600;
		color: var(--text-main);
	}

	.toast-detail {
		font-size: 0.75rem;
		color: var(--text-muted);
	}

	.toast-dismiss {
		display: inline-grid;
		flex: 0 0 auto;
		place-items: center;
		width: 1.5rem;
		height: 1.5rem;
		padding: 0;
		border: none;
		border-radius: 6px;
		background: transparent;
		color: var(--text-muted);
		cursor: pointer;
	}

	.toast-dismiss:hover {
		background: rgba(15, 23, 42, 0.06);
		color: var(--text-main);
	}

	@keyframes toast-in {
		from {
			opacity: 0;
			transform: translateY(6px);
		}
		to {
			opacity: 1;
			transform: translateY(0);
		}
	}
</style>
