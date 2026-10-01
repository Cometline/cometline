import { beforeEach, describe, expect, it, vi } from 'vitest';
import * as cometmind from '$lib/client/cometmind';
import { loadPanelFileOptions } from './panel-file-options';

vi.mock('$lib/client/cometmind', () => ({
	listWorkspaceFiles: vi.fn()
}));

describe('loadPanelFileOptions', () => {
	beforeEach(() => {
		vi.clearAllMocks();
	});

	it('returns workspace matches for a query', async () => {
		vi.mocked(cometmind.listWorkspaceFiles).mockResolvedValue({
			files: ['cometline/README.md', 'cometline/package.json'],
			truncated: true
		});

		const result = await loadPanelFileOptions('/workspace', 'cometline');
		expect(result.some((path) => path.startsWith('cometline/'))).toBe(true);
	});

	it('returns empty list without a workspace', async () => {
		expect(await loadPanelFileOptions('/', 'foo')).toEqual([]);
		expect(cometmind.listWorkspaceFiles).not.toHaveBeenCalled();
	});
});
