import { describe, expect, it } from 'vitest';
import { parseGitDiffLines } from './git-diff-lines';

describe('parseGitDiffLines', () => {
	it('classifies unified diff lines', () => {
		const lines = parseGitDiffLines(
			[
				'diff --git a/a.go b/a.go',
				'--- a/a.go',
				'+++ b/a.go',
				'@@ -1,2 +1,3 @@',
				' package a',
				'-old',
				'+new',
				'+more'
			].join('\n')
		);
		expect(lines.map((l) => l.kind)).toEqual([
			'other',
			'meta',
			'meta',
			'hunk',
			'ctx',
			'del',
			'add',
			'add'
		]);
	});
});
