import { beforeEach, describe, expect, it, vi } from 'vitest';
import * as cometmind from '$lib/client/cometmind';
import { loadPanelFileOptions } from './panel-file-options';

vi.mock('$lib/client/cometmind', () => ({
	listWorkspaceFiles: vi.fn(),
	listWikiFiles: vi.fn()
}));

describe('loadPanelFileOptions', () => {
	beforeEach(() => {
		vi.clearAllMocks();
	});

	it('merges workspace files that match the query', async () => {
		vi.mocked(cometmind.listWorkspaceFiles).mockResolvedValue({
			files: ['docs/index.md'],
			truncated: false
		});
		vi.mocked(cometmind.listWikiFiles).mockResolvedValue({
			files: ['index.md'],
			truncated: false
		});

		const result = await loadPanelFileOptions('/workspace', 'index');
		expect(result).toEqual(['@runtime/wiki/index.md', 'docs/index.md']);
	});

	it('returns wiki-only results without workspace', async () => {
		vi.mocked(cometmind.listWikiFiles).mockResolvedValue({
			files: ['entities/foo.md'],
			truncated: false
		});

		const result = await loadPanelFileOptions('/', 'foo');
		expect(result).toEqual(['@runtime/wiki/entities/foo.md']);
		expect(cometmind.listWorkspaceFiles).not.toHaveBeenCalled();
	});
});
