import { describe, expect, it } from 'vitest';
import { createFileDiff } from './file-diff';

describe('createFileDiff', () => {
	it('returns an empty diff when file contents match', () => {
		expect(createFileDiff('same\n', 'same\n')).toBe('');
	});

	it('creates a unified hunk with additions, deletions, and context', () => {
		expect(createFileDiff('one\ntwo\nthree\n', 'one\nsecond\nthree\n')).toBe(
			[
				'--- Current draft',
				'+++ Disk version',
				'@@ -1,3 +1,3 @@',
				' one',
				'-two',
				'+second',
				' three',
				''
			].join('\n')
		);
	});
});
