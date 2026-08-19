import { beforeEach, describe, expect, it, vi } from 'vitest';
import { clearAllFileIndexes, refreshFileIndex } from './file-index';
import { loadWorkspacePanelFileOptions, rankWorkspaceFileMatches } from './workspace-panel-input-options';
import * as cometmind from '$lib/client/cometmind';

vi.mock('$lib/client/cometmind', () => ({
	listWorkspaceFiles: vi.fn()
}));

function wf(files: string[], truncated = false) {
	return { files, truncated };
}

describe('workspace-panel-input-options', () => {
	beforeEach(() => {
		clearAllFileIndexes();
		vi.resetAllMocks();
	});

	it('returns no file options for blank input or root workspace', async () => {
		expect(await loadWorkspacePanelFileOptions('/workspace', '   ')).toEqual([]);
		expect(await loadWorkspacePanelFileOptions('/', 'README.md')).toEqual([]);
		expect(cometmind.listWorkspaceFiles).not.toHaveBeenCalled();
	});

	it('refreshes a missing index and returns ranked cached matches', async () => {
		vi.mocked(cometmind.listWorkspaceFiles).mockResolvedValue(
			wf(['src/youtube.ts', 'README.md', 'youtube'])
		);

		const result = await loadWorkspacePanelFileOptions('/workspace', 'youtube');

		expect(result).toEqual(['youtube', 'src/youtube.ts']);
		expect(cometmind.listWorkspaceFiles).toHaveBeenCalledWith('/workspace', '', 50000, {
			index: true
		});
	});

	it('uses cached fresh index without another refresh', async () => {
		vi.mocked(cometmind.listWorkspaceFiles).mockResolvedValue(wf(['src/app.svelte']));
		await refreshFileIndex('/workspace');

		const result = await loadWorkspacePanelFileOptions('/workspace', 'app');

		expect(result).toEqual(['src/app.svelte']);
		expect(cometmind.listWorkspaceFiles).toHaveBeenCalledTimes(1);
	});

	it('uses server search when the cached index is truncated', async () => {
		vi.mocked(cometmind.listWorkspaceFiles)
			.mockResolvedValueOnce(wf(['a.ts'], true))
			.mockResolvedValueOnce(wf(['deep/youtube.md', 'youtube']));

		const result = await loadWorkspacePanelFileOptions('/workspace', 'youtube');

		expect(result).toEqual(['youtube', 'deep/youtube.md']);
		expect(cometmind.listWorkspaceFiles).toHaveBeenNthCalledWith(2, '/workspace', 'youtube', 50, {
			index: true
		});
	});

	it('limits returned file options', async () => {
		vi.mocked(cometmind.listWorkspaceFiles).mockResolvedValue(wf(['a.ts', 'b.ts', 'c.ts']));

		expect(await loadWorkspacePanelFileOptions('/workspace', '.ts', 2)).toEqual(['a.ts', 'b.ts']);
	});

	it('ranks exact, basename, prefix, then substring matches', () => {
		expect(
			rankWorkspaceFileMatches(
				['src/foo-youtube.ts', 'src/youtube.ts', 'youtube', 'docs/YOUTUBE.md'],
				'youtube'
			)
		).toEqual(['youtube', 'src/youtube.ts', 'docs/YOUTUBE.md', 'src/foo-youtube.ts']);
	});
});
