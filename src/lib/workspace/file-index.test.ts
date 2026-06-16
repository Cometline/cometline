import { describe, expect, it, vi, beforeEach } from 'vitest';
import {
	clearAllFileIndexes,
	filterFileIndex,
	getFileIndex,
	isFileIndexReady,
	refreshFileIndex
} from './file-index';
import * as cometmind from '$lib/client/cometmind';

vi.mock('$lib/client/cometmind', () => ({
	listWorkspaceFiles: vi.fn()
}));

describe('file-index', () => {
	beforeEach(() => {
		clearAllFileIndexes();
		vi.resetAllMocks();
	});

	it('returns null before loading', () => {
		expect(getFileIndex('/workspace')).toBeNull();
		expect(isFileIndexReady('/workspace')).toBe(false);
	});

	it('loads and caches the file list', async () => {
		vi.mocked(cometmind.listWorkspaceFiles).mockResolvedValue(['a.go', 'b.md']);

		const result = await refreshFileIndex('/workspace');

		expect(result.files).toEqual(['a.go', 'b.md']);
		expect(result.loaded).toBe(true);
		expect(result.loading).toBe(false);
		expect(isFileIndexReady('/workspace')).toBe(true);
		expect(cometmind.listWorkspaceFiles).toHaveBeenCalledTimes(1);

		const cached = await refreshFileIndex('/workspace');
		expect(cached.files).toEqual(['a.go', 'b.md']);
		expect(cometmind.listWorkspaceFiles).toHaveBeenCalledTimes(2);
	});

	it('deduplicates concurrent refresh requests', async () => {
		vi.mocked(cometmind.listWorkspaceFiles).mockImplementation(
			() => new Promise((resolve) => setTimeout(() => resolve(['x.go']), 10))
		);

		const [a, b] = await Promise.all([
			refreshFileIndex('/workspace'),
			refreshFileIndex('/workspace')
		]);

		expect(a.files).toEqual(['x.go']);
		expect(b.files).toEqual(['x.go']);
		expect(cometmind.listWorkspaceFiles).toHaveBeenCalledTimes(1);
	});

	it('records an error without clearing existing data', async () => {
		vi.mocked(cometmind.listWorkspaceFiles)
			.mockResolvedValueOnce(['a.go'])
			.mockRejectedValueOnce(new Error('network error'));

		await refreshFileIndex('/workspace');
		const result = await refreshFileIndex('/workspace');

		expect(result.error).toBe('network error');
		expect(result.files).toEqual(['a.go']);
		expect(result.loaded).toBe(true);
	});

	it('filters files by query case-insensitively', () => {
		const files = ['README.md', 'src/app.svelte', 'main.go'];
		expect(filterFileIndex(files, 'md')).toEqual(['README.md']);
		expect(filterFileIndex(files, 'svelte')).toEqual(['src/app.svelte']);
		expect(filterFileIndex(files, '')).toEqual(files);
	});
});
