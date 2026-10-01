import { describe, expect, it } from 'vitest';
import {
	discardConfirmDescription,
	gitFileDir,
	gitFileName,
	gitStatusBadge,
	type GitFile
} from './git-changes-browser';

function file(overrides: Partial<GitFile>): GitFile {
	return {
		path: 'src/a.ts',
		status: 'modified',
		staged: false,
		untracked: false,
		xy: ' M',
		...overrides
	} as GitFile;
}

describe('git-changes-browser', () => {
	it('maps statuses to single-letter badges', () => {
		expect(gitStatusBadge(file({ untracked: true, status: 'modified' }))).toBe('U');
		expect(gitStatusBadge(file({ status: 'modified' }))).toBe('M');
		expect(gitStatusBadge(file({ status: 'added' }))).toBe('A');
		expect(gitStatusBadge(file({ status: 'deleted' }))).toBe('D');
		expect(gitStatusBadge(file({ status: 'renamed' }))).toBe('R');
		expect(gitStatusBadge(file({ status: 'conflict' }))).toBe('!');
		expect(gitStatusBadge(file({ status: 'copied' }))).toBe('C');
		expect(gitStatusBadge(file({ status: '' }))).toBe('?');
	});

	it('splits paths into name and directory', () => {
		expect(gitFileName('src/lib/a.ts')).toBe('a.ts');
		expect(gitFileName('a.ts')).toBe('a.ts');
		expect(gitFileDir('src\\lib\\a.ts')).toBe('src/lib');
		expect(gitFileDir('a.ts')).toBe('');
	});

	it('describes pending discard confirmations', () => {
		expect(discardConfirmDescription({ kind: 'one', path: 'a.ts' }, 3)).toContain(
			'Discard changes to “a.ts”?'
		);
		expect(discardConfirmDescription({ kind: 'all' }, 1)).toContain(
			'Discard all 1 unstaged change?'
		);
		expect(discardConfirmDescription({ kind: 'all' }, 2)).toContain(
			'Discard all 2 unstaged changes?'
		);
		expect(discardConfirmDescription(null, 0)).toBe(
			'Tracked files restore to HEAD. Untracked files are deleted. This cannot be undone.'
		);
	});
});
