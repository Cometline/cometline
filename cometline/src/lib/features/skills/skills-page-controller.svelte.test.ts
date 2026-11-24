import { beforeEach, describe, expect, it, vi } from 'vitest';
import {
	createSkillsPageController,
	nextSkillsTab,
	skillsTabFromQuery
} from './skills-page-controller.svelte.js';
import { skillUpdatesStore } from '#lib/stores/skill-updates.svelte.js';
import {
	getArchivedSkill,
	getSkill,
	listArchivedSkills,
	listSkillDrafts,
	listSkills
} from '#lib/client/cometmind.js';
import type { Skill } from '#lib/generated/cometmind-api/types.gen.js';

vi.mock('#lib/client/cometmind.js', () => ({
	deleteSkill: vi.fn(),
	getArchivedSkill: vi.fn(),
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

describe('skills tabs', () => {
	it('cycles drafts, skills, then archived', () => {
		expect(nextSkillsTab('drafts')).toBe('skills');
		expect(nextSkillsTab('skills')).toBe('archived');
		expect(nextSkillsTab('archived')).toBe('drafts');
	});

	it('reads the archived query without treating an unknown tab as archived', () => {
		expect(skillsTabFromQuery('archived')).toBe('archived');
		expect(skillsTabFromQuery('skills')).toBe('skills');
		expect(skillsTabFromQuery(null)).toBe('drafts');
		expect(skillsTabFromQuery('nope')).toBe('drafts');
	});
});

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

	it('opens the first archived skill in the same preview shape as drafts', async () => {
		const archived = skill('old-skill');
		vi.mocked(listSkills).mockResolvedValue({ skills: [skill('live')] });
		vi.mocked(listArchivedSkills).mockResolvedValue([archived]);
		vi.mocked(getArchivedSkill).mockResolvedValue({
			skill: archived,
			content: '# old-skill'
		});

		const controller = createSkillsPageController({ getTab: () => 'archived' });
		await controller.load();

		expect(controller.archived.map((item) => item.name)).toEqual(['old-skill']);
		expect(controller.selectedArchivedName).toBe('old-skill');
		expect(controller.archivedContent).toBe('# old-skill');
		expect(getArchivedSkill).toHaveBeenCalledWith('old-skill');
	});
});
