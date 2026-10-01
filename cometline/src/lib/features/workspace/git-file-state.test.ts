import { describe, expect, it } from 'vitest';
import { canStageGitFile, canUnstageGitFile, hasUnstagedSide } from './git-file-state';

describe('git-file-state', () => {
	it('treats untracked files as having an unstaged side', () => {
		const file = { untracked: true, staged: false, xy: '??' };
		expect(hasUnstagedSide(file)).toBe(true);
		expect(canStageGitFile(file)).toBe(true);
		expect(canUnstageGitFile(file)).toBe(false);
	});

	it('disables stage for fully staged tracked files', () => {
		const file = { untracked: false, staged: true, xy: 'M ' };
		expect(hasUnstagedSide(file)).toBe(false);
		expect(canStageGitFile(file)).toBe(false);
		expect(canUnstageGitFile(file)).toBe(true);
	});

	it('allows both stage and unstage for partially staged files', () => {
		const file = { untracked: false, staged: true, xy: 'MM' };
		expect(hasUnstagedSide(file)).toBe(true);
		expect(canStageGitFile(file)).toBe(true);
		expect(canUnstageGitFile(file)).toBe(true);
	});

	it('allows stage only for unstaged worktree changes', () => {
		const file = { untracked: false, staged: false, xy: ' M' };
		expect(canStageGitFile(file)).toBe(true);
		expect(canUnstageGitFile(file)).toBe(false);
	});

	it('returns false for missing file state', () => {
		expect(canStageGitFile(null)).toBe(false);
		expect(canUnstageGitFile(undefined)).toBe(false);
	});
});
