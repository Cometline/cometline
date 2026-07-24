import { describe, expect, it } from 'vitest';
import { isHttpUrl, normalizeUserUrl } from './http-url';

describe('http-url', () => {
	it('accepts http and https URLs', () => {
		expect(isHttpUrl('https://example.com')).toBe(true);
		expect(isHttpUrl('http://localhost:5173')).toBe(true);
		expect(isHttpUrl('mailto:test@example.com')).toBe(false);
	});

	it('normalizes user-typed URLs with https prefix', () => {
		expect(normalizeUserUrl('youtube.com/watch?v=abc')).toBe('https://youtube.com/watch?v=abc');
		expect(normalizeUserUrl('https://example.com/path')).toBe('https://example.com/path');
		expect(normalizeUserUrl('localhost:5173')).toBe('https://localhost:5173/');
	});

	it('treats bare words and invalid hosts as search queries', () => {
		expect(normalizeUserUrl('facebook')).toBe('https://www.google.com/search?q=facebook');
		expect(normalizeUserUrl('not a url!!!')).toBe(
			'https://www.google.com/search?q=not%20a%20url!!!'
		);
	});

	it('rejects empty input', () => {
		expect(normalizeUserUrl('')).toBeNull();
		expect(normalizeUserUrl('   ')).toBeNull();
	});
});
