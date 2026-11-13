import { describe, expect, it, vi } from 'vitest';

const { searchWorkspaceContent, searchWikiContent } = vi.hoisted(() => ({
	searchWorkspaceContent: vi.fn(),
	searchWikiContent: vi.fn()
}));

vi.mock('#lib/client/cometmind.js', () => ({ searchWorkspaceContent, searchWikiContent }));

import { runContentSearch } from './content-search';

describe('runContentSearch', () => {
	it('does not call the endpoint for an empty query', async () => {
		await expect(runContentSearch('workspace', '  ', '/repo')).resolves.toEqual([]);
		expect(searchWorkspaceContent).not.toHaveBeenCalled();
	});

	it('returns workspace matches', async () => {
		searchWorkspaceContent.mockResolvedValue({
			matches: [{ path: 'src/app.ts', line: 12, column: 1, preview: 'match' }],
			truncated: false
		});
		await expect(runContentSearch('workspace', 'match', '/repo')).resolves.toEqual([
			{ path: 'src/app.ts', line: 12, column: 1, preview: 'match' }
		]);
	});

	it('keeps a failed search from returning matches', async () => {
		searchWorkspaceContent.mockRejectedValue(new Error('search failed'));
		await expect(runContentSearch('workspace', 'match', '/repo')).rejects.toThrow(
			'search failed'
		);
	});
});
