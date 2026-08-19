// @vitest-environment jsdom
import { render, screen, waitFor } from '@testing-library/svelte';
import { beforeEach, describe, expect, it, vi } from 'vitest';

const {
	getFileTreeExpanded,
	listWikiFileChildren,
	listWorkspaceFileChildren,
	listWorkspaceFiles,
	setFileTreeExpanded,
	getFileIndex,
	refreshFileIndex,
	searchWorkspaceFiles,
	isFileIndexTruncated,
	getCachedWikiFiles,
	refreshWikiFileIndex
} = vi.hoisted(() => ({
	getFileTreeExpanded: vi.fn(() => ({})),
	listWikiFileChildren: vi.fn(),
	listWorkspaceFileChildren: vi.fn(),
	listWorkspaceFiles: vi.fn(),
	setFileTreeExpanded: vi.fn(),
	getFileIndex: vi.fn(),
	refreshFileIndex: vi.fn(),
	searchWorkspaceFiles: vi.fn(),
	isFileIndexTruncated: vi.fn(() => false),
	getCachedWikiFiles: vi.fn(() => []),
	refreshWikiFileIndex: vi.fn(async () => [])
}));

vi.mock('$lib/client/cometmind', () => ({
	listWikiFileChildren,
	listWikiFiles: vi.fn(),
	listWorkspaceFileChildren,
	listWorkspaceFiles
}));

vi.mock('$lib/stores/shell.svelte', () => ({
	shellStore: { getFileTreeExpanded, setFileTreeExpanded }
}));

vi.mock('$lib/workspace/file-index', () => ({
	normalizeWorkspacePath: (path: string) => path,
	getFileIndex,
	refreshFileIndex,
	searchWorkspaceFiles,
	isFileIndexTruncated,
	filterFileIndex: (files: string[], query: string) => {
		const q = query.trim().toLowerCase();
		if (!q) return files;
		return files.filter((path) => path.toLowerCase().includes(q));
	}
}));

vi.mock('$lib/wiki/wiki-file-index', () => ({
	getCachedWikiFiles,
	refreshWikiFileIndex
}));

vi.mock('$lib/workspace/workspace-change.svelte', () => ({
	workspaceChangeVersion: () => 0
}));

import FileTreeBrowser from './FileTreeBrowser.svelte';

describe('FileTreeBrowser', () => {
	beforeEach(() => {
		getFileTreeExpanded.mockReturnValue({});
		listWikiFileChildren.mockReset();
		listWikiFileChildren.mockResolvedValue({ files: ['entities/'], truncated: false });
		listWorkspaceFileChildren.mockReset();
		listWorkspaceFileChildren.mockResolvedValue({ files: ['src/'], truncated: false });
		listWorkspaceFiles.mockReset();
		setFileTreeExpanded.mockReset();
		getFileIndex.mockReset();
		refreshFileIndex.mockReset();
		searchWorkspaceFiles.mockReset();
		isFileIndexTruncated.mockReturnValue(false);
		getCachedWikiFiles.mockReturnValue([]);
		refreshWikiFileIndex.mockResolvedValue([]);
	});

	it('loads the root directory once without retriggering from its cache update', async () => {
		render(FileTreeBrowser, {
			workspacePath: '',
			onSelectFile: () => {},
			source: 'wiki'
		});

		await waitFor(() => {
			expect(screen.getByRole('button', { name: 'entities' })).toBeTruthy();
		});
		await new Promise((resolve) => setTimeout(resolve, 20));
		expect(listWikiFileChildren).toHaveBeenCalledTimes(1);
		expect(listWikiFileChildren).toHaveBeenCalledWith('', 10000);
	});

	it('filters workspace files from the local index without a 10k list query', async () => {
		getFileIndex.mockReturnValue({
			files: ['src/app.ts', 'src/lib/foo.ts', 'README.md'],
			loading: false,
			loaded: true,
			error: null,
			loadedAt: Date.now(),
			truncated: false
		});

		render(FileTreeBrowser, {
			workspacePath: '/repo',
			onSelectFile: () => {},
			source: 'workspace',
			filter: 'app'
		});

		await waitFor(() => {
			expect(screen.getByRole('button', { name: /app\.ts/ })).toBeTruthy();
		});
		expect(listWorkspaceFiles).not.toHaveBeenCalled();
		expect(listWorkspaceFileChildren).not.toHaveBeenCalled();
		expect(screen.queryByRole('button', { name: 'src' })).toBeNull();
	});
});
