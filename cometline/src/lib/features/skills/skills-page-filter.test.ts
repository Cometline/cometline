import { describe, expect, it } from 'vitest';
import type { SkillResource } from '#lib/types.js';
import { filterSkills } from './skills-page-filter';

function skill(name: string, description: string, path: string): SkillResource {
	return {
		name,
		description,
		path,
		source: path,
		internal: false,
		is_symlink: false,
		can_delete: false,
		can_export: true,
		can_edit: true
	};
}

const skills = [
	skill('alpha', 'Writes release notes', '/skills/alpha'),
	skill('beta', 'Reviews code', '/workspace/.agents/skills/beta')
];

describe('filterSkills', () => {
	it('returns every skill for a blank query', () => {
		expect(filterSkills(skills, '   ')).toBe(skills);
	});

	it('matches name, description, or path case-insensitively', () => {
		expect(filterSkills(skills, 'ALPHA').map((s) => s.name)).toEqual(['alpha']);
		expect(filterSkills(skills, 'review').map((s) => s.name)).toEqual(['beta']);
		expect(filterSkills(skills, '.agents').map((s) => s.name)).toEqual(['beta']);
	});

	it('returns no skills when nothing matches', () => {
		expect(filterSkills(skills, 'missing')).toEqual([]);
	});
});
