import { describe, expect, it, vi } from 'vitest';
import * as cometmind from '$lib/client/cometmind';
import { loadWikiFileOptions } from './file-options';

vi.mock('$lib/client/cometmind', () => ({
	listWikiFiles: vi.fn()
}));

describe('loadWikiFileOptions', () => {
	it('returns empty list for blank query', async () => {
		expect(await loadWikiFileOptions('   ')).toEqual([]);
		expect(cometmind.listWikiFiles).not.toHaveBeenCalled();
	});

	it('maps wiki files to UI paths', async () => {
		vi.mocked(cometmind.listWikiFiles).mockResolvedValue({
			files: ['index.md', 'entities/foo.md'],
			truncated: false
		});
		const result = await loadWikiFileOptions('index');
		expect(result).toEqual([
			'@runtime/wiki/index.md',
			'@runtime/wiki/entities/foo.md'
		]);
		expect(cometmind.listWikiFiles).toHaveBeenCalledWith('index', 8);
	});
});
