<script lang="ts">
	import { LoaderCircle } from '@lucide/svelte';
	import { page } from '$app/state';
	import { onMount } from 'svelte';
	import {
		createJobsPageController,
		OBSERVER_REFRESH_MS
	} from '$lib/features/jobs/jobs-page.svelte';
	import { createJobsScheduleController } from '$lib/features/jobs/jobs-page-schedule.svelte';
	import { shellStore } from '$lib/stores/shell.svelte';
	import JobDetailDrawer from './JobDetailDrawer.svelte';
	import JobsKanbanBoard from './JobsKanbanBoard.svelte';
	import JobsArchivedPanel from './jobs-page/JobsArchivedPanel.svelte';
	import JobsObserverPanel from './jobs-page/JobsObserverPanel.svelte';
	import JobsPageHeader from './jobs-page/JobsPageHeader.svelte';
	import JobsScheduledPanel from './jobs-page/JobsScheduledPanel.svelte';

	const jobs = createJobsPageController({ getWorkspacePath: () => shellStore.workspacePath });
	const schedule = createJobsScheduleController({
		isScheduledView: () => jobs.view === 'scheduled',
		getWorkspacePath: () => shellStore.workspacePath,
		setError: (message) => (jobs.error = message),
		setSaving: (saving) => (jobs.saving = saving)
	});

	onMount(() => {
		void jobs.loadJobs();
		void schedule.loadScheduledJobs({ silent: true });
		const jobsTimer = setInterval(() => {
			void jobs.loadJobs({ silent: true });
			if (jobs.view === 'scheduled') void schedule.loadScheduledJobs({ silent: true });
		}, OBSERVER_REFRESH_MS);
		const clockTimer = setInterval(() => (jobs.nowMs = Date.now()), 1_000);
		return () => {
			clearInterval(jobsTimer);
			clearInterval(clockTimer);
		};
	});

	$effect(() => {
		if (jobs.view === 'scheduled') {
			void schedule.loadScheduledJobs({ silent: true });
		}
	});

	$effect(() => {
		jobs.openDeepLinkedJob(page.url.searchParams.get('job')?.trim());
	});
</script>

<div class="jobs-page settings-ui">
	<JobsPageHeader
		bind:view={jobs.view}
		bind:statusFilter={jobs.statusFilter}
		loading={jobs.loading}
		refreshing={jobs.refreshing}
		onRefresh={() => {
			void jobs.loadJobs({ silent: true });
			void schedule.loadScheduledJobs({ silent: true });
		}}
	/>

	{#if jobs.error}
		<p class="jobs-error">{jobs.error}</p>
	{/if}

	<div class="jobs-content">
		{#if jobs.loading}
			<div class="jobs-loading">
				<LoaderCircle size={18} class="spin" />
				<span>Loading jobs…</span>
			</div>
		{:else if jobs.view === 'archived'}
			<JobsArchivedPanel
				archivedJobs={jobs.archivedJobs}
				selectedJobId={jobs.selectedJob?.id ?? null}
				onSelectJob={(job) => void jobs.openJob(job)}
			/>
		{:else if jobs.view === 'scheduled'}
			<JobsScheduledPanel {schedule} saving={jobs.saving} nowMs={jobs.nowMs} />
		{:else}
			<JobsObserverPanel
				activeJobs={jobs.activeJobs}
				readyJobCount={jobs.readyJobCount}
				lastLoadedAt={jobs.lastLoadedAt}
				refreshing={jobs.refreshing}
				nowMs={jobs.nowMs}
				selectedJobId={jobs.selectedJob?.id ?? null}
				onSelectJob={(job) => void jobs.openJob(job)}
			/>

			<JobsKanbanBoard
				grouped={jobs.filteredGrouped}
				statusFilter={jobs.statusFilter}
				selectedJobId={jobs.selectedJob?.id ?? null}
				onSelectJob={(job) => void jobs.openJob(job)}
				onAddJob={jobs.openCreate}
			/>
		{/if}
	</div>
</div>

{#if jobs.drawerMode}
	<JobDetailDrawer
		job={jobs.selectedJob}
		mode={jobs.drawerMode}
		events={jobs.events}
		saving={jobs.saving}
		loadingEvents={jobs.loadingEvents}
		bind:editDescription={jobs.editDescription}
		bind:editDod={jobs.editDod}
		bind:editWorkspacePath={jobs.editWorkspacePath}
		bind:createDescription={jobs.createDescription}
		bind:createDod={jobs.createDod}
		bind:createWorkspacePath={jobs.createWorkspacePath}
		onClose={jobs.closeDrawer}
		onSave={jobs.handleSave}
		onDelete={jobs.handleDelete}
		onArchive={jobs.handleArchive}
		onUnarchive={jobs.handleUnarchive}
		onRetry={jobs.handleRetryJob}
		onCreate={jobs.handleCreate}
	/>
{/if}

<style>
	.jobs-page {
		display: flex;
		flex-direction: column;
		box-sizing: border-box;
		height: 100%;
		min-height: 0;
		min-width: 0;
		width: 100%;
		max-width: 100%;
		padding: 20px 24px;
		overflow: hidden;
	}

	.jobs-content {
		flex: 1;
		min-height: 0;
		min-width: 0;
		width: 100%;
		display: flex;
		flex-direction: column;
		gap: 12px;
	}

	.jobs-loading {
		display: inline-flex;
		align-items: center;
		gap: 8px;
		font-size: 13px;
		color: var(--text-muted);
	}

	.jobs-error {
		margin: 0 0 12px;
		padding-left: 14px;
		font-size: 12px;
		color: var(--status-error);
	}

	:global(.spin) {
		animation: spin 1s linear infinite;
	}

	@keyframes spin {
		to {
			transform: rotate(360deg);
		}
	}

	@media (max-width: 900px) {
		.jobs-page {
			padding: 16px;
		}
	}

	@container main-pane (max-width: 760px) {
		.jobs-page {
			padding: 16px;
		}
	}
</style>
