import { describe, expect, it } from 'vitest';
import { skillExpiryLabel } from './skill-expiry';

const now = Date.parse('2026-10-08T00:00:00Z');
const day = 24 * 60 * 60 * 1000;

describe('skillExpiryLabel', () => {
	it('says when an active skill archives', () => {
		expect(skillExpiryLabel(now + 20 * day, 'active', now)).toBe('archives in 20 days');
		expect(skillExpiryLabel(now + day, 'stale', now)).toBe('archives in 1 day');
	});

	it('says when an archived skill deletes', () => {
		expect(skillExpiryLabel(now + 7 * day, 'archived', now)).toBe('deletes in 7 days');
		expect(skillExpiryLabel(now - day, 'archived', now)).toBe('deletes today');
	});

	it('is empty when the skill does not expire', () => {
		expect(skillExpiryLabel(undefined, 'active', now)).toBe('');
		expect(skillExpiryLabel(0, 'active', now)).toBe('');
		expect(skillExpiryLabel(now + day, 'active', now, true)).toBe('');
	});
});
