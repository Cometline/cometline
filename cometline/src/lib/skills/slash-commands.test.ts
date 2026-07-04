import { describe, expect, it } from 'vitest';
import {
	expandCreateSkillCommand,
	filterJobOptions,
	parseJobCommand,
	parseListJobsCommand
} from './slash-commands';

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
		{ id: '01JOBAUTH', description: 'Fix auth module' },
		{ id: '01JOBDOC', description: 'Write docs' }
	];

	it('filters by description and id', () => {
		expect(filterJobOptions('auth', jobs)).toHaveLength(1);
		expect(filterJobOptions('01JOB', jobs)).toHaveLength(2);
	});
});

describe('expandCreateSkillCommand', () => {
	it('targets skill drafts instead of live skills', () => {
		const prompt = expandCreateSkillCommand('review helper');
		expect(prompt).toContain('write_skill_draft');
		expect(prompt).toContain('~/.cometmind/skill-drafts');
		expect(prompt).toContain('review helper');
	});

	it('can infer from current session context when no request is given', () => {
		const prompt = expandCreateSkillCommand('');
		expect(prompt).toContain('current session context');
		expect(prompt).toContain('current completed job');
	});
});
