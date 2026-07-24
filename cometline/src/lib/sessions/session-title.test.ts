import { describe, expect, it } from 'vitest';
import { DEFAULT_SESSION_TITLE, sessionDisplayTitle } from './session-title';

describe('sessionDisplayTitle', () => {
	it('returns New Chat for empty titles', () => {
		expect(sessionDisplayTitle('')).toBe(DEFAULT_SESSION_TITLE);
		expect(sessionDisplayTitle('   ')).toBe(DEFAULT_SESSION_TITLE);
		expect(sessionDisplayTitle(null)).toBe(DEFAULT_SESSION_TITLE);
		expect(sessionDisplayTitle(undefined)).toBe(DEFAULT_SESSION_TITLE);
	});

	it('returns the trimmed title when set', () => {
		expect(sessionDisplayTitle(' Hello ')).toBe('Hello');
	});
});
