<script lang="ts">
	import { CalendarClock, LoaderCircle, Pencil, Plus, Trash2, X } from '@lucide/svelte';
	import { formatClock, nextRunLabel, scheduleLabel } from '$lib/features/jobs/jobs-page-format';
	import type { JobsScheduleController } from '$lib/features/jobs/jobs-page-schedule.svelte';
	import JobsScheduleForm from './JobsScheduleForm.svelte';

	let {
		schedule,
		saving,
		nowMs
	}: {
		schedule: JobsScheduleController;
		saving: boolean;
		nowMs: number;
	} = $props();
</script>

<section class="scheduled-panel settings-panel-frame">
	<header class="scheduled-header">
		<h2>Scheduled</h2>
		<div class="scheduled-header-actions">
			{#if schedule.showForm}
				<button type="button" class="secondary" onclick={schedule.cancelForm}>
					<X size={14} />
					Cancel
				</button>
				<button
					type="submit"
					class="primary"
					form="schedule-job-form"
					disabled={saving || !schedule.description.trim()}
				>
					{#if saving}
						<LoaderCircle size={14} class="spin" />
					{/if}
					{schedule.editingId ? 'Save schedule' : 'Create schedule'}
				</button>
			{:else}
				<button type="button" class="secondary" onclick={schedule.openNewForm}>
					<Plus size={14} />
					Schedule job
				</button>
			{/if}
		</div>
	</header>

	{#if schedule.showForm}
		<JobsScheduleForm {schedule} />
	{/if}

	{#if !schedule.showForm}
		{#if schedule.scheduledJobs.length === 0}
			<p class="jobs-muted">No scheduled jobs. Create one to defer work.</p>
		{:else}
			<div class="scheduled-list scrollbar-none">
				{#each schedule.scheduledJobs as job (job.id)}
					<div class="scheduled-card" class:disabled={!job.enabled}>
						<div class="scheduled-card-main">
							<strong title={job.description}>{job.description}</strong>
							<div class="scheduled-card-meta">
								<span class="chip">
									<CalendarClock size={11} />
									{scheduleLabel(job)}
								</span>
								<span class="chip">Next: {nextRunLabel(job, nowMs)}</span>
								{#if job.last_run_at}
									<span class="chip">Last: {formatClock(job.last_run_at)}</span>
								{/if}
							</div>
						</div>
						<div class="scheduled-card-actions">
							<button
								type="button"
								class="secondary icon-only"
								title="Edit"
								onclick={() => schedule.editScheduled(job)}
							>
								<Pencil size={14} />
							</button>
							<button
								type="button"
								class="secondary icon-only"
								title={job.enabled ? 'Disable' : 'Enable'}
								onclick={() => void schedule.handleToggleScheduled(job)}
							>
								{job.enabled ? 'Disable' : 'Enable'}
							</button>
							<button
								type="button"
								class="danger icon-only"
								title="Delete"
								onclick={() => void schedule.handleDeleteScheduled(job)}
							>
								<Trash2 size={14} />
							</button>
						</div>
					</div>
				{/each}
			</div>
		{/if}
	{/if}
</section>

<style>
	.jobs-muted {
		margin: 0;
		font-size: 12px;
		color: var(--text-muted);
	}

	/* Keep before the base rules below, which override this block's align-items, justify-content, and actions flex-direction. */
	@container main-pane (max-width: 760px) {
		.scheduled-header {
			flex-direction: column;
			align-items: flex-start;
		}

		.scheduled-header-actions {
			width: 100%;
			justify-content: flex-start;
		}

		.scheduled-card {
			flex-direction: column;
		}

		.scheduled-card-actions {
			width: 100%;
			flex-direction: row;
			flex-wrap: wrap;
		}
	}

	.scheduled-panel {
		display: flex;
		flex-direction: column;
		gap: 12px;
		min-height: 0;
		height: 100%;
		padding: 14px;
		overflow: hidden;
	}

	.scheduled-header {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 8px;
	}

	.scheduled-header-actions {
		display: flex;
		align-items: center;
		gap: 8px;
		flex-wrap: wrap;
		justify-content: flex-end;
	}

	.scheduled-header h2 {
		margin: 0;
		font-size: 14px;
		font-weight: 650;
	}

	.scheduled-list {
		display: flex;
		flex-direction: column;
		gap: 10px;
		overflow-y: auto;
		min-height: 0;
		padding-right: 2px;
	}

	.scheduled-card {
		display: flex;
		align-items: flex-start;
		justify-content: space-between;
		gap: 10px;
		border: 1px solid var(--border-soft);
		border-radius: 10px;
		background: var(--panel-bg);
		padding: 10px 12px;
	}

	.scheduled-card.disabled {
		opacity: 0.55;
	}

	.scheduled-card-main {
		display: flex;
		flex: 1;
		flex-direction: column;
		gap: 6px;
		min-width: 0;
	}

	.scheduled-card-main strong {
		font-size: 12px;
		line-height: 1.45;
		color: var(--text-main);
		display: -webkit-box;
		-webkit-line-clamp: 2;
		line-clamp: 2;
		-webkit-box-orient: vertical;
		overflow: hidden;
		word-break: break-word;
	}

	.scheduled-card-meta {
		display: flex;
		flex-wrap: wrap;
		gap: 5px;
	}

	.chip {
		display: inline-flex;
		align-items: center;
		gap: 3px;
		font-size: 10px;
		font-weight: 600;
		padding: 2px 7px;
		border-radius: 999px;
		background: rgba(15, 23, 42, 0.06);
		color: var(--text-muted);
	}

	.scheduled-card-actions {
		display: flex;
		flex-direction: column;
		gap: 5px;
		flex-shrink: 0;
	}

	.scheduled-card-actions button {
		font-size: 10px;
		padding: 4px 8px;
	}
</style>
