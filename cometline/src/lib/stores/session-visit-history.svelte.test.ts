import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import {
	LAST_VISITED_SESSION_STORAGE_KEY,
	sessionVisitHistory
} from './session-visit-history.svelte';

describe('sessionVisitHistory', () => {
	let stored: Record<string, string>;

	beforeEach(() => {
		stored = {};
		vi.stubGlobal('window', {
			localStorage: {
				getItem: (key: string) => stored[key] ?? null,
				setItem: (key: string, value: string) => {
					stored[key] = value;
				},
				removeItem: (key: string) => {
					delete stored[key];
				}
			}
		});
		sessionVisitHistory.reset();
	});

	afterEach(() => {
		vi.unstubAllGlobals();
	});

	it('records visits and truncates the forward stack', () => {
		sessionVisitHistory.recordVisit('a');
		sessionVisitHistory.recordVisit('b');
		sessionVisitHistory.recordVisit('c');
		expect(sessionVisitHistory.goBack(() => true)).toBe('b');
		sessionVisitHistory.recordVisit('d');
		expect(sessionVisitHistory.stack).toEqual(['a', 'b', 'd']);
		expect(sessionVisitHistory.goForward(() => true)).toBeNull();
	});

	it('ignores duplicate consecutive visits', () => {
		sessionVisitHistory.recordVisit('a');
		sessionVisitHistory.recordVisit('a');
		expect(sessionVisitHistory.stack).toEqual(['a']);
		expect(sessionVisitHistory.index).toBe(0);
	});

	it('skips and prunes missing sessions when going back', () => {
		sessionVisitHistory.recordVisit('a');
		sessionVisitHistory.recordVisit('gone');
		sessionVisitHistory.recordVisit('c');
		const exists = (id: string) => id !== 'gone';
		expect(sessionVisitHistory.goBack(exists)).toBe('a');
		expect(sessionVisitHistory.stack).toEqual(['a', 'c']);
	});

	it('skips and prunes missing sessions when going forward', () => {
		sessionVisitHistory.recordVisit('a');
		sessionVisitHistory.recordVisit('b');
		sessionVisitHistory.recordVisit('gone');
		sessionVisitHistory.recordVisit('d');
		expect(sessionVisitHistory.goBack(() => true)).toBe('gone');
		expect(sessionVisitHistory.goBack(() => true)).toBe('b');
		const exists = (id: string) => id !== 'gone';
		expect(sessionVisitHistory.goForward(exists)).toBe('d');
		expect(sessionVisitHistory.stack).toEqual(['a', 'b', 'd']);
	});

	it('returns the current visit without moving history', () => {
		sessionVisitHistory.recordVisit('a');
		sessionVisitHistory.recordVisit('b');
		sessionVisitHistory.recordVisit('c');
		expect(sessionVisitHistory.mostRecent(() => true)).toBe('c');
		expect(sessionVisitHistory.goBack(() => true)).toBe('b');
		expect(sessionVisitHistory.mostRecent(() => true)).toBe('b');
		expect(sessionVisitHistory.index).toBe(1);
	});

	it('skips missing sessions when peeking most recent', () => {
		sessionVisitHistory.recordVisit('a');
		sessionVisitHistory.recordVisit('gone');
		const exists = (id: string) => id !== 'gone';
		expect(sessionVisitHistory.mostRecent(exists)).toBe('a');
		expect(sessionVisitHistory.stack).toEqual(['a', 'gone']);
		expect(sessionVisitHistory.index).toBe(1);
	});

	it('restores the last visit from browser storage after a restart', () => {
		sessionVisitHistory.recordVisit('persisted');
		expect(stored[LAST_VISITED_SESSION_STORAGE_KEY]).toBe('persisted');

		sessionVisitHistory.reset();
		expect(sessionVisitHistory.mostRecent((id) => id === 'persisted')).toBe('persisted');
	});

	it('persists history navigation without changing the visit stack', () => {
		sessionVisitHistory.recordVisit('a');
		sessionVisitHistory.recordVisit('b');
		expect(sessionVisitHistory.goBack(() => true)).toBe('a');

		sessionVisitHistory.markActive('a');
		expect(sessionVisitHistory.stack).toEqual(['a', 'b']);
		expect(sessionVisitHistory.index).toBe(0);
		expect(stored[LAST_VISITED_SESSION_STORAGE_KEY]).toBe('a');
	});

	it('clears a stale persisted visit', () => {
		stored[LAST_VISITED_SESSION_STORAGE_KEY] = 'gone';

		expect(sessionVisitHistory.mostRecent(() => false)).toBeNull();
		expect(stored[LAST_VISITED_SESSION_STORAGE_KEY]).toBeUndefined();
	});
});
