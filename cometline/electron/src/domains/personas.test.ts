import { describe, expect, it } from 'vitest';

import {
	decodePersonaAvatarDataUrl,
	nextCustomPersonaId,
	normalizePersonaSlug
} from './personas.js';

describe('persona input validation', () => {
	it('normalizes custom IDs into traversal-safe slugs', () => {
		expect(normalizePersonaSlug(' ../My Persona/../../ ')).toBe('my-persona');
		expect(normalizePersonaSlug('...')).toBe('');
		expect(normalizePersonaSlug('a'.repeat(60))).toHaveLength(48);
	});

	it('accepts only PNG, JPEG, and WebP avatar data URLs', () => {
		expect(decodePersonaAvatarDataUrl('data:image/png;base64,YQ==')).toMatchObject({
			ext: '.png',
			buffer: Buffer.from('a')
		});
		expect(decodePersonaAvatarDataUrl('data:image/jpeg;base64,YQ==')).toMatchObject({ ext: '.jpg' });
		expect(decodePersonaAvatarDataUrl('data:image/webp;base64,YQ==')).toMatchObject({ ext: '.webp' });
		expect(decodePersonaAvatarDataUrl('data:image/jpg;base64,YQ==')).toBeNull();
		expect(decodePersonaAvatarDataUrl('data:image/gif;base64,YQ==')).toBeNull();
		expect(decodePersonaAvatarDataUrl('data:image/png;base64,not valid')).toBeNull();
	});

	it('rejects avatars larger than the 20 MB limit', () => {
		const payload = Buffer.alloc(20 * 1024 * 1024 + 1).toString('base64');
		expect(() => decodePersonaAvatarDataUrl(`data:image/png;base64,${payload}`)).toThrow(
			'Avatar image exceeds 20 MB limit'
		);
	});

	it('creates a stable, collision-free ID for an unnamed custom persona', () => {
		expect(
			nextCustomPersonaId('', 'My Persona', [{ id: 'my-persona' }], () => 1234)
		).toBe('my-persona-2');
		expect(nextCustomPersonaId('preferred', 'My Persona', [{ id: 'preferred' }])).toBe('preferred');
	});
});
