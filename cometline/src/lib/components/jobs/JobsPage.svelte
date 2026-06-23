<script lang="ts">
	import { LoaderCircle, RefreshCw } from '@lucide/svelte';
	import { onMount } from 'svelte';
	import {
		createJob,
		deleteJob,
		listJobEvents,
		listJobs,
		updateJob,
		type JobEventResource,
		type JobResource
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

	const STATUS_FILTERS: { id: StatusFilter; label: string }[] = [
		{ id: 'all', label: 'All' },
		{ id: 'todo', label: 'Todo' },
		{ id: 'ongoing', label: 'Ongoing' },
		{ id: 'done', label: 'Done' }
	];

	let jobs = $state<JobResource[]>([]);
	let grouped = $state<GroupedJobs>({ todo: [], ongoing: [], done: [] });
	let archivedJobs = $state<JobResource[]>([]);
	let statusFilter = $state<StatusFilter>('all');
	let filteredGrouped = $derived(filterGroupedByStatus(grouped, statusFilter));
	let loading = $state(true);
	let refreshing = $state(false);
	let error = $state('');
	let showArchived = $state(false);
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

	function applyJobs(next: JobResource[]) {
		jobs = next;
		grouped = groupJobsByColumn(next);
		archivedJobs = filterArchivedJobs(next);
	}

	async function loadJobs(options: { silent?: boolean } = {}) {
		if (!options.silent) loading = true;
		else refreshing = true;
		error = '';
		try {
			const res = await listJobs({ include_deleted: true });
			applyJobs(res.jobs ?? []);
			if (selectedJob) {
				const refreshed = (res.jobs ?? []).find((job) => job.id === selectedJob?.id) ?? null;
				selectedJob = refreshed;
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
		loadingEvents = true;
		try {
			const res = await listJobEvents(job.id);
			events = res.events ?? [];
		} catch {
			events = [];
		} finally {
			loadingEvents = false;
		}
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

	onMount(() => {
		void loadJobs();
	});
</script>

<div class="jobs-page settings-ui">
	<header class="jobs-header">
		<div>
			<h1>Jobs</h1>
			<p>Global work queue shared across sessions.</p>
		</div>
		<div class="jobs-header-actions">
			{#if !showArchived}
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
			<label class="archived-toggle">
				<input type="checkbox" bind:checked={showArchived} />
				<span>Show archived</span>
			</label>
			<button
				type="button"
				class="secondary icon-only"
				aria-label="Refresh jobs"
				title="Refresh"
				disabled={loading || refreshing}
				onclick={() => void loadJobs({ silent: true })}
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
		{:else if showArchived}
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
		{:else}
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

	.archived-toggle {
		display: inline-flex;
		align-items: center;
		gap: 8px;
		font-size: 12px;
		color: var(--text-muted);
		user-select: none;
	}

	.jobs-content {
		flex: 1;
		min-height: 0;
		display: flex;
		flex-direction: column;
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
		color: #b42318;
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
	}
</style>
