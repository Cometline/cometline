import { describe, expect, it } from 'vitest';
import { formatReadyJobsList, listJobsUserDisplayText } from './format-ready-jobs-list';
import type { JobResource } from '$lib/client/cometmind';

function job(partial: Partial<JobResource> & Pick<JobResource, 'description'>): JobResource {
	return {
		id: 'job-1',
		definition_of_done: '',
		progress: '',
		status: 'todo',
		priority: 0,
		created_at: 0,
		updated_at: 0,
		...partial
	} as JobResource;
}

describe('formatReadyJobsList', () => {
	it('returns empty message', () => {
		expect(formatReadyJobsList([])).toBe('No ready jobs.');
	});

	it('formats jobs without ids', () => {
		const text = formatReadyJobsList([
			job({ description: 'Fix auth', priority: 2 }),
			job({ description: 'Update docs' })
		]);
		expect(text).toContain('(p=2) Fix auth');
		expect(text).toContain('• Update docs');
		expect(text).not.toContain('job-1');
	});
});

describe('listJobsUserDisplayText', () => {
	it('prefixes with /list-jobs', () => {
		const text = listJobsUserDisplayText([job({ description: 'Ship feature' })]);
		expect(text.startsWith('/list-jobs\n\n')).toBe(true);
		expect(text).toContain('Ship feature');
	});
});
