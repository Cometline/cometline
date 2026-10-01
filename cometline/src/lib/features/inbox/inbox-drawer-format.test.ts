import { describe, expect, it } from 'vitest';
import { formatRelativeTime, previewSnippet } from './inbox-drawer-format';

const MINUTE = 60_000;

describe('formatRelativeTime', () => {
	const now = 1_000 * 24 * 60 * MINUTE;

	it('buckets elapsed time into minutes, hours, and days', () => {
		expect(formatRelativeTime(now - 30_000, now)).toBe('just now');
		expect(formatRelativeTime(now - 5 * MINUTE, now)).toBe('5m ago');
		expect(formatRelativeTime(now - 3 * 60 * MINUTE, now)).toBe('3h ago');
		expect(formatRelativeTime(now - 2 * 24 * 60 * MINUTE, now)).toBe('2d ago');
	});
});

describe('previewSnippet', () => {
	it('collapses whitespace and keeps short bodies intact', () => {
		expect(previewSnippet('  hello\n\n  world  ')).toBe('hello world');
	});

	it('truncates long bodies with an ellipsis at the limit', () => {
		expect(previewSnippet('abcdefghij', 5)).toBe('abcd…');
	});
});
