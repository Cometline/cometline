import { describe, expect, it } from 'vitest';
import { jobMenuSubtitle, jobUserDisplayText, truncateJobLabel } from './format-job-label';

describe('truncateJobLabel', () => {
	it('returns short text unchanged', () => {
		expect(truncateJobLabel('Fix auth')).toBe('Fix auth');
	});

	it('truncates long text', () => {
		const long = 'a'.repeat(100);
		expect(truncateJobLabel(long, 80)).toHaveLength(80);
		expect(truncateJobLabel(long, 80).endsWith('…')).toBe(true);
	});
});

describe('jobMenuSubtitle', () => {
	it('omits zero priority', () => {
		expect(jobMenuSubtitle({ priority: 0 })).toBe('');
	});

	it('includes priority and workspace', () => {
		expect(
			jobMenuSubtitle({ priority: 2, workspace_path: '/Users/me/project/src' })
		).toContain('Priority 2');
	});
});

describe('jobUserDisplayText', () => {
	it('returns /job with description', () => {
		expect(jobUserDisplayText({ description: 'Fix login' })).toBe('/job Fix login');
	});

	it('returns bare /job for empty description', () => {
		expect(jobUserDisplayText({ description: '   ' })).toBe('/job');
	});
});
