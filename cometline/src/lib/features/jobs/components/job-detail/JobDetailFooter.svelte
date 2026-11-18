<script lang="ts">
	import { Archive, RotateCcw, Trash2, RefreshCw } from '@lucide/svelte';
	import type { JobResource } from '#lib/client/cometmind.js';

	let {
		job,
		saving = false,
		isArchived,
		isBlocked,
		onDelete,
		onArchive,
		onUnarchive,
		onRetry
	}: {
		job: JobResource;
		saving?: boolean;
		isArchived: boolean;
		isBlocked: boolean;
		onDelete?: (job: JobResource) => void | Promise<void>;
		onArchive?: (job: JobResource) => void | Promise<void>;
		onUnarchive?: (job: JobResource) => void | Promise<void>;
		onRetry?: (job: JobResource) => void | Promise<void>;
	} = $props();
</script>

<footer class="drawer-footer">
	{#if isBlocked && !isArchived}
		<button type="button" class="primary" disabled={saving} onclick={() => void onRetry?.(job)}>
			<RefreshCw size={14} />
			Retry now
		</button>
	{/if}
	{#if job.status === 'done' && !isArchived}
		<button
			type="button"
			class="secondary"
			disabled={saving}
			onclick={() => void onArchive?.(job)}
		>
			<Archive size={14} />
			Archive
		</button>
	{/if}
	{#if isArchived}
		<button
			type="button"
			class="secondary"
			disabled={saving}
			onclick={() => void onUnarchive?.(job)}
		>
			<RotateCcw size={14} />
			Unarchive
		</button>
	{/if}
	{#if !isArchived}
		<button
			type="button"
			class="secondary danger"
			disabled={saving}
			onclick={() => void onDelete?.(job)}
		>
			<Trash2 size={14} />
			Delete
		</button>
	{/if}
</footer>

<style>
	.drawer-footer {
		display: flex;
		gap: 8px;
		padding: 12px 16px 16px;
		border-top: 1px solid var(--border-soft);
		background: var(--panel-bg);
	}
</style>
