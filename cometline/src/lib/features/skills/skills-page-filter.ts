import type { SkillResource } from '#lib/types.js';

export function filterSkills(skills: SkillResource[], search: string): SkillResource[] {
	const q = search.trim().toLowerCase();
	if (!q) return skills;
	return skills.filter((skill) => {
		return (
			skill.name.toLowerCase().includes(q) ||
			skill.description.toLowerCase().includes(q) ||
			skill.path.toLowerCase().includes(q)
		);
	});
}
