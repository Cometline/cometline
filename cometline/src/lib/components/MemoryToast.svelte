<script lang="ts">
	import { Brain, CircleCheck, Pencil, Trash2, X } from '@lucide/svelte';
	import { memoryToastStore, type MemoryToastAction } from '$lib/stores/memory-toasts.svelte';

	const icons = {
		create: CircleCheck,
		update: Pencil,
		delete: Trash2,
		supersede: Pencil,
		compact: Brain
	} satisfies Record<MemoryToastAction, typeof Brain>;
</script>

{#if memoryToastStore.toasts.length > 0}
	<div class="toast-container" aria-live="polite" aria-label="Memory updates">
		{#each memoryToastStore.toasts as toast (toast.id)}
			{@const Icon = icons[toast.action]}
			<div class="toast">
				<span class="toast-icon">
					<Icon size={17} strokeWidth={2} aria-hidden="true" />
				</span>
				<div class="toast-body">
					<span class="toast-label">{toast.label}</span>
					<span class="toast-detail">{toast.preview}</span>
				</div>
				<button
					class="toast-dismiss"
					type="button"
					aria-label="Dismiss memory update"
					onclick={() => memoryToastStore.dismiss(toast.id)}
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
		color: var(--text-muted);
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
		border: 0;
		border-radius: 999px;
		color: var(--text-soft);
		background: transparent;
	}

	.toast-dismiss:hover {
		color: var(--text-main);
		background: color-mix(in srgb, var(--text-muted) 10%, transparent);
	}

	@keyframes toast-in {
		from {
			opacity: 0;
			transform: translateY(0.75rem) scale(0.96);
		}
		to {
			opacity: 1;
			transform: translateY(0) scale(1);
		}
	}

	@media (prefers-reduced-motion: reduce) {
		.toast {
			animation: none;
		}
	}
</style>
