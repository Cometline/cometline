import { beforeEach, describe, expect, it } from 'vitest';
import { sessionVisitHistory } from './session-visit-history.svelte';

describe('sessionVisitHistory', () => {
	beforeEach(() => {
		sessionVisitHistory.reset();
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
});
