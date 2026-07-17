import { describe, expect, it } from 'vitest';
import {
	resolveMentionSourcePaths,
	shouldRunMentionServerSearch
} from './composer-mention-search';

describe('shouldRunMentionServerSearch', () => {
	it('searches the full workspace even when the warm cache already matches', () => {
		expect(shouldRunMentionServerSearch(true, 'cometline')).toBe(true);
	});

	it('uses server search only when truncated and the cache has no matches', () => {
		expect(shouldRunMentionServerSearch(true, 'deep/nested')).toBe(true);
		expect(shouldRunMentionServerSearch(false, 'deep/nested')).toBe(false);
	});
});

describe('resolveMentionSourcePaths', () => {
	it('keeps local matches while the user completes a filename', () => {
		expect(
			resolveMentionSourcePaths(
				['cometline/README.md'],
				[],
				'',
				'cometline/readme',
				true
			)
		).toEqual(['cometline/README.md']);
	});

	it('uses complete server results after a truncated index search', () => {
		expect(
			resolveMentionSourcePaths(
				['src/index.ts'],
				['src/index.ts', 'packages/cli/index.ts'],
				'index',
				'index',
				true
			)
		).toEqual(['src/index.ts', 'packages/cli/index.ts']);
	});

	it('falls back to server results when the cache is empty', () => {
		expect(
			resolveMentionSourcePaths(
				[],
				['deep/nested/match.go'],
				'match',
				'match',
				true
			)
		).toEqual(['deep/nested/match.go']);
	});

	it('treats null server results as an empty list', () => {
		expect(resolveMentionSourcePaths([], null, 'asdsad', 'asdsad', true)).toEqual([]);
	});
});
