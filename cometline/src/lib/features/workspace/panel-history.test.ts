import { describe, expect, it } from 'vitest';
import {
	canGoBack,
	canGoForward,
	createPanelHistoryState,
	currentEntry,
	goBack,
	goForward,
	pushEntry
} from './panel-history';

describe('panel-history', () => {
	it('seeds browse under the first file push with the given source', () => {
		let state = createPanelHistoryState();
		state = pushEntry(state, { kind: 'file', path: 'a.md' }, 'workspace');
		expect(state.entries).toEqual([
			{ kind: 'browse', source: 'workspace' },
			{ kind: 'file', path: 'a.md' }
		]);
		expect(currentEntry(state)).toEqual({ kind: 'file', path: 'a.md' });
		expect(canGoBack(state)).toBe(true);
	});

	it('does not duplicate the current entry', () => {
		let state = pushEntry(createPanelHistoryState(), { kind: 'browse', source: 'wiki' });
		state = pushEntry(state, { kind: 'browse', source: 'wiki' });
		expect(state.entries).toEqual([{ kind: 'browse', source: 'wiki' }]);
		expect(state.index).toBe(0);
	});

	it('treats different browse sources as distinct history entries', () => {
		let state = pushEntry(createPanelHistoryState(), { kind: 'browse', source: 'wiki' });
		state = pushEntry(state, { kind: 'browse', source: 'workspace' });
		state = pushEntry(state, { kind: 'browse', source: 'changes' });
		expect(state.entries).toEqual([
			{ kind: 'browse', source: 'wiki' },
			{ kind: 'browse', source: 'workspace' },
			{ kind: 'browse', source: 'changes' }
		]);
		state = goBack(state);
		expect(currentEntry(state)).toEqual({ kind: 'browse', source: 'workspace' });
		state = goBack(state);
		expect(currentEntry(state)).toEqual({ kind: 'browse', source: 'wiki' });
	});

	it('truncates forward stack on push', () => {
		let state = pushEntry(createPanelHistoryState(), { kind: 'browse', source: 'wiki' });
		state = pushEntry(state, { kind: 'file', path: 'a.md' });
		state = pushEntry(state, { kind: 'url', url: 'https://example.com' });
		state = goBack(state);
		state = goBack(state);
		expect(currentEntry(state)).toEqual({ kind: 'browse', source: 'wiki' });
		state = pushEntry(state, { kind: 'file', path: 'b.md' });
		expect(state.entries).toEqual([
			{ kind: 'browse', source: 'wiki' },
			{ kind: 'file', path: 'b.md' }
		]);
		expect(canGoForward(state)).toBe(false);
	});

	it('supports back and forward across browse sources, files, git-diff, and urls', () => {
		let state = pushEntry(createPanelHistoryState(), { kind: 'browse', source: 'wiki' });
		state = pushEntry(state, { kind: 'browse', source: 'workspace' });
		state = pushEntry(state, { kind: 'file', path: 'a.md' });
		state = pushEntry(state, { kind: 'browse', source: 'changes' });
		state = pushEntry(state, { kind: 'git-diff', path: 'a.md' });
		state = pushEntry(state, { kind: 'url', url: 'https://example.com' });
		expect(canGoForward(state)).toBe(false);

		state = goBack(state);
		expect(currentEntry(state)).toEqual({ kind: 'git-diff', path: 'a.md' });
		state = goBack(state);
		expect(currentEntry(state)).toEqual({ kind: 'browse', source: 'changes' });
		state = goBack(state);
		expect(currentEntry(state)).toEqual({ kind: 'file', path: 'a.md' });
		state = goBack(state);
		expect(currentEntry(state)).toEqual({ kind: 'browse', source: 'workspace' });
		state = goBack(state);
		expect(currentEntry(state)).toEqual({ kind: 'browse', source: 'wiki' });
		expect(canGoBack(state)).toBe(false);

		state = goForward(state);
		expect(currentEntry(state)).toEqual({ kind: 'browse', source: 'workspace' });
		state = goForward(state);
		expect(currentEntry(state)).toEqual({ kind: 'file', path: 'a.md' });
	});

	it('no-ops back/forward at ends', () => {
		const empty = createPanelHistoryState();
		expect(goBack(empty)).toEqual(empty);
		expect(goForward(empty)).toEqual(empty);

		const browse = pushEntry(empty, { kind: 'browse', source: 'wiki' });
		expect(goBack(browse)).toEqual(browse);
	});
});
