<script lang="ts">
	import { Archive, RotateCcw, Trash2, ExternalLink, RefreshCw } from '@lucide/svelte';
	import type { JobResource } from '#lib/client/cometmind.js';

	let {
		job,
		saving = false,
		openingSession,
		isArchived,
		isBlocked,
		onOpenSession,
		onDelete,
		onArchive,
		onUnarchive,
		onRetry
	}: {
		job: JobResource;
		saving?: boolean;
		openingSession: boolean;
		isArchived: boolean;
		isBlocked: boolean;
		onOpenSession: () => void;
		onDelete?: (job: JobResource) => void | Promise<void>;
		onArchive?: (job: JobResource) => void | Promise<void>;
		onUnarchive?: (job: JobResource) => void | Promise<void>;
		onRetry?: (job: JobResource) => void | Promise<void>;
	} = $props();
</script>

<footer class="drawer-footer">
	{#if job.assigned_session_id && job.status === 'done'}
		<button
			type="button"
			class="secondary"
			title="Open the completed session transcript."
			disabled={openingSession || saving}
			onclick={onOpenSession}
		>
			<ExternalLink size={14} />
			{openingSession ? 'Opening…' : 'Open session'}
		</button>
	{/if}
	{#if isBlocked && !isArchived}
		<button
			type="button"
			class="primary"
			disabled={saving || openingSession}
			onclick={() => void onRetry?.(job)}
		>
			<RefreshCw size={14} />
			Retry now
		</button>
	{/if}
	{#if job.status === 'done' && !isArchived}
		<button
			type="button"
			class="secondary"
			disabled={saving || openingSession}
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
			disabled={saving || openingSession}
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
			disabled={saving || openingSession}
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
