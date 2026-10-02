import {
	archiveJob,
	createJob,
	deleteJob,
	getJob,
	listJobEvents,
	listJobs,
	updateJob,
	unblockJob,
	unarchiveJob,
	type JobEventResource,
	type JobResource
} from '#lib/client/cometmind.js';
import {
	filterArchivedJobs,
	filterGroupedByStatus,
	groupJobsByColumn,
	type GroupedJobs,
	type JobColumn
} from '#lib/features/jobs/group-jobs.js';
import { truncateJobLabel } from '#lib/features/jobs/format-job-label.js';
import { jobsIndicatorStore } from '#lib/stores/jobs-indicator.svelte.js';

export type JobsDrawerMode = 'detail' | 'create' | null;
export type JobsStatusFilter = 'all' | JobColumn;
export type JobsView = 'active' | 'archived' | 'scheduled';

export const OBSERVER_REFRESH_MS = 5_000;

export function createJobsPageController(deps: { getWorkspacePath: () => string | undefined }) {
	let grouped = $state<GroupedJobs>({ todo: [], ongoing: [], done: [] });
	let archivedJobs = $state<JobResource[]>([]);
	let statusFilter = $state<JobsStatusFilter>('all');
	const filteredGrouped = $derived(filterGroupedByStatus(grouped, statusFilter));
	const activeJobs = $derived(grouped.ongoing);
	const readyJobCount = $derived(grouped.todo.length);
	let loading = $state(true);
	let refreshing = $state(false);
	let error = $state('');
	let lastLoadedAt = $state(0);
	let nowMs = $state(Date.now());
	let view = $state<JobsView>('active');
	let drawerMode = $state<JobsDrawerMode>(null);
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
		grouped = groupJobsByColumn(next);
		archivedJobs = filterArchivedJobs(next);
		jobsIndicatorStore.setOngoingCount(grouped.ongoing.length);
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
		createWorkspacePath = deps.getWorkspacePath()?.trim() ?? '';
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

	function openDeepLinkedJob(jobId: string | undefined) {
		if (!jobId || loading) return;
		if (selectedJob?.id === jobId && drawerMode === 'detail') return;
		const fromList =
			[...grouped.todo, ...grouped.ongoing, ...grouped.done, ...archivedJobs].find(
				(job) => job.id === jobId
			) ?? null;
		if (fromList) {
			void openJob(fromList);
			return;
		}
		void getJob(jobId)
			.then((job) => openJob(job))
			.catch(() => {
				/* ignore missing deep-link targets */
			});
	}

	return {
		get archivedJobs() {
			return archivedJobs;
		},
		get statusFilter() {
			return statusFilter;
		},
		set statusFilter(value: JobsStatusFilter) {
			statusFilter = value;
		},
		get filteredGrouped() {
			return filteredGrouped;
		},
		get activeJobs() {
			return activeJobs;
		},
		get readyJobCount() {
			return readyJobCount;
		},
		get loading() {
			return loading;
		},
		get refreshing() {
			return refreshing;
		},
		get error() {
			return error;
		},
		set error(value: string) {
			error = value;
		},
		get lastLoadedAt() {
			return lastLoadedAt;
		},
		get nowMs() {
			return nowMs;
		},
		set nowMs(value: number) {
			nowMs = value;
		},
		get view() {
			return view;
		},
		set view(value: JobsView) {
			view = value;
		},
		get drawerMode() {
			return drawerMode;
		},
		get selectedJob() {
			return selectedJob;
		},
		get events() {
			return events;
		},
		get loadingEvents() {
			return loadingEvents;
		},
		get saving() {
			return saving;
		},
		set saving(value: boolean) {
			saving = value;
		},
		get editDescription() {
			return editDescription;
		},
		set editDescription(value: string) {
			editDescription = value;
		},
		get editDod() {
			return editDod;
		},
		set editDod(value: string) {
			editDod = value;
		},
		get editWorkspacePath() {
			return editWorkspacePath;
		},
		set editWorkspacePath(value: string) {
			editWorkspacePath = value;
		},
		get createDescription() {
			return createDescription;
		},
		set createDescription(value: string) {
			createDescription = value;
		},
		get createDod() {
			return createDod;
		},
		set createDod(value: string) {
			createDod = value;
		},
		get createWorkspacePath() {
			return createWorkspacePath;
		},
		set createWorkspacePath(value: string) {
			createWorkspacePath = value;
		},
		loadJobs,
		closeDrawer,
		openJob,
		openCreate,
		handleCreate,
		handleSave,
		handleDelete,
		handleArchive,
		handleUnarchive,
		handleRetryJob,
		openDeepLinkedJob
	};
}

export type JobsPageController = ReturnType<typeof createJobsPageController>;
