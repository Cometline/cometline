import { describe, expect, it, vi, beforeEach } from 'vitest';
import {
	clearAllFileIndexes,
	filterFileIndex,
	getFileIndex,
	isFileIndexFresh,
	isFileIndexReady,
	isFileIndexTruncated,
	refreshFileIndex,
	searchWorkspaceFiles
} from './file-index';
import * as cometmind from '$lib/client/cometmind';

vi.mock('$lib/client/cometmind', () => ({
	listWorkspaceFiles: vi.fn()
}));

/** Build the client return shape { files, truncated }. */
function wf(files: string[], truncated = false) {
	return { files, truncated };
}

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
		vi.mocked(cometmind.listWorkspaceFiles).mockResolvedValue(wf(['a.go', 'b.md']));

		const result = await refreshFileIndex('/workspace');

		expect(result.files).toEqual(['a.go', 'b.md']);
		expect(result.loaded).toBe(true);
		expect(result.loading).toBe(false);
		expect(result.truncated).toBe(false);
		expect(isFileIndexReady('/workspace')).toBe(true);
		expect(cometmind.listWorkspaceFiles).toHaveBeenCalledTimes(1);

		const cached = await refreshFileIndex('/workspace');
		expect(cached.files).toEqual(['a.go', 'b.md']);
		expect(cometmind.listWorkspaceFiles).toHaveBeenCalledTimes(2);
	});

	it('records truncation from the backend', async () => {
		vi.mocked(cometmind.listWorkspaceFiles).mockResolvedValue(wf(['a.go'], true));
		await refreshFileIndex('/workspace');
		expect(isFileIndexTruncated('/workspace')).toBe(true);
	});

	it('deduplicates concurrent refresh requests', async () => {
		vi.mocked(cometmind.listWorkspaceFiles).mockImplementation(
			() => new Promise((resolve) => setTimeout(() => resolve(wf(['x.go'])), 10))
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
			.mockResolvedValueOnce(wf(['a.go']))
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

	it('treats the index as stale after the TTL elapses', async () => {
		vi.useFakeTimers();
		try {
			vi.mocked(cometmind.listWorkspaceFiles).mockResolvedValue(wf(['a.go']));
			await refreshFileIndex('/workspace');

			expect(isFileIndexFresh('/workspace', 1000)).toBe(true);
			vi.advanceTimersByTime(1500);
			expect(isFileIndexFresh('/workspace', 1000)).toBe(false);
			// Still ready (usable), just stale.
			expect(isFileIndexReady('/workspace')).toBe(true);
		} finally {
			vi.useRealTimers();
		}
	});

	it('keeps the existing list usable while a background refresh runs', async () => {
		vi.mocked(cometmind.listWorkspaceFiles).mockResolvedValueOnce(wf(['a.go']));
		await refreshFileIndex('/workspace');
		expect(isFileIndexReady('/workspace')).toBe(true);

		let resolve!: (value: { files: string[]; truncated: boolean }) => void;
		vi.mocked(cometmind.listWorkspaceFiles).mockImplementationOnce(
			() => new Promise((r) => (resolve = r))
		);
		const refreshing = refreshFileIndex('/workspace');
		// During the background refresh the index stays "ready" with old files.
		expect(isFileIndexReady('/workspace')).toBe(true);
		expect(getFileIndex('/workspace')?.files).toEqual(['a.go']);

		resolve(wf(['a.go', 'b.go']));
		await refreshing;
		expect(getFileIndex('/workspace')?.files).toEqual(['a.go', 'b.go']);
	});

	it('searchWorkspaceFiles queries the backend with the term', async () => {
		vi.mocked(cometmind.listWorkspaceFiles).mockResolvedValue(wf(['deep/nested/match.go']));
		const result = await searchWorkspaceFiles('/workspace', 'match');
		expect(result).toEqual(['deep/nested/match.go']);
		expect(cometmind.listWorkspaceFiles).toHaveBeenCalledWith('/workspace', 'match', 50);
	});

	it('searchWorkspaceFiles returns empty for blank query without hitting backend', async () => {
		const result = await searchWorkspaceFiles('/workspace', '   ');
		expect(result).toEqual([]);
		expect(cometmind.listWorkspaceFiles).not.toHaveBeenCalled();
	});
});
