<script lang="ts">
	import { CalendarClock, LoaderCircle, Plus, RefreshCw, Trash2 } from '@lucide/svelte';
	import { onMount } from 'svelte';
	import {
		archiveJob,
		createJob,
		createScheduledJob,
		deleteJob,
		deleteScheduledJob,
		listJobEvents,
		listJobs,
		listScheduledJobs,
		updateJob,
		updateScheduledJob,
		unblockJob,
		unarchiveJob,
		type JobEventResource,
		type JobResource,
		type ScheduledJobResource
	} from '$lib/client/cometmind';
	import {
		filterArchivedJobs,
		filterGroupedByStatus,
		groupJobsByColumn,
		type GroupedJobs,
		type JobColumn
	} from '$lib/jobs/group-jobs';
	import { truncateJobLabel } from '$lib/jobs/format-job-label';
	import { shellStore } from '$lib/stores/shell.svelte';
	import JobCard from './JobCard.svelte';
	import JobDetailDrawer from './JobDetailDrawer.svelte';
	import JobsKanbanBoard from './JobsKanbanBoard.svelte';

	type DrawerMode = 'detail' | 'create' | null;
	type StatusFilter = 'all' | JobColumn;
	type View = 'active' | 'archived' | 'scheduled';

	const STATUS_FILTERS: { id: StatusFilter; label: string }[] = [
		{ id: 'all', label: 'All' },
		{ id: 'todo', label: 'Todo' },
		{ id: 'ongoing', label: 'Ongoing' },
		{ id: 'done', label: 'Done' }
	];
	const OBSERVER_REFRESH_MS = 5_000;

	let grouped = $state<GroupedJobs>({ todo: [], ongoing: [], done: [] });
	let archivedJobs = $state<JobResource[]>([]);
	let scheduledJobs = $state<ScheduledJobResource[]>([]);
	let statusFilter = $state<StatusFilter>('all');
	let filteredGrouped = $derived(filterGroupedByStatus(grouped, statusFilter));
	let activeJobs = $derived(grouped.ongoing);
	let readyJobCount = $derived(grouped.todo.length);
	let loading = $state(true);
	let refreshing = $state(false);
	let error = $state('');
	let lastLoadedAt = $state(0);
	let nowMs = $state(Date.now());
	let view = $state<View>('active');
	let drawerMode = $state<DrawerMode>(null);
	let selectedJob = $state<JobResource | null>(null);
	let events = $state<JobEventResource[]>([]);
	let loadingEvents = $state(false);
	let saving = $state(false);

	let editDescription = $state('');
	let editDod = $state('');
	let editWorkspacePath = $state('');

	let createDescription = $state('');
	let createDod = $state('');
	let createWorkspacePath = $state('');

	let showScheduleForm = $state(false);
	let scheduleDescription = $state('');
	let scheduleDod = $state('');
	let scheduleWorkspacePath = $state('');
	let scheduleCronExpr = $state('');
	let scheduleRunAtLocal = $state('');

	function applyJobs(next: JobResource[]) {
		grouped = groupJobsByColumn(next);
		archivedJobs = filterArchivedJobs(next);
		lastLoadedAt = Date.now();
	}

	async function loadEventsForJob(jobId: string, options: { silent?: boolean } = {}) {
		if (!options.silent) loadingEvents = true;
		try {
			const res = await listJobEvents(jobId);
			events = res.events ?? [];
		} catch {
			events = [];
		} finally {
			if (!options.silent) loadingEvents = false;
		}
	}

	async function loadJobs(options: { silent?: boolean } = {}) {
		if (options.silent && (loading || refreshing)) return;
		if (!options.silent) loading = true;
		else refreshing = true;
		error = '';
		try {
			const res = await listJobs({ include_deleted: true, include_archived: true });
			applyJobs(res.jobs ?? []);
			if (selectedJob) {
				const refreshed =
					(res.jobs ?? []).find((job) => job.id === selectedJob?.id) ?? null;
				selectedJob = refreshed;
				if (refreshed && drawerMode === 'detail') {
					void loadEventsForJob(refreshed.id, { silent: true });
				}
			}
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load jobs';
		} finally {
			loading = false;
			refreshing = false;
		}
	}

	function resetCreateForm() {
		createDescription = '';
		createDod = '';
		createWorkspacePath = '';
	}

	function closeDrawer() {
		drawerMode = null;
		selectedJob = null;
		events = [];
		resetCreateForm();
	}

	async function openJob(job: JobResource) {
		selectedJob = job;
		drawerMode = 'detail';
		editDescription = job.description;
		editDod = job.definition_of_done ?? '';
		editWorkspacePath = job.workspace_path ?? '';
		await loadEventsForJob(job.id);
	}

	function openCreate() {
		selectedJob = null;
		events = [];
		resetCreateForm();
		createWorkspacePath = shellStore.workspacePath?.trim() ?? '';
		drawerMode = 'create';
	}

	async function handleCreate() {
		if (!createDescription.trim()) return;
		saving = true;
		error = '';
		try {
			const created = await createJob({
				description: createDescription.trim(),
				definition_of_done: createDod.trim(),
				workspace_path: createWorkspacePath.trim() || undefined,
				created_by: 'user',
				source_platform: 'desktop'
			});
			await loadJobs({ silent: true });
			resetCreateForm();
			await openJob(created);
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to create job';
		} finally {
			saving = false;
		}
	}

	async function handleSave() {
		if (!selectedJob || selectedJob.status !== 'todo') return;
		saving = true;
		error = '';
		try {
			const updated = await updateJob(selectedJob.id, {
				description: editDescription.trim(),
				definition_of_done: editDod.trim(),
				workspace_path: editWorkspacePath.trim() || undefined
			});
			selectedJob = updated;
			await loadJobs({ silent: true });
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to update job';
		} finally {
			saving = false;
		}
	}

	async function handleDelete(job: JobResource) {
		if (!confirm(`Delete "${truncateJobLabel(job.description)}"?`)) return;
		try {
			await deleteJob(job.id);
			closeDrawer();
			await loadJobs({ silent: true });
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to delete job';
		}
	}

	async function handleArchive(job: JobResource) {
		if (!confirm(`Archive "${truncateJobLabel(job.description)}"?`)) return;
		saving = true;
		error = '';
		try {
			selectedJob = await archiveJob(job.id);
			await loadJobs({ silent: true });
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to archive job';
		} finally {
			saving = false;
		}
	}

	async function handleUnarchive(job: JobResource) {
		saving = true;
		error = '';
		try {
			selectedJob = await unarchiveJob(job.id);
			await loadJobs({ silent: true });
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to unarchive job';
		} finally {
			saving = false;
		}
	}

	async function handleRetryJob(job: JobResource) {
		saving = true;
		error = '';
		try {
			selectedJob = await unblockJob(job.id);
			await loadJobs({ silent: true });
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to retry job';
		} finally {
			saving = false;
		}
	}

	async function loadScheduledJobs(options: { silent?: boolean } = {}) {
		try {
			const res = await listScheduledJobs();
			scheduledJobs = res.scheduled_jobs ?? [];
		} catch (e) {
			if (!options.silent) scheduledJobs = [];
			if (!options.silent || view === 'scheduled') {
				error = e instanceof Error ? e.message : 'Failed to load scheduled jobs';
			}
		}
	}

	function resetScheduleForm() {
		scheduleDescription = '';
		scheduleDod = '';
		scheduleWorkspacePath = '';
		scheduleCronExpr = '';
		scheduleRunAtLocal = '';
	}

	function localDatetimeToMillis(local: string): number | undefined {
		if (!local) return undefined;
		const ms = new Date(local).getTime();
		return Number.isNaN(ms) ? undefined : ms;
	}

	async function handleCreateScheduled() {
		if (!scheduleDescription.trim()) return;
		const cron = scheduleCronExpr.trim();
		const runAt = localDatetimeToMillis(scheduleRunAtLocal);
		if (!cron && !runAt) {
			error = 'Provide either a cron expression or a run time.';
			return;
		}
		saving = true;
		error = '';
		try {
			await createScheduledJob({
				description: scheduleDescription.trim(),
				definition_of_done: scheduleDod.trim() || undefined,
				workspace_path: scheduleWorkspacePath.trim() || undefined,
				cron_expr: cron || undefined,
				run_at: runAt,
				created_by: 'user',
				source_platform: 'desktop'
			});
			resetScheduleForm();
			showScheduleForm = false;
			await loadScheduledJobs({ silent: true });
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to create scheduled job';
		} finally {
			saving = false;
		}
	}

	async function handleDeleteScheduled(job: ScheduledJobResource) {
		if (!confirm(`Delete scheduled job "${truncateJobLabel(job.description)}"?`)) return;
		try {
			await deleteScheduledJob(job.id);
			await loadScheduledJobs({ silent: true });
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to delete scheduled job';
		}
	}

	async function handleToggleScheduled(job: ScheduledJobResource) {
		try {
			await updateScheduledJob(job.id, { enabled: !job.enabled });
			await loadScheduledJobs({ silent: true });
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to toggle scheduled job';
		}
	}

	onMount(() => {
		void loadJobs();
		void loadScheduledJobs({ silent: true });
		const jobsTimer = setInterval(() => {
			void loadJobs({ silent: true });
			if (view === 'scheduled') void loadScheduledJobs({ silent: true });
		}, OBSERVER_REFRESH_MS);
		const clockTimer = setInterval(() => (nowMs = Date.now()), 1_000);
		return () => {
			clearInterval(jobsTimer);
			clearInterval(clockTimer);
		};
	});

	$effect(() => {
		if (view === 'scheduled') {
			void loadScheduledJobs({ silent: true });
		}
	});

	function formatClock(ms: number): string {
		if (!ms) return 'Never';
		return new Intl.DateTimeFormat(undefined, {
			hour: '2-digit',
			minute: '2-digit',
			second: '2-digit'
		}).format(new Date(ms));
	}

	function formatRelativeTime(ms?: number): string {
		if (!ms) return 'unknown';
		const diff = ms - nowMs;
		const abs = Math.abs(diff);
		if (abs < 5_000) return diff >= 0 ? 'now' : 'just now';
		const units: [number, string][] = [
			[86_400_000, 'd'],
			[3_600_000, 'h'],
			[60_000, 'm'],
			[1_000, 's']
		];
		const [unitMs, label] = units.find(([size]) => abs >= size) ?? units[units.length - 1];
		const value = Math.floor(abs / unitMs);
		return `${value}${label} ${diff >= 0 ? 'left' : 'ago'}`;
	}

	function leaseLabel(job: JobResource): string {
		if (!job.lease_expires_at) return 'No active lease expiry';
		if (job.lease_expires_at <= nowMs) return 'Lease expired';
		return `Lease ${formatRelativeTime(job.lease_expires_at)}`;
	}

	function progressPreview(job: JobResource): string {
		return job.progress?.trim().split('\n')[0] ?? '';
	}

	function sessionLabel(job: JobResource): string {
		return job.assigned_session_id ? job.assigned_session_id.slice(0, 8) : 'unassigned';
	}

	function scheduleLabel(job: ScheduledJobResource): string {
		if (job.cron_expr) return `cron: ${job.cron_expr}`;
		if (job.run_at) return `one-shot: ${formatClock(job.run_at)}`;
		return 'unscheduled';
	}

	function nextRunLabel(job: ScheduledJobResource): string {
		if (!job.enabled) return 'disabled';
		return formatRelativeTime(job.next_run_at);
	}
</script>

<div class="jobs-page settings-ui">
	<header class="jobs-header">
		<div>
			<h1>Jobs</h1>
			<p>Global work queue shared across sessions.</p>
		</div>
		<div class="jobs-header-actions">
			{#if view === 'active'}
				<div class="status-filters" role="group" aria-label="Filter by status">
					{#each STATUS_FILTERS as filter (filter.id)}
						<button
							type="button"
							class="status-filter"
							class:active={statusFilter === filter.id}
							aria-pressed={statusFilter === filter.id}
							onclick={() => (statusFilter = filter.id)}
						>
							{filter.label}
						</button>
					{/each}
				</div>
			{/if}
			<div class="view-toggle" role="group" aria-label="Switch view">
				<button
					type="button"
					class="view-btn"
					class:active={view === 'active'}
					aria-pressed={view === 'active'}
					onclick={() => (view = 'active')}
				>
					Active
				</button>
				<button
					type="button"
					class="view-btn"
					class:active={view === 'archived'}
					aria-pressed={view === 'archived'}
					onclick={() => (view = 'archived')}
				>
					Archived
				</button>
				<button
					type="button"
					class="view-btn"
					class:active={view === 'scheduled'}
					aria-pressed={view === 'scheduled'}
					onclick={() => (view = 'scheduled')}
				>
					Scheduled
				</button>
			</div>
			<button
				type="button"
				class="secondary icon-only"
				aria-label="Refresh jobs"
				title="Refresh"
				disabled={loading || refreshing}
				onclick={() => {
					void loadJobs({ silent: true });
					void loadScheduledJobs({ silent: true });
				}}
			>
				<RefreshCw size={14} class={refreshing ? 'spin' : ''} />
			</button>
		</div>
	</header>

	{#if error}
		<p class="jobs-error">{error}</p>
	{/if}

	<div class="jobs-content">
		{#if loading}
			<div class="jobs-loading">
				<LoaderCircle size={18} class="spin" />
				<span>Loading jobs…</span>
			</div>
		{:else if view === 'archived'}
			<section class="archived-panel settings-panel-frame">
				<header class="archived-header">
					<h2>Archived</h2>
					<span class="archived-count">{archivedJobs.length}</span>
				</header>
				{#if archivedJobs.length === 0}
					<p class="jobs-muted">No archived jobs.</p>
				{:else}
					<div class="archived-list scrollbar-none">
						{#each archivedJobs as job (job.id)}
							<JobCard
								{job}
								selected={selectedJob?.id === job.id}
								onclick={() => void openJob(job)}
							/>
						{/each}
					</div>
				{/if}
			</section>
		{:else if view === 'scheduled'}
			<section class="scheduled-panel settings-panel-frame">
				<header class="scheduled-header">
					<h2>Scheduled</h2>
					<button
						type="button"
						class="secondary"
						onclick={() => {
							showScheduleForm = !showScheduleForm;
							if (showScheduleForm) {
								scheduleWorkspacePath = shellStore.workspacePath?.trim() ?? '';
							}
						}}
					>
						<Plus size={14} />
						{showScheduleForm ? 'Cancel' : 'Schedule job'}
					</button>
				</header>

				{#if showScheduleForm}
					<form
						class="schedule-form"
						onsubmit={(e) => {
							e.preventDefault();
							void handleCreateScheduled();
						}}
					>
						<label class="form-field">
							<span>Description</span>
							<input
								type="text"
								bind:value={scheduleDescription}
								placeholder="What should this job do?"
								required
							/>
						</label>
						<label class="form-field">
							<span>Definition of done</span>
							<textarea bind:value={scheduleDod} rows="2"></textarea>
						</label>
						<label class="form-field">
							<span>Workspace path</span>
							<input type="text" bind:value={scheduleWorkspacePath} />
						</label>
						<div class="schedule-mode">
							<label class="form-field">
								<span>Cron expression (recurring)</span>
								<input
									type="text"
									bind:value={scheduleCronExpr}
									placeholder="0 9 * * 1"
									disabled={!!scheduleRunAtLocal}
								/>
								<small
									>5-field cron, e.g. <code>0 9 * * 1</code> for every Monday 9am</small
								>
							</label>
							<label class="form-field">
								<span>Run at (one-shot)</span>
								<input
									type="datetime-local"
									bind:value={scheduleRunAtLocal}
									disabled={!!scheduleCronExpr}
								/>
								<small>Local time. Leave empty if using cron.</small>
							</label>
						</div>
						<button type="submit" class="primary" disabled={saving}>
							{#if saving}
								<LoaderCircle size={14} class="spin" />
							{/if}
							Create schedule
						</button>
					</form>
				{/if}

				{#if scheduledJobs.length === 0}
					<p class="jobs-muted">No scheduled jobs. Create one to defer work.</p>
				{:else}
					<div class="scheduled-list scrollbar-none">
						{#each scheduledJobs as job (job.id)}
							<div class="scheduled-card" class:disabled={!job.enabled}>
								<div class="scheduled-card-main">
									<strong>{job.description}</strong>
									<div class="scheduled-card-meta">
										<span class="chip">
											<CalendarClock size={11} />
											{scheduleLabel(job)}
										</span>
										<span class="chip">Next: {nextRunLabel(job)}</span>
										{#if job.last_run_at}
											<span class="chip"
												>Last: {formatClock(job.last_run_at)}</span
											>
										{/if}
									</div>
								</div>
								<div class="scheduled-card-actions">
									<button
										type="button"
										class="secondary icon-only"
										title={job.enabled ? 'Disable' : 'Enable'}
										onclick={() => void handleToggleScheduled(job)}
									>
										{job.enabled ? 'Disable' : 'Enable'}
									</button>
									<button
										type="button"
										class="danger icon-only"
										title="Delete"
										onclick={() => void handleDeleteScheduled(job)}
									>
										<Trash2 size={14} />
									</button>
								</div>
							</div>
						{/each}
					</div>
				{/if}
			</section>
		{:else}
			<section class="autonomy-observer settings-panel-frame" aria-label="Live job activity">
				<header class="observer-header">
					<div>
						<p class="observer-eyebrow">Live activity</p>
						<h2>Autonomous job observation</h2>
					</div>
					<div class="observer-status">
						<span>{activeJobs.length} running</span>
						<span>{readyJobCount} ready</span>
						<span>Updated {formatClock(lastLoadedAt)}</span>
						{#if refreshing}
							<span class="observer-polling">Refreshing…</span>
						{/if}
					</div>
				</header>

				{#if activeJobs.length === 0}
					<p class="observer-empty">
						No jobs are running. When autonomous pickup or a chat session claims a ready
						job, it will appear here within {Math.round(OBSERVER_REFRESH_MS / 1_000)} seconds.
					</p>
				{:else}
					<div class="observer-list">
						{#each activeJobs as job (job.id)}
							<button
								type="button"
								class="observer-job"
								class:selected={selectedJob?.id === job.id}
								onclick={() => void openJob(job)}
							>
								<div class="observer-job-main">
									<span class="observer-dot" aria-hidden="true"></span>
									<div>
										<strong>{job.description}</strong>
										<p>{progressPreview(job) || 'No progress note yet.'}</p>
									</div>
								</div>
								<div class="observer-job-meta">
									<span>Session {sessionLabel(job)}</span>
									<span>{leaseLabel(job)}</span>
									<span>Updated {formatRelativeTime(job.updated_at)}</span>
								</div>
							</button>
						{/each}
					</div>
				{/if}
			</section>

			<JobsKanbanBoard
				grouped={filteredGrouped}
				{statusFilter}
				selectedJobId={selectedJob?.id ?? null}
				onSelectJob={(job) => void openJob(job)}
				onAddJob={openCreate}
			/>
		{/if}
	</div>
</div>

{#if drawerMode}
	<JobDetailDrawer
		job={selectedJob}
		mode={drawerMode}
		{events}
		{saving}
		{loadingEvents}
		bind:editDescription
		bind:editDod
		bind:editWorkspacePath
		bind:createDescription
		bind:createDod
		bind:createWorkspacePath
		onClose={closeDrawer}
		onSave={handleSave}
		onDelete={handleDelete}
		onArchive={handleArchive}
		onUnarchive={handleUnarchive}
		onRetry={handleRetryJob}
		onCreate={handleCreate}
	/>
{/if}

<style>
	.jobs-page {
		display: flex;
		flex-direction: column;
		height: 100%;
		min-height: 0;
		padding: 20px 24px;
		overflow: hidden;
	}

	.jobs-header {
		display: flex;
		align-items: flex-start;
		justify-content: space-between;
		gap: 16px;
		flex-shrink: 0;
		margin-bottom: 16px;
	}

	.jobs-header h1 {
		margin: 0 0 4px;
		font-size: 20px;
		font-weight: 650;
		color: var(--text-main);
	}

	.jobs-header p {
		margin: 0;
		font-size: 12px;
		color: var(--text-muted);
	}

	.jobs-header-actions {
		display: flex;
		align-items: center;
		gap: 8px;
		flex-shrink: 0;
		flex-wrap: wrap;
		justify-content: flex-end;
	}

	.status-filters {
		display: inline-flex;
		align-items: center;
		gap: 4px;
		padding: 3px;
		border-radius: 999px;
		background: rgba(15, 23, 42, 0.05);
	}

	.status-filter {
		border: none;
		background: transparent;
		color: var(--text-muted);
		font: inherit;
		font-size: 11px;
		font-weight: 600;
		padding: 5px 10px;
		border-radius: 999px;
		cursor: pointer;
	}

	.status-filter.active {
		background: var(--panel-bg);
		color: var(--text-main);
		box-shadow: 0 1px 2px rgba(15, 23, 42, 0.08);
	}

	.status-filter:hover:not(.active) {
		color: var(--text-main);
	}

	.view-toggle {
		display: inline-flex;
		align-items: center;
		gap: 4px;
		padding: 3px;
		border-radius: 999px;
		background: rgba(15, 23, 42, 0.05);
	}

	.view-btn {
		border: none;
		background: transparent;
		color: var(--text-muted);
		font: inherit;
		font-size: 11px;
		font-weight: 600;
		padding: 5px 10px;
		border-radius: 999px;
		cursor: pointer;
	}

	.view-btn.active {
		background: var(--panel-bg);
		color: var(--text-main);
		box-shadow: 0 1px 2px rgba(15, 23, 42, 0.08);
	}

	.view-btn:hover:not(.active) {
		color: var(--text-main);
	}

	.jobs-content {
		flex: 1;
		min-height: 0;
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
		font-size: 12px;
		color: var(--status-error);
	}

	.jobs-muted {
		margin: 0;
		font-size: 12px;
		color: var(--text-muted);
	}

	.archived-panel {
		display: flex;
		flex-direction: column;
		gap: 12px;
		min-height: 0;
		height: 100%;
		padding: 14px;
		overflow: hidden;
	}

	.archived-header {
		display: flex;
		align-items: center;
		gap: 8px;
	}

	.archived-header h2 {
		margin: 0;
		font-size: 14px;
		font-weight: 650;
	}

	.archived-count {
		font-size: 11px;
		font-weight: 600;
		padding: 2px 7px;
		border-radius: 999px;
		background: rgba(15, 23, 42, 0.06);
		color: var(--text-muted);
	}

	.archived-list {
		display: grid;
		grid-template-columns: repeat(auto-fill, minmax(260px, 1fr));
		gap: 10px;
		overflow-y: auto;
		min-height: 0;
		padding-right: 2px;
	}

	.autonomy-observer {
		display: flex;
		flex-direction: column;
		gap: 12px;
		padding: 14px;
		flex-shrink: 0;
		background:
			linear-gradient(135deg, rgba(96, 165, 250, 0.1), rgba(168, 85, 247, 0.08)),
			var(--panel-bg);
	}

	.observer-header {
		display: flex;
		align-items: flex-start;
		justify-content: space-between;
		gap: 12px;
	}

	.observer-eyebrow {
		margin: 0 0 3px;
		font-size: 10px;
		font-weight: 700;
		letter-spacing: 0.08em;
		text-transform: uppercase;
		color: var(--text-muted);
	}

	.observer-header h2 {
		margin: 0;
		font-size: 14px;
		font-weight: 650;
		color: var(--text-main);
	}

	.observer-status,
	.observer-job-meta {
		display: flex;
		align-items: center;
		flex-wrap: wrap;
		gap: 6px;
	}

	.observer-status span,
	.observer-job-meta span {
		font-size: 10px;
		font-weight: 600;
		line-height: 1.3;
		padding: 3px 7px;
		border-radius: 999px;
		background: rgba(15, 23, 42, 0.06);
		color: var(--text-muted);
	}

	.observer-status .observer-polling {
		color: var(--accent);
		background: color-mix(in srgb, var(--accent) 12%, transparent);
	}

	.observer-empty {
		margin: 0;
		max-width: 760px;
		font-size: 12px;
		line-height: 1.5;
		color: var(--text-muted);
	}

	.observer-list {
		display: grid;
		grid-template-columns: repeat(auto-fit, minmax(260px, 1fr));
		gap: 10px;
	}

	.observer-job {
		width: 100%;
		border: 1px solid color-mix(in srgb, var(--accent) 18%, var(--border-soft));
		border-radius: 12px;
		background: rgba(255, 255, 255, 0.78);
		padding: 10px 12px;
		text-align: left;
		cursor: pointer;
		display: flex;
		flex-direction: column;
		gap: 9px;
		transition:
			background var(--duration-fast) var(--ease-smooth),
			border-color var(--duration-fast) var(--ease-smooth),
			box-shadow var(--duration-fast) var(--ease-smooth);
	}

	.observer-job:hover,
	.observer-job.selected {
		background: rgba(255, 255, 255, 0.96);
		border-color: var(--pane-focus-border);
		box-shadow: 0 8px 24px rgba(15, 23, 42, 0.08);
	}

	.observer-job-main {
		display: grid;
		grid-template-columns: auto minmax(0, 1fr);
		gap: 8px;
		align-items: flex-start;
	}

	.observer-dot {
		width: 8px;
		height: 8px;
		margin-top: 5px;
		border-radius: 999px;
		background: var(--accent);
		box-shadow: 0 0 0 5px color-mix(in srgb, var(--accent) 14%, transparent);
	}

	.observer-job strong {
		display: block;
		font-size: 12px;
		line-height: 1.35;
		color: var(--text-main);
		display: -webkit-box;
		-webkit-line-clamp: 2;
		line-clamp: 2;
		-webkit-box-orient: vertical;
		overflow: hidden;
	}

	.observer-job p {
		margin: 3px 0 0;
		font-size: 11px;
		line-height: 1.4;
		color: var(--text-muted);
		display: -webkit-box;
		-webkit-line-clamp: 2;
		line-clamp: 2;
		-webkit-box-orient: vertical;
		overflow: hidden;
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

		.jobs-header {
			flex-direction: column;
		}

		.observer-header {
			flex-direction: column;
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

	.scheduled-header h2 {
		margin: 0;
		font-size: 14px;
		font-weight: 650;
	}

	.schedule-form {
		display: flex;
		flex-direction: column;
		gap: 10px;
		padding: 12px;
		border-radius: 10px;
		background: var(--panel-bg);
		border: 1px solid var(--border-soft);
	}

	.form-field {
		display: flex;
		flex-direction: column;
		gap: 4px;
		font-size: 12px;
		color: var(--text-muted);
	}

	.form-field input,
	.form-field textarea {
		font: inherit;
		font-size: 13px;
		color: var(--text-main);
		padding: 7px 9px;
		border: 1px solid var(--border-soft);
		border-radius: 7px;
		background: var(--panel-bg);
		resize: vertical;
	}

	.form-field small {
		font-size: 10px;
		color: var(--text-muted);
	}

	.form-field code {
		font-size: 10px;
		padding: 1px 4px;
		border-radius: 4px;
		background: rgba(15, 23, 42, 0.06);
	}

	.schedule-mode {
		display: grid;
		grid-template-columns: 1fr 1fr;
		gap: 10px;
	}

	.scheduled-list {
		display: grid;
		grid-template-columns: repeat(auto-fill, minmax(300px, 1fr));
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
		flex-direction: column;
		gap: 6px;
		min-width: 0;
	}

	.scheduled-card-main strong {
		font-size: 12px;
		color: var(--text-main);
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
