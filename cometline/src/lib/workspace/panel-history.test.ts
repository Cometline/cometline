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
	it('seeds browse under the first file push', () => {
		let state = createPanelHistoryState();
		state = pushEntry(state, { kind: 'file', path: 'a.md' });
		expect(state.entries).toEqual([{ kind: 'browse' }, { kind: 'file', path: 'a.md' }]);
		expect(currentEntry(state)).toEqual({ kind: 'file', path: 'a.md' });
		expect(canGoBack(state)).toBe(true);
	});

	it('does not duplicate the current entry', () => {
		let state = pushEntry(createPanelHistoryState(), { kind: 'browse' });
		state = pushEntry(state, { kind: 'browse' });
		expect(state.entries).toEqual([{ kind: 'browse' }]);
		expect(state.index).toBe(0);
	});

	it('truncates forward stack on push', () => {
		let state = pushEntry(createPanelHistoryState(), { kind: 'browse' });
		state = pushEntry(state, { kind: 'file', path: 'a.md' });
		state = pushEntry(state, { kind: 'url', url: 'https://example.com' });
		state = goBack(state);
		state = goBack(state);
		expect(currentEntry(state)).toEqual({ kind: 'browse' });
		state = pushEntry(state, { kind: 'file', path: 'b.md' });
		expect(state.entries).toEqual([{ kind: 'browse' }, { kind: 'file', path: 'b.md' }]);
		expect(canGoForward(state)).toBe(false);
	});

	it('supports back and forward across browse/file/url', () => {
		let state = pushEntry(createPanelHistoryState(), { kind: 'browse' });
		state = pushEntry(state, { kind: 'file', path: 'a.md' });
		state = pushEntry(state, { kind: 'url', url: 'https://example.com' });
		expect(canGoForward(state)).toBe(false);

		state = goBack(state);
		expect(currentEntry(state)).toEqual({ kind: 'file', path: 'a.md' });
		state = goBack(state);
		expect(currentEntry(state)).toEqual({ kind: 'browse' });
		expect(canGoBack(state)).toBe(false);

		state = goForward(state);
		expect(currentEntry(state)).toEqual({ kind: 'file', path: 'a.md' });
		state = goForward(state);
		expect(currentEntry(state)).toEqual({ kind: 'url', url: 'https://example.com' });
	});

	it('no-ops back/forward at ends', () => {
		const empty = createPanelHistoryState();
		expect(goBack(empty)).toEqual(empty);
		expect(goForward(empty)).toEqual(empty);

		const browse = pushEntry(empty, { kind: 'browse' });
		expect(goBack(browse)).toEqual(browse);
	});
});
