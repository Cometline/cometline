import { beforeEach, describe, expect, it, vi } from 'vitest';

vi.mock('$lib/client/cometmind', () => ({
	listWikiFiles: vi.fn()
}));

vi.mock('$lib/wiki/wiki-file-index', () => ({
	getCachedWikiFiles: vi.fn(() => []),
	refreshWikiFileIndex: vi.fn(async () => [])
}));

vi.mock('$lib/workspace/file-index', () => ({
	filterFileIndex: vi.fn((files: string[], query: string) => {
		const q = query.trim().toLowerCase();
		if (!q) return files;
		return files.filter((path) => path.toLowerCase().includes(q));
	}),
	getFileIndex: vi.fn(() => null),
	isFileIndexFresh: vi.fn(() => false),
	normalizeWorkspacePath: (path: string) => path.replace(/\/+$/, ''),
	refreshFileIndex: vi.fn(async () => ({
		files: ['src/app.ts', 'src/lib/foo.ts', 'README.md'],
		loading: false,
		loaded: true,
		error: null,
		loadedAt: Date.now(),
		truncated: false
	})),
	searchWorkspaceFiles: vi.fn(async () => [])
}));

import { listWikiFiles } from '$lib/client/cometmind';
import { refreshWikiFileIndex } from '$lib/wiki/wiki-file-index';
import { refreshFileIndex } from '$lib/workspace/file-index';
import { loadFileSearchOptions, rankFilePaths } from './file-search';

describe('rankFilePaths', () => {
	it('ranks basename prefix matches above path substring matches', () => {
		const ranked = rankFilePaths(
			['pkg/foo/bar.ts', 'foo.ts', 'src/food.ts'],
			'foo'
		);
		expect(ranked[0]).toBe('foo.ts');
	});

	it('sorts alphabetically when the query is empty', () => {
		expect(rankFilePaths(['b.ts', 'a.ts'], '')).toEqual(['a.ts', 'b.ts']);
	});
});

describe('loadFileSearchOptions', () => {
	beforeEach(() => {
		vi.clearAllMocks();
	});

	it('loads only non-hidden workspace files before the user enters a query', async () => {
		vi.mocked(refreshFileIndex).mockResolvedValueOnce({
			files: ['.env', '.github/workflows/test.yml', 'README.md', 'src/', 'src/app.ts'],
			loading: false,
			loaded: true,
			error: null,
			loadedAt: Date.now(),
			truncated: false
		});
		const files = await loadFileSearchOptions('workspace', '/repo', '', 10);
		expect(refreshFileIndex).toHaveBeenCalledWith('/repo');
		expect(files).toEqual(['README.md', 'src/app.ts']);
	});

	it('loads only non-hidden wiki files before the user enters a query', async () => {
		vi.mocked(refreshWikiFileIndex).mockResolvedValueOnce([
			'.private.md',
			'topics/',
			'topics/overview.md',
			'index.md'
		]);
		const files = await loadFileSearchOptions('wiki', '/repo', '', 10);
		expect(files).toEqual(['index.md', 'topics/overview.md']);
	});

	it('queries listWikiFiles when filtering wiki results', async () => {
		vi.mocked(listWikiFiles).mockResolvedValueOnce({
			files: ['topics/', 'topics/setup.md', 'topics/setup-guide.md'],
			truncated: false
		});
		const files = await loadFileSearchOptions('wiki', '/repo', 'setup', 10);
		expect(listWikiFiles).toHaveBeenCalled();
		expect(files[0]).toBe('topics/setup.md');
		expect(files).not.toContain('topics/');
	});

	it('returns hidden files when the query matches', async () => {
		vi.mocked(listWikiFiles).mockResolvedValueOnce({
			files: ['.private.md'],
			truncated: false
		});
		const files = await loadFileSearchOptions('wiki', '/repo', 'private', 10);
		expect(files).toEqual(['.private.md']);
	});

	it('omits directories from workspace search results', async () => {
		vi.mocked(refreshFileIndex).mockResolvedValueOnce({
			files: ['src/', 'src/app.ts'],
			loading: false,
			loaded: true,
			error: null,
			loadedAt: Date.now(),
			truncated: false
		});
		const files = await loadFileSearchOptions('workspace', '/repo', 'src', 10);
		expect(files).toEqual(['src/app.ts']);
	});
});
