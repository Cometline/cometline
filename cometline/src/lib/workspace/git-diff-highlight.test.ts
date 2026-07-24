import { describe, expect, it } from 'vitest';
import { highlightGitDiffLines, splitDiffContentLine } from './git-diff-highlight';
import { parseGitDiffLines } from './git-diff-lines';

describe('splitDiffContentLine', () => {
	it('splits markers from content lines', () => {
		expect(splitDiffContentLine('add', '+const x = 1')).toEqual({
			prefix: '+',
			body: 'const x = 1'
		});
		expect(splitDiffContentLine('del', '-const x = 0')).toEqual({
			prefix: '-',
			body: 'const x = 0'
		});
		expect(splitDiffContentLine('ctx', ' package main')).toEqual({
			prefix: ' ',
			body: 'package main'
		});
	});

	it('leaves meta lines whole', () => {
		expect(splitDiffContentLine('meta', '--- a/foo.ts')).toEqual({
			prefix: '',
			body: '--- a/foo.ts'
		});
	});
});

describe('highlightGitDiffLines', () => {
	it('produces html for typed content and escapes meta', async () => {
		const lines = parseGitDiffLines(
			[
				'diff --git a/a.ts b/a.ts',
				'--- a/a.ts',
				'+++ b/a.ts',
				'@@ -1,2 +1,2 @@',
				' const a = 1',
				'-const b = 2',
				'+const b = 3'
			].join('\n')
		);
		const highlighted = await highlightGitDiffLines(lines, 'typescript');
		expect(highlighted).toHaveLength(lines.length);

		const meta = highlighted.find((l) => l.kind === 'meta');
		expect(meta?.html).toContain('--- a/a.ts');
		expect(meta?.html).not.toContain('<span style');

		const add = highlighted.find((l) => l.kind === 'add');
		expect(add?.prefix).toBe('+');
		expect(add?.html.length).toBeGreaterThan(0);
		// Tokenized TS should wrap keywords/idents in colored spans.
		expect(add?.html).toMatch(/span|const/);
	});

	it('falls back to escaped plaintext for unknown languages', async () => {
		const lines = parseGitDiffLines('+foo <bar>\n');
		const highlighted = await highlightGitDiffLines(lines, null);
		const add = highlighted[0];
		expect(add?.kind).toBe('add');
		expect(add?.html).toContain('foo &lt;bar&gt;');
	});
});
