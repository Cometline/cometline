import { describe, expect, it } from 'vitest';
import {
	commitMenuItems,
	dialogInputValue,
	formatGraphDate,
	isUnsafeRefName,
	graphStrokes,
	laneColumnCount,
	refMenuItems,
	type GitGraphCommit
} from './git-graph';

function commit(partial: Partial<GitGraphCommit> & Pick<GitGraphCommit, 'sha'>): GitGraphCommit {
	return {
		short_sha: partial.sha.slice(0, 7),
		parents: [],
		subject: partial.sha,
		lane: 0,
		lanes: [{ lane: 0, kind: 'commit' }],
		refs: [],
		...partial
	};
}

describe('git graph menu', () => {
	it('hides delete on the current branch and hides remote actions', () => {
		expect(refMenuItems({ name: 'main', kind: 'branch', current: true }).map((item) => item.id)).toEqual([
			'checkout',
			'rename'
		]);
		expect(refMenuItems({ name: 'origin/main', kind: 'remote' })).toEqual([]);
		expect(refMenuItems({ name: 'HEAD → dev', kind: 'head', current: true })).toEqual([]);
		expect(refMenuItems({ name: 'v1', kind: 'tag' }).map((item) => item.id)).toEqual([
			'rename',
			'delete'
		]);
	});

	it('offers checkout, branch, and tag on a non-HEAD commit', () => {
		const row = commit({ sha: 'abc1234def' });
		expect(commitMenuItems(row, 'other').map((item) => item.id)).toEqual([
			'checkout',
			'create-branch',
			'create-tag'
		]);
		expect(commitMenuItems(row, row.sha).map((item) => item.id)).toEqual([
			'create-branch',
			'create-tag'
		]);
	});

	it('reads rename input separately from create input', () => {
		expect(dialogInputValue({ kind: 'rename-branch', next: 'next' })).toBe('next');
		expect(dialogInputValue({ kind: 'create-tag', name: 'v1' })).toBe('v1');
		expect(dialogInputValue({ kind: 'delete-tag' })).toBe('');
	});

	it('formats the graph date like a short local timestamp', () => {
		expect(formatGraphDate(0)).toBe('');
		expect(formatGraphDate(1_762_502_640)).toMatch(/\d{4}/);
	});

	it('rejects empty and HEAD names before an API call', () => {
		expect(isUnsafeRefName('')).toBe(true);
		expect(isUnsafeRefName('  ')).toBe(true);
		expect(isUnsafeRefName('HEAD')).toBe(true);
		expect(isUnsafeRefName('feature')).toBe(false);
	});
});

describe('git graph strokes', () => {
	it('draws one continuous rail instead of a line per row', () => {
		const rows = [
			commit({ sha: 'a', lane: 0, lanes: [{ lane: 0, kind: 'commit' }] }),
			commit({ sha: 'b', lane: 0, lanes: [{ lane: 0, kind: 'commit' }] }),
			commit({ sha: 'c', lane: 0, lanes: [{ lane: 0, kind: 'commit' }] })
		];
		const strokes = graphStrokes(rows, false);
		expect(strokes).toHaveLength(1);
		expect(strokes[0].d).toContain('L');
	});

	it('joins uncommitted changes to the next commit circle', () => {
		const rows = [
			commit({ sha: 'head', lane: 0, lanes: [{ lane: 0, kind: 'commit' }] }),
			commit({ sha: 'parent', lane: 0, lanes: [{ lane: 0, kind: 'commit' }] })
		];
		const strokes = graphStrokes(rows, true);
		expect(strokes[0].d).toBe('M8,14 L8,42');
		expect(strokes[0].color).toBe(strokes[1].color);
	});
});

describe('git graph lanes', () => {
	it('counts a side branch column past the merge', () => {
		const rows = [
			commit({
				sha: 'merge',
				lane: 0,
				parents: ['main', 'side'],
				lanes: [
					{ lane: 0, kind: 'merge' },
					{ lane: 1, kind: 'pass' }
				]
			}),
			commit({
				sha: 'side',
				lane: 1,
				parents: ['root'],
				lanes: [
					{ lane: 0, kind: 'pass' },
					{ lane: 1, kind: 'commit' }
				]
			})
		];
		expect(laneColumnCount(rows)).toBe(2);
		expect(rows[1].lane).not.toBe(rows[0].lane);
	});
});
