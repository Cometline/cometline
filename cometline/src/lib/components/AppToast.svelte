<script lang="ts">
	import { CircleCheck, X } from '@lucide/svelte';
	import { appToastStore } from '$lib/stores/app-toasts.svelte';
</script>

{#if appToastStore.toasts.length > 0}
	<div class="toast-container" aria-live="polite" aria-label="Notifications">
		{#each appToastStore.toasts as toast (toast.id)}
			<div class="toast">
				<span class="toast-icon">
					<CircleCheck size={17} strokeWidth={2} aria-hidden="true" />
				</span>
				<div class="toast-body">
					<span class="toast-label">{toast.label}</span>
					{#if toast.detail}
						<span class="toast-detail">{toast.detail}</span>
					{/if}
				</div>
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
		gap: 0.625rem;
		min-width: 0;
		padding: 0.625rem 0.75rem 0.625rem 0.875rem;
		background: var(--panel-bg);
		border: 1px solid var(--border-soft);
		border-radius: var(--radius-card);
		box-shadow: var(--shadow-card);
		pointer-events: auto;
		animation: toast-in var(--duration-fast) var(--ease-smooth) both;
	}

	.toast-icon {
		flex: 0 0 auto;
		color: var(--status-success, #15803d);
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
