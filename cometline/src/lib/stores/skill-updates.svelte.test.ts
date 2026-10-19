import { beforeEach, describe, expect, it, vi } from 'vitest';

describe('skillUpdatesStore', () => {
	beforeEach(() => {
		const store = new Map<string, string>();
		vi.stubGlobal('localStorage', {
			getItem: (key: string) => store.get(key) ?? null,
			setItem: (key: string, value: string) => store.set(key, value),
			clear: () => store.clear()
		});
	});

	it('keeps a badge until the updated skills are marked seen', async () => {
		vi.resetModules();
		const { skillUpdatesStore } = await import('./skill-updates.svelte');
		skillUpdatesStore.note(['ship-checklist', 'ship-checklist', '']);
		expect(skillUpdatesStore.hasUnseen).toBe(true);
		expect(skillUpdatesStore.unseen).toEqual(['ship-checklist']);

		skillUpdatesStore.markSeen();
		expect(skillUpdatesStore.hasUnseen).toBe(false);

		skillUpdatesStore.note(['ship-checklist']);
		expect(skillUpdatesStore.hasUnseen).toBe(false);

		skillUpdatesStore.note(['ship-checklist', 'retry-pattern']);
		expect(skillUpdatesStore.unseen).toEqual(['retry-pattern']);
		expect(skillUpdatesStore.recent).toEqual(['ship-checklist', 'retry-pattern']);
		skillUpdatesStore.markSeen();
		expect(skillUpdatesStore.hasUnseen).toBe(false);
		expect(skillUpdatesStore.recent).toEqual(['ship-checklist', 'retry-pattern']);
	});
});
