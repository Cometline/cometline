import type { JobResource } from '$lib/client/cometmind';

export type JobColumn = 'todo' | 'ongoing' | 'done';

export type GroupedJobs = Record<JobColumn, JobResource[]>;

export function sortJobs(jobs: JobResource[]): JobResource[] {
	return [...jobs].sort((a, b) => (a.updated_at ?? 0) - (b.updated_at ?? 0));
}

export function groupJobsByColumn(jobs: JobResource[]): GroupedJobs {
	const active = jobs.filter((job) => !job.deleted_at && !job.archived_at);
	return {
		todo: sortJobs(active.filter((job) => job.status === 'todo' || job.status === 'blocked')),
		ongoing: sortJobs(active.filter((job) => job.status === 'ongoing')),
		done: sortJobs(active.filter((job) => job.status === 'done'))
	};
}

export function filterGroupedByStatus(
	grouped: GroupedJobs,
	status: 'all' | JobColumn
): GroupedJobs {
	if (status === 'all') return grouped;
	return {
		todo: status === 'todo' ? grouped.todo : [],
		ongoing: status === 'ongoing' ? grouped.ongoing : [],
		done: status === 'done' ? grouped.done : []
	};
}

export function filterArchivedJobs(jobs: JobResource[]): JobResource[] {
	return sortJobs(jobs.filter((job) => job.archived_at != null && job.deleted_at == null));
}

export function truncateWorkspacePath(path: string, maxLength = 28): string {
	const normalized = path.replace(/\\/g, '/');
	if (normalized.length <= maxLength) return normalized;
	const parts = normalized.split('/').filter(Boolean);
	const name = parts[parts.length - 1] ?? normalized;
	if (name.length <= maxLength) return `…/${name}`;
	return `…${name.slice(-(maxLength - 1))}`;
}
