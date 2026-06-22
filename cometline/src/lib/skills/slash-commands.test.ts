import { describe, expect, it } from 'vitest';
import { filterJobOptions, parseJobCommand, parseListJobsCommand } from './slash-commands';

describe('parseListJobsCommand', () => {
	it('matches exact /list-jobs', () => {
		expect(parseListJobsCommand('/list-jobs')).toBe(true);
		expect(parseListJobsCommand('/list-jobs ')).toBe(true);
		expect(parseListJobsCommand('/job')).toBe(false);
	});
});

describe('parseJobCommand', () => {
	it('parses /job with optional query', () => {
		expect(parseJobCommand('/job')).toEqual({ query: '' });
		expect(parseJobCommand('/job auth')).toEqual({ query: 'auth' });
	});
});

describe('filterJobOptions', () => {
	const jobs = [
		{ id: '01JOBAUTH', description: 'Fix auth module', priority: 5 },
		{ id: '01JOBDOC', description: 'Write docs', priority: 1 }
	];

	it('filters by description and id', () => {
		expect(filterJobOptions('auth', jobs)).toHaveLength(1);
		expect(filterJobOptions('01JOB', jobs)).toHaveLength(2);
	});
});
