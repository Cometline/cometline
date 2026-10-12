import { describe, expect, it } from 'vitest';
import type { SkillResource } from '#lib/types.js';
import { availableSkillTags, filterSkills } from './skills-page-filter';

function skill(
	name: string,
	description: string,
	path: string,
	extra: Partial<SkillResource> = {}
): SkillResource {
	return {
		name,
		description,
		path,
		source: path,
		internal: false,
		is_symlink: false,
		can_delete: false,
		can_export: true,
		can_edit: true,
		...extra
	};
}

const skills = [
	skill('alpha', 'Writes release notes', '/skills/alpha', { tags: ['release'] }),
	skill('beta', 'Reviews code', '/workspace/.agents/skills/beta', {
		origin: 'self-improvement',
		tags: ['review', 'go']
	}),
	skill('gamma', 'Old notes', '/skills/gamma', { status: 'stale', tags: ['review'] }),
	skill('delta', 'Linked copy', '/skills/delta', { is_symlink: true, can_edit: false })
];

describe('filterSkills', () => {
	it('returns every skill for a blank query', () => {
		expect(filterSkills(skills, '   ')).toEqual(skills);
	});

	it('matches name, description, path, or tag case-insensitively', () => {
		expect(filterSkills(skills, 'ALPHA').map((s) => s.name)).toEqual(['alpha']);
		expect(filterSkills(skills, 'Reviews code').map((s) => s.name)).toEqual(['beta']);
		expect(filterSkills(skills, '.agents').map((s) => s.name)).toEqual(['beta']);
		expect(filterSkills(skills, 'RELEASE').map((s) => s.name)).toEqual(['alpha']);
	});

	it('filters by selected tags, including origin and status', () => {
		expect(filterSkills(skills, '', ['tag:review']).map((s) => s.name)).toEqual(['beta', 'gamma']);
		expect(filterSkills(skills, '', ['origin:self-improvement']).map((s) => s.name)).toEqual([
			'beta'
		]);
		expect(filterSkills(skills, 'code', ['tag:go']).map((s) => s.name)).toEqual(['beta']);
		expect(filterSkills(skills, '', ['tag:review', 'status:stale']).map((s) => s.name)).toEqual([
			'gamma'
		]);
		expect(filterSkills(skills, '', ['flag:symlink', 'flag:read-only']).map((s) => s.name)).toEqual([
			'delta'
		]);
		expect(filterSkills(skills, '', ['flag:untagged']).map((s) => s.name)).toEqual(['delta']);
	});

	it('returns no skills when nothing matches', () => {
		expect(filterSkills(skills, 'missing')).toEqual([]);
	});

	it('lists unique tags sorted by label', () => {
		expect(availableSkillTags(skills).map((tag) => tag.label)).toEqual([
			'go',
			'read-only',
			'release',
			'review',
			'self-improvement',
			'stale',
			'symlink',
			'untagged'
		]);
	});
});
