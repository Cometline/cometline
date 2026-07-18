import { describe, expect, it } from 'vitest';
import {
	buildFileSnippetContext,
	findSelectionInSource,
	lineRangeFromOffsets
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
