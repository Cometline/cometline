import { describe, expect, it } from 'vitest';
import type { JobResource } from '$lib/client/cometmind';
import { filterArchivedJobs, groupJobsByColumn, sortJobs } from './group-jobs';

function job(overrides: Partial<JobResource> & Pick<JobResource, 'id' | 'status'>): JobResource {
	return {
		description: 'test',
		definition_of_done: '',
		progress: '',
		created_by: 'user',
		failure_count: 0,
		created_at: 1,
		updated_at: 1,
		...overrides
	};
}

describe('sortJobs', () => {
	it('sorts by updated_at asc', () => {
		const jobs = [
			job({ id: 'a', status: 'todo', updated_at: 20 }),
			job({ id: 'b', status: 'todo', updated_at: 10 }),
			job({ id: 'c', status: 'todo', updated_at: 30 })
		];
		expect(sortJobs(jobs).map((j) => j.id)).toEqual(['b', 'a', 'c']);
	});
});

describe('groupJobsByColumn', () => {
	it('groups active jobs by status', () => {
		const jobs = [
			job({ id: 't', status: 'todo' }),
			job({ id: 'b', status: 'blocked' }),
			job({ id: 'o', status: 'ongoing' }),
			job({ id: 'd', status: 'done' }),
			job({ id: 'archived', status: 'done', archived_at: 88 }),
			job({ id: 'del', status: 'todo', deleted_at: 99 })
		];
		const grouped = groupJobsByColumn(jobs);
		expect(grouped.todo.map((j) => j.id)).toEqual(['t', 'b']);
		expect(grouped.ongoing.map((j) => j.id)).toEqual(['o']);
		expect(grouped.done.map((j) => j.id)).toEqual(['d']);
	});
});

describe('filterArchivedJobs', () => {
	it('returns only archived non-deleted jobs', () => {
		const jobs = [
			job({ id: 'a', status: 'todo' }),
			job({ id: 'b', status: 'done', archived_at: 100 }),
			job({ id: 'c', status: 'done', archived_at: 100, deleted_at: 101 })
		];
		expect(filterArchivedJobs(jobs).map((j) => j.id)).toEqual(['b']);
	});
});
