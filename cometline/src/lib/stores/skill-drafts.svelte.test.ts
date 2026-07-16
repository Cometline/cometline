import { beforeEach, describe, expect, it, vi } from 'vitest';

vi.mock('$lib/client/cometmind', () => ({
	listSkillDrafts: vi.fn(async () => [{ name: 'draft-a' }, { name: 'draft-b' }])
}));

describe('skillDraftsStore', () => {
	beforeEach(() => {
		vi.resetModules();
	});

	it('refreshes draft count from the API', async () => {
		const { skillDraftsStore } = await import('./skill-drafts.svelte');
		await skillDraftsStore.refresh();
		expect(skillDraftsStore.count).toBe(2);
		expect(skillDraftsStore.hasDrafts).toBe(true);
	});

	it('updates count after promote/reject via setCount', async () => {
		const { skillDraftsStore } = await import('./skill-drafts.svelte');
		skillDraftsStore.setCount(0);
		expect(skillDraftsStore.hasDrafts).toBe(false);
		skillDraftsStore.setCount(3);
		expect(skillDraftsStore.count).toBe(3);
	});
});
