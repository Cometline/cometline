import { describe, expect, it } from 'vitest';
import {
	resolveMentionSourcePaths,
	shouldRunMentionServerSearch
} from './composer-mention-search';

describe('shouldRunMentionServerSearch', () => {
	it('skips server search when the warm cache already matches', () => {
		expect(shouldRunMentionServerSearch(true, 'cometline', ['cometline/README.md'])).toBe(
			false
		);
	});

	it('uses server search only when truncated and the cache has no matches', () => {
		expect(shouldRunMentionServerSearch(true, 'deep/nested', [])).toBe(true);
		expect(shouldRunMentionServerSearch(false, 'deep/nested', [])).toBe(false);
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
