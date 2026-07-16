import { describe, expect, it } from 'vitest';
import { classifyDiffLine, parseEditDiff } from './parse-edit-diff';

describe('parseEditDiff', () => {
	it('parses edit_file wrapper', () => {
		const output = `edited main.go (+1 -1)

*** Begin Diff
--- a/main.go
+++ b/main.go
@@ -1,3 +1,3 @@
 package main
-func Hello() {}
+func Hello() string { return "hi" }
*** End Diff`;

		const parsed = parseEditDiff(output);
		expect(parsed).not.toBeNull();
		expect(parsed?.summary).toContain('edited main.go');
		expect(parsed?.lines.some((l) => l.kind === 'add')).toBe(true);
		expect(parsed?.lines.some((l) => l.kind === 'del')).toBe(true);
	});

	it('returns null without markers', () => {
		expect(parseEditDiff('just text')).toBeNull();
	});
});

describe('classifyDiffLine', () => {
	it('classifies prefixes', () => {
		expect(classifyDiffLine('--- a/x')).toBe('meta');
		expect(classifyDiffLine('+++ b/x')).toBe('meta');
		expect(classifyDiffLine('@@ -1 +1 @@')).toBe('hunk');
		expect(classifyDiffLine('+added')).toBe('add');
		expect(classifyDiffLine('-removed')).toBe('del');
		expect(classifyDiffLine(' context')).toBe('ctx');
	});
});
