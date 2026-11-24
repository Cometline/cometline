<script lang="ts">
	import type { SkillsPageController } from '#lib/features/skills/skills-page-controller.svelte.js';
	import type { SkillResource } from '#lib/types.js';
	import { skillExpiryLabel } from '#lib/features/skills/skill-expiry.js';
	import SkillsListPanel from './SkillsListPanel.svelte';
	import SkillsListRow from './SkillsListRow.svelte';
	import SkillsPreviewPanel from './SkillsPreviewPanel.svelte';

	let { controller }: { controller: SkillsPageController } = $props();

	function badgesFor(skill: SkillResource): string[] {
		const badges: string[] = [];
		if (skill.origin === 'self-improvement') badges.push('self-improvement');
		const expiry = skillExpiryLabel(skill.expires_at, 'archived', Date.now(), skill.pinned);
		if (expiry) badges.push(expiry);
		return badges;
	}

	function previewNotice(): string {
		const skill = controller.selectedArchived?.skill;
		if (!skill) return '';
		const expiry = skillExpiryLabel(skill.expires_at, 'archived', Date.now(), skill.pinned);
		if (expiry) return `Archived. This skill ${expiry}.`;
		return 'Archived. Restore it to edit and use it again.';
	}
</script>

<SkillsListPanel title="Archived" countLabel={String(controller.archived.length)}>
	{#each controller.archived as skill (skill.name)}
		<SkillsListRow
			name={skill.name}
			description={skill.description}
			active={controller.selectedArchivedName === skill.name}
			badges={badgesFor(skill)}
			onclick={() => void controller.openArchived(skill.name)}
		/>
	{/each}
</SkillsListPanel>

<SkillsPreviewPanel
	loading={controller.contentBusy}
	loadingLabel="Loading archived skill…"
	emptyLabel="Select an archived skill to preview it."
	item={controller.selectedArchived?.skill ?? null}
	path={controller.selectedArchived?.skill.path}
	notice={previewNotice()}
	readonly
	value={controller.archivedContent}
>
	{#snippet actions()}
		<button
			type="button"
			class="primary"
			disabled={controller.busy || !controller.selectedArchivedId}
			onclick={() => void controller.restoreArchived(controller.selectedArchivedId)}
		>
			Restore
		</button>
	{/snippet}
</SkillsPreviewPanel>
