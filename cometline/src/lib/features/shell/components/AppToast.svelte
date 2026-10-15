<script lang="ts">
	import type { Attachment } from 'svelte/attachments';
	import { CircleAlert, CircleCheck, TriangleAlert, X } from '@lucide/svelte';
	import { appToastStore, type AppToastTone } from '#lib/stores/app-toasts.svelte.js';

	const icons = {
		success: CircleCheck,
		warning: TriangleAlert,
		error: CircleAlert
	} satisfies Record<AppToastTone, typeof CircleCheck>;

	let expanded = $state<Record<string, boolean>>({});
	let overflow = $state<Record<string, boolean>>({});

	function open(id: string, onOpen?: () => void) {
		onOpen?.();
		appToastStore.dismiss(id);
	}

	// Line-clamp clips inside the text node, so the body box itself does not overflow.
	function trackOverflow(id: string, isExpanded: () => boolean): Attachment {
		return (node) => {
			const measure = () => {
				if (isExpanded()) return;
				const clipped = [...node.querySelectorAll('.toast-label, .toast-detail')].some(
					(el) => el.scrollHeight > el.clientHeight + 1
				);
				if (overflow[id] !== clipped) overflow[id] = clipped;
			};
			const observer = new ResizeObserver(() => requestAnimationFrame(measure));
			observer.observe(node);
			const frame = requestAnimationFrame(measure);
			return () => {
				cancelAnimationFrame(frame);
				observer.disconnect();
			};
		};
	}

	function activate(id: string, onOpen?: () => void) {
		if (onOpen) {
			open(id, onOpen);
			return;
		}
		const next = !expanded[id];
		expanded[id] = next;
		if (next) appToastStore.hold(id);
		else appToastStore.release(id);
	}
</script>

{#if appToastStore.toasts.length > 0}
	<div class="toast-container" aria-live="polite" aria-label="Notifications">
		{#each appToastStore.toasts as toast (toast.id)}
			{@const Icon = icons[toast.tone]}
			{@const isExpanded = !!expanded[toast.id]}
			{@const canExpand = !toast.onOpen && (!!overflow[toast.id] || isExpanded)}
			<div
				class={[
					'toast',
					{ actionable: !!toast.onOpen, expandable: canExpand, expanded: isExpanded }
				]}
				data-tone={toast.tone}
			>
				<button
					class="toast-open"
					type="button"
					disabled={!toast.onOpen && !canExpand}
					aria-expanded={canExpand ? isExpanded : undefined}
					onclick={() => activate(toast.id, toast.onOpen)}
				>
					<span class="toast-icon">
						<Icon size={17} strokeWidth={2} aria-hidden="true" />
					</span>
					<span class="toast-body" {@attach trackOverflow(toast.id, () => isExpanded)}>
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

	.toast.actionable .toast-open,
	.toast.expandable .toast-open {
		cursor: pointer;
	}

	.toast.actionable .toast-open:hover,
	.toast.expandable .toast-open:hover {
		background: rgba(15, 23, 42, 0.04);
	}

	.toast:has(.toast-detail),
	.toast.expanded {
		align-items: flex-start;
	}

	.toast:has(.toast-detail) .toast-open,
	.toast.expanded .toast-open {
		align-items: flex-start;
	}

	.toast:has(.toast-detail) .toast-icon,
	.toast.expanded .toast-icon {
		margin-top: 0.0625rem;
	}

	.toast:has(.toast-detail) .toast-dismiss,
	.toast.expanded .toast-dismiss {
		margin-top: 0.125rem;
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
		display: -webkit-box;
		overflow: hidden;
		-webkit-box-orient: vertical;
		-webkit-line-clamp: 2;
		white-space: normal;
		overflow-wrap: break-word;
	}

	.toast-label {
		font-size: 0.8125rem;
		font-weight: 600;
		color: var(--text-main);
	}

	.toast-detail {
		-webkit-line-clamp: 3;
		font-size: 0.75rem;
		color: var(--text-muted);
	}

	.toast.expanded .toast-label,
	.toast.expanded .toast-detail {
		display: block;
		overflow: visible;
		-webkit-line-clamp: unset;
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
