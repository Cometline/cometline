import { describe, expect, it } from 'vitest';
import { buildFileTree, dirKeysToExpandForPaths } from './file-tree';

describe('buildFileTree', () => {
	it('returns an empty tree for empty input', () => {
		expect(buildFileTree([])).toEqual([]);
	});

	it('handles single-segment paths', () => {
		expect(buildFileTree(['index.md'])).toEqual([{ name: 'index.md', path: 'index.md' }]);
	});

	it('nests directories and files', () => {
		expect(buildFileTree(['entities/foo.md', 'entities/bar.md', 'index.md'])).toEqual([
			{
				name: 'entities',
				children: [
					{ name: 'bar.md', path: 'entities/bar.md' },
					{ name: 'foo.md', path: 'entities/foo.md' }
				]
			},
			{ name: 'index.md', path: 'index.md' }
		]);
	});

	it('sorts directories before files and sorts names', () => {
		expect(buildFileTree(['z.md', 'a/b.md', 'm.md'])).toEqual([
			{
				name: 'a',
				children: [{ name: 'b.md', path: 'a/b.md' }]
			},
			{ name: 'm.md', path: 'm.md' },
			{ name: 'z.md', path: 'z.md' }
		]);
	});

	it('normalizes backslashes and leading slashes', () => {
		expect(buildFileTree(['\\entities\\foo.md', '/index.md'])).toEqual([
			{
				name: 'entities',
				children: [{ name: 'foo.md', path: 'entities/foo.md' }]
			},
			{ name: 'index.md', path: 'index.md' }
		]);
	});

	it('ignores blank paths', () => {
		expect(buildFileTree(['', '  ', 'ok.md'])).toEqual([{ name: 'ok.md', path: 'ok.md' }]);
	});
});

describe('dirKeysToExpandForPaths', () => {
	it('expands each ancestor directory for nested matches', () => {
		expect(dirKeysToExpandForPaths(['src/lib/foo.ts', 'README.md', 'a/b/c.md'])).toEqual({
			src: true,
			'src/lib': true,
			a: true,
			'a/b': true
		});
	});

	it('returns an empty map for root-level files only', () => {
		expect(dirKeysToExpandForPaths(['a.md', 'b.md'])).toEqual({});
	});
});
