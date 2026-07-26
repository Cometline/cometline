// @vitest-environment jsdom
import { describe, expect, it } from 'vitest';
import {
	buildFileSnippetContext,
	findSelectionInSource,
	lineRangeFromOffsets,
	sourceLineRangeFromDomRange
} from './selection-snippet';

describe('lineRangeFromOffsets', () => {
	it('maps offsets to 1-based lines', () => {
		const source = 'a\nbb\nccc\n';
		expect(lineRangeFromOffsets(source, 0, 1)).toEqual({ startLine: 1, endLine: 1 });
		expect(lineRangeFromOffsets(source, 2, 4)).toEqual({ startLine: 2, endLine: 2 });
		expect(lineRangeFromOffsets(source, 2, 8)).toEqual({ startLine: 2, endLine: 3 });
	});
});

describe('findSelectionInSource', () => {
	it('finds a unique exact match', () => {
		expect(findSelectionInSource('one\ntwo\nthree', 'two')).toEqual({ start: 4, end: 7 });
	});

	it('returns null when the selection is ambiguous', () => {
		expect(findSelectionInSource('foo bar foo', 'foo')).toBeNull();
	});

	it('matches across normalized whitespace', () => {
		const source = 'hello   world\nnext';
		expect(findSelectionInSource(source, 'hello world')).toEqual({ start: 0, end: 13 });
	});
});

describe('sourceLineRangeFromDomRange', () => {
	it('reads a line from the nearest annotated rendered node', () => {
		const root = document.createElement('div');
		root.innerHTML =
			'<p data-source-start-line="2" data-source-end-line="3"><strong data-source-start-line="3" data-source-end-line="3"><span data-source-start-line="3" data-source-end-line="3">bold</span></strong></p>';
		const text = root.querySelector('span')?.firstChild;
		expect(text).toBeTruthy();
		const range = document.createRange();
		range.setStart(text!, 0);
		range.setEnd(text!, 4);
		expect(sourceLineRangeFromDomRange(range, root)).toEqual({ startLine: 3, endLine: 3 });
	});

	it('combines annotations for a cross-node selection', () => {
		const root = document.createElement('div');
		root.innerHTML = [
			'<span data-source-start-line="4" data-source-end-line="4">first</span>',
			'<em data-source-start-line="6" data-source-end-line="6"><span data-source-start-line="6" data-source-end-line="6">last</span></em>'
		].join(' ');
		const first = root.querySelector('span')?.firstChild;
		const last = root.querySelector('em span')?.firstChild;
		expect(first).toBeTruthy();
		expect(last).toBeTruthy();
		const range = document.createRange();
		range.setStart(first!, 1);
		range.setEnd(last!, 3);
		expect(sourceLineRangeFromDomRange(range, root)).toEqual({ startLine: 4, endLine: 6 });
	});

	it('returns null for unannotated content so source matching remains available', () => {
		const root = document.createElement('div');
		root.textContent = 'plain text';
		const text = root.firstChild!;
		const range = document.createRange();
		range.setStart(text, 0);
		range.setEnd(text, 5);
		expect(sourceLineRangeFromDomRange(range, root)).toBeNull();
	});
});

describe('buildFileSnippetContext', () => {
	it('builds a workspace snippet with line range', () => {
		const ctx = buildFileSnippetContext({
			filePath: 'src/a.ts',
			selectedText: 'const x = 1',
			sourceText: 'intro\nconst x = 1\nend\n'
		});
		expect(ctx).toEqual({
			kind: 'file',
			title: 'a.ts:2',
			source: 'workspace-file:src/a.ts#L2-L2',
			content: 'const x = 1'
		});
	});

	it('keeps wiki paths', () => {
		const ctx = buildFileSnippetContext({
			filePath: '@runtime/wiki/notes.md',
			selectedText: 'hello',
			lineRange: { startLine: 3, endLine: 5 }
		});
		expect(ctx?.source).toBe('@runtime/wiki/notes.md#L3-L5');
		expect(ctx?.title).toBe('notes.md:3-5');
	});
});
