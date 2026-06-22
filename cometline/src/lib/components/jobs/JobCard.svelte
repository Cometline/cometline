<script lang="ts">
	import type { JobResource } from '$lib/client/cometmind';
	import {
		formatScheduledLabel,
		isJobScheduledNotReady,
		truncateWorkspacePath
	} from '$lib/jobs/group-jobs';

	let {
		job,
		selected = false,
		onclick
	}: {
		job: JobResource;
		selected?: boolean;
		onclick?: () => void;
	} = $props();

	const scheduled = $derived(
		job.scheduled_at != null && isJobScheduledNotReady(job)
	);
	const scheduledLabel = $derived(
		job.scheduled_at != null ? formatScheduledLabel(job.scheduled_at) : ''
	);
	const progressPreview = $derived(job.progress?.trim().split('\n')[0] ?? '');
</script>

<button
	type="button"
	class="job-card"
	class:selected
	class:scheduled
	aria-pressed={selected}
	onclick={() => onclick?.()}
>
	<div class="job-card-top">
		<p class="job-card-title">{job.description}</p>
		{#if (job.priority ?? 0) > 0}
			<span class="job-priority">P{job.priority}</span>
		{/if}
	</div>

	{#if scheduled || job.workspace_path || job.assigned_session_id || progressPreview}
		<div class="job-card-chips">
			{#if scheduled}
				<span class="job-chip scheduled">Scheduled {scheduledLabel}</span>
			{/if}
			{#if job.workspace_path}
				<span class="job-chip" title={job.workspace_path}>
					{truncateWorkspacePath(job.workspace_path)}
				</span>
			{/if}
			{#if job.assigned_session_id}
				<span class="job-chip">Assigned {job.assigned_session_id.slice(0, 8)}</span>
			{/if}
			{#if progressPreview}
				<span class="job-chip progress" title={job.progress}>{progressPreview}</span>
			{/if}
		</div>
	{/if}

</button>

<style>
	.job-card {
		width: 100%;
		text-align: left;
		border: 1px solid var(--border-soft);
		border-radius: 12px;
		background: rgba(255, 255, 255, 0.82);
		padding: 10px 12px;
		display: flex;
		flex-direction: column;
		gap: 8px;
		cursor: pointer;
		transition:
			background var(--duration-fast) var(--ease-smooth),
			border-color var(--duration-fast) var(--ease-smooth),
			box-shadow var(--duration-fast) var(--ease-smooth);
		box-shadow: 0 4px 14px rgba(15, 23, 42, 0.04);
	}

	.job-card:hover {
		background: rgba(255, 255, 255, 0.96);
		border-color: color-mix(in srgb, var(--accent) 24%, var(--border-soft));
	}

	.job-card.selected {
		border-color: var(--pane-focus-border);
		box-shadow: var(--shadow-card);
	}

	.job-card.scheduled {
		opacity: 0.72;
	}

	.job-card-top {
		display: flex;
		align-items: flex-start;
		justify-content: space-between;
		gap: 8px;
	}

	.job-card-title {
		margin: 0;
		font-size: 13px;
		font-weight: 600;
		line-height: 1.45;
		color: var(--text-main);
		display: -webkit-box;
		-webkit-line-clamp: 2;
		line-clamp: 2;
		-webkit-box-orient: vertical;
		overflow: hidden;
	}

	.job-priority {
		flex-shrink: 0;
		font-size: 10px;
		font-weight: 700;
		padding: 2px 6px;
		border-radius: 999px;
		background: color-mix(in srgb, var(--accent) 12%, transparent);
		color: var(--accent);
	}

	.job-card-chips {
		display: flex;
		flex-wrap: wrap;
		gap: 4px;
	}

	.job-chip {
		font-size: 10px;
		line-height: 1.3;
		padding: 3px 7px;
		border-radius: 999px;
		background: rgba(15, 23, 42, 0.06);
		color: var(--text-muted);
		max-width: 100%;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.job-chip.scheduled {
		background: color-mix(in srgb, var(--accent) 10%, transparent);
		color: var(--accent);
	}

	.job-chip.progress {
		max-width: 100%;
	}
</style>
