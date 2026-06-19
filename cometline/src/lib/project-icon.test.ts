import { describe, expect, it } from 'vitest';
import {
	normalizeIconVariant,
	projectAvatarSrc,
	projectAvatarSrcset,
	systemSoulFilename
} from './project-icon';

describe('project-icon', () => {
	it('builds default avatar paths', () => {
		expect(projectAvatarSrc('default', 96)).toBe('/project_avatar_96.png');
		expect(projectAvatarSrcset('default')).toContain('/project_avatar_384.png 384w');
	});

	it('builds man avatar paths', () => {
		expect(projectAvatarSrc('man', 192)).toBe('/project_avatar_man_192.png');
		expect(projectAvatarSrcset('man')).toContain('/project_avatar_man_96.png 96w');
	});

	it('normalizes unknown variants to default', () => {
		expect(normalizeIconVariant('man')).toBe('man');
		expect(normalizeIconVariant('other')).toBe('default');
	});

	it('maps icon variants to SOUL filenames', () => {
		expect(systemSoulFilename('default')).toBe('SOUL.md');
		expect(systemSoulFilename('man')).toBe('SOUL_MAN.md');
	});
});
