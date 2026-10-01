import { describe, expect, it } from 'vitest';
import {
	expandCreateSkillCommand,
	filterJobOptions,
	filterSlashMenuOptions,
	isChangeWorkspaceCommand,
	isJobCommand,
	isModelCommand,
	parseChangeCommand,
	parseJobCommand,
	parseModelCommand,
	scoreSlashMenuMatch
} from './slash-commands';

describe('filterSlashMenuOptions', () => {
	const skills = [
		{ name: 'code-review-master', description: 'Review code and suggest changes' },
		{ name: 'cometline-promo-writer', description: 'Write promo posts for product changes' },
		{ name: 'changelog', description: 'Draft release notes' }
	];

	it('ranks an exact system command first and drops description-only skills', () => {
		const options = filterSlashMenuOptions('change', skills);
		expect(options.map((option) => option.name)).toEqual(['change', 'changelog']);
		expect(options.every((option) => option.name !== 'code-review-master')).toBe(true);
		expect(options.every((option) => option.name !== 'cometline-promo-writer')).toBe(true);
	});

	it('keeps name substring matches alongside a strong prefix hit', () => {
		const options = filterSlashMenuOptions('cha', [
			{ name: 'changelog', description: 'Draft release notes' },
			{ name: 'code-review-master', description: 'Review code and suggest changes' }
		]);
		expect(options.map((option) => option.name)).toEqual(['change', 'changelog']);
	});

	it('still allows description matches when nothing matches by name', () => {
		const options = filterSlashMenuOptions('promo', [
			{ name: 'cometline-promo-writer', description: 'Write promo posts' },
			{ name: 'code-review-master', description: 'Review code' }
		]);
		expect(options.map((option) => option.name)).toEqual(['cometline-promo-writer']);
	});
});

describe('scoreSlashMenuMatch', () => {
	it('scores exact matches above prefixes', () => {
		expect(scoreSlashMenuMatch('change', 'change', 'Fork session')).toBe(4);
		expect(scoreSlashMenuMatch('ch', 'change', 'Fork session')).toBe(3);
		expect(scoreSlashMenuMatch('ange', 'change', 'Fork session')).toBe(2);
		expect(scoreSlashMenuMatch('fork', 'change', 'Fork session')).toBe(1);
	});
});

describe('parseJobCommand', () => {
	it('requires a trailing space before opening the job picker', () => {
		expect(parseJobCommand('/job')).toBeNull();
		expect(parseJobCommand('/job ')).toEqual({ query: '' });
		expect(parseJobCommand('/job auth')).toEqual({ query: 'auth' });
		expect(isJobCommand('/job')).toBe(true);
		expect(isJobCommand('/job auth')).toBe(true);
	});
});

describe('parseChangeCommand', () => {
	it('requires a trailing space before opening the workspace picker', () => {
		expect(parseChangeCommand('/change')).toBeNull();
		expect(parseChangeCommand('/change ')).toEqual({ query: '' });
		expect(parseChangeCommand('/change ~/code')).toEqual({ query: '~/code' });
		expect(isChangeWorkspaceCommand('/change')).toBe(true);
		expect(isChangeWorkspaceCommand('/change ')).toBe(true);
	});
});

describe('parseModelCommand', () => {
	it('requires a trailing space before opening the model picker', () => {
		expect(parseModelCommand('/model')).toBeNull();
		expect(parseModelCommand('/model ')).toEqual({ query: '' });
		expect(parseModelCommand('/model gpt')).toEqual({ query: 'gpt' });
		expect(isModelCommand('/model')).toBe(true);
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
