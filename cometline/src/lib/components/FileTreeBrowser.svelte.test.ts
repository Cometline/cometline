// @vitest-environment jsdom
import { render, screen, waitFor } from '@testing-library/svelte';
import { beforeEach, describe, expect, it, vi } from 'vitest';

const { getFileTreeExpanded, listWikiFileChildren, setFileTreeExpanded } = vi.hoisted(() => ({
	getFileTreeExpanded: vi.fn(() => ({})),
	listWikiFileChildren: vi.fn(),
	setFileTreeExpanded: vi.fn()
}));

vi.mock('$lib/client/cometmind', () => ({
	listWikiFileChildren,
	listWikiFiles: vi.fn(),
	listWorkspaceFileChildren: vi.fn(),
	listWorkspaceFiles: vi.fn()
}));

vi.mock('$lib/stores/shell.svelte', () => ({
	shellStore: { getFileTreeExpanded, setFileTreeExpanded }
}));

vi.mock('$lib/workspace/file-index', () => ({
	normalizeWorkspacePath: (path: string) => path
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
		setFileTreeExpanded.mockReset();
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
});
