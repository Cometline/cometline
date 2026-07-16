import { describe, expect, it } from 'vitest';
import {
	classifyDiffLine,
	isEditFileTool,
	parseEditDiff,
	shouldRenderEditDiff
} from './parse-edit-diff';

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
		expect(parsed?.path).toBe('main.go');
		expect(parsed?.added).toBe(1);
		expect(parsed?.deleted).toBe(1);
		expect(parsed?.lines.some((l) => l.kind === 'add')).toBe(true);
		expect(parsed?.lines.some((l) => l.kind === 'del')).toBe(true);
		expect(parsed?.lines.some((l) => l.text.includes('*** Begin'))).toBe(false);
	});

	it('parses Go DiffArtifact Format with nested path', () => {
		const output = `edited cometmind/internal/tools/diffartifact/diffartifact.go (+1 -0)

*** Begin Diff
--- a/cometmind/internal/tools/diffartifact/diffartifact.go
+++ b/cometmind/internal/tools/diffartifact/diffartifact.go
@@ -3,6 +3,7 @@
 // One module owns markers and formatting so Go tool output and the
 // UI parser cannot drift. The model still sees a plain-text projection;
+// smoke-test: DiffArtifact contract
 // structured fields exist for callers that need them without re-parsing.
*** End Diff`;
		const parsed = parseEditDiff(output);
		expect(parsed?.path).toBe('cometmind/internal/tools/diffartifact/diffartifact.go');
		expect(parsed?.added).toBe(1);
		expect(parsed?.lines.some((l) => l.kind === 'add' && l.text.includes('smoke-test'))).toBe(
			true
		);
	});

	it('tolerates missing End marker', () => {
		const output = `edited x.go (+1 -0)

*** Begin Diff
--- a/x.go
+++ b/x.go
+hi`;
		expect(parseEditDiff(output)?.lines.some((l) => l.kind === 'add')).toBe(true);
	});

	it('returns null without markers', () => {
		expect(parseEditDiff('just text')).toBeNull();
	});
});

describe('shouldRenderEditDiff', () => {
	it('matches edit_file or marker presence', () => {
		expect(shouldRenderEditDiff('edit_file', '*** Begin Diff\nx\n*** End Diff')).toBe(true);
		expect(shouldRenderEditDiff('run_command', '*** Begin Diff\nx\n*** End Diff')).toBe(true);
		expect(shouldRenderEditDiff('run_command', 'hello')).toBe(false);
		expect(isEditFileTool('edit_file')).toBe(true);
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
