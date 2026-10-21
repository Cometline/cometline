import { beforeEach, describe, expect, it, vi } from 'vitest';
import { createSkillsPageController } from './skills-page-controller.svelte.js';
import { skillUpdatesStore } from '#lib/stores/skill-updates.svelte.js';
import {
	getSkill,
	listArchivedSkills,
	listSkillDrafts,
	listSkills
} from '#lib/client/cometmind.js';
import type { Skill } from '#lib/generated/cometmind-api/types.gen.js';

vi.mock('#lib/client/cometmind.js', () => ({
	deleteSkill: vi.fn(),
	getSkill: vi.fn(),
	getSkillDraft: vi.fn(),
	listArchivedSkills: vi.fn(),
	listSkillDrafts: vi.fn(),
	listSkills: vi.fn(),
	pinSkill: vi.fn(),
	restoreArchivedSkill: vi.fn(),
	promoteSkillDraft: vi.fn(),
	rejectSkillDraft: vi.fn(),
	updateSkill: vi.fn(),
	updateSkillDraft: vi.fn()
}));

function skill(name: string): Skill {
	return {
		name,
		description: `${name} description`,
		path: `/skills/${name}`,
		source: `/skills/${name}`,
		internal: false,
		is_symlink: false,
		can_delete: true,
		can_export: true,
		can_edit: true
	};
}

describe('createSkillsPageController', () => {
	beforeEach(() => {
		vi.clearAllMocks();
		vi.mocked(listSkillDrafts).mockResolvedValue([]);
		vi.mocked(listArchivedSkills).mockResolvedValue([]);
		vi.mocked(getSkill).mockImplementation(async (name) => ({
			skill: skill(name),
			content: `# ${name}`
		}));
	});

	it('opens the newest recent skill instead of the alphabetically first skill', async () => {
		vi.mocked(listSkills).mockResolvedValue({
			skills: [
				skill('alpha'),
				skill('bulk-clone-github-org-repos'),
				skill('trace-derived-field-header-source')
			]
		});
		skillUpdatesStore.note([
			'trace-derived-field-header-source',
			'schema-field-drop-by-usage',
			'bulk-clone-github-org-repos'
		]);

		const controller = createSkillsPageController({ getTab: () => 'skills' });
		await controller.load();

		expect(controller.filteredSkills.map((item) => item.name)).toEqual([
			'trace-derived-field-header-source',
			'bulk-clone-github-org-repos',
			'alpha'
		]);
		expect(controller.selectedSkillName).toBe('trace-derived-field-header-source');
		expect(getSkill).toHaveBeenCalledWith(
			'trace-derived-field-header-source',
			expect.any(String)
		);
	});
});
