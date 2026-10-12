import type { SkillResource } from '#lib/types.js';

export type SkillTag = {
	id: string;
	label: string;
};

const statusTags = new Set(['stale', 'archived']);

export function skillTags(skill: SkillResource): SkillTag[] {
	const tags: SkillTag[] = [];
	if (skill.origin === 'self-improvement') {
		tags.push({ id: 'origin:self-improvement', label: 'self-improvement' });
	}
	if (skill.status && statusTags.has(skill.status)) {
		tags.push({ id: `status:${skill.status}`, label: skill.status });
	}
	if (skill.pinned) tags.push({ id: 'flag:pinned', label: 'pinned' });
	if (skill.is_symlink) tags.push({ id: 'flag:symlink', label: 'symlink' });
	if (!skill.can_edit) tags.push({ id: 'flag:read-only', label: 'read-only' });
	for (const tag of skill.tags ?? []) {
		const label = tag.trim();
		if (!label) continue;
		tags.push({ id: `tag:${label.toLowerCase()}`, label });
	}
	return tags;
}

export function availableSkillTags(skills: SkillResource[]): SkillTag[] {
	const tags = new Map<string, SkillTag>();
	for (const skill of skills) {
		for (const tag of skillTags(skill)) tags.set(tag.id, tag);
	}
	return [...tags.values()].sort((a, b) => a.label.localeCompare(b.label));
}

export function filterSkills(
	skills: SkillResource[],
	search: string,
	selectedTags: string[] = []
): SkillResource[] {
	const q = search.trim().toLowerCase();
	const selected = new Set(selectedTags);
	return skills.filter((skill) => {
		if (selected.size > 0) {
			const ids = new Set(skillTags(skill).map((tag) => tag.id));
			for (const tag of selected) {
				if (!ids.has(tag)) return false;
			}
		}
		if (!q) return true;
		return (
			skill.name.toLowerCase().includes(q) ||
			skill.description.toLowerCase().includes(q) ||
			skill.path.toLowerCase().includes(q) ||
			(skill.tags ?? []).some((tag) => tag.toLowerCase().includes(q))
		);
	});
}
