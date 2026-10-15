<script lang="ts">
	import type { SkillsPageController } from '#lib/features/skills/skills-page-controller.svelte.js';
	import type { SkillResource } from '#lib/types.js';
	import { skillUpdatesStore } from '#lib/stores/skill-updates.svelte.js';
	import SkillsListPanel from './SkillsListPanel.svelte';
	import SkillsListRow from './SkillsListRow.svelte';
	import SkillsPreviewPanel from './SkillsPreviewPanel.svelte';

	let { controller }: { controller: SkillsPageController } = $props();

	let countLabel = $derived(
		controller.filteredSkills.length === controller.skills.length
			? String(controller.skills.length)
			: `${controller.filteredSkills.length} / ${controller.skills.length}`
	);

	function badgesFor(skill: SkillResource): string[] {
		const badges: string[] = [];
		if (skill.origin === 'self-improvement') badges.push('self-improvement');
		if (skill.status === 'stale') badges.push('stale');
		if (skill.pinned) badges.push('pinned');
		if (skill.is_symlink) badges.push('symlink');
		if (!skill.can_edit) badges.push('read-only');
		return badges;
	}
</script>

<SkillsListPanel title="Available" {countLabel}>
	{#snippet toolbar()}
		<input
			class="skill-search"
			type="search"
			bind:value={controller.skillSearch}
			placeholder="Search skills by name, description, path, or tag…"
			spellcheck="false"
		/>
		{#if controller.skillTagOptions.length > 0}
			<div class="tag-filters" role="group" aria-label="Filter by tag">
				{#each controller.skillTagOptions as tag (tag.id)}
					<button
						type="button"
						class="tag-filter"
						class:active={controller.selectedTags.includes(tag.id)}
						aria-pressed={controller.selectedTags.includes(tag.id)}
						onclick={() => controller.toggleTag(tag.id)}
					>
						{tag.label}
					</button>
				{/each}
			</div>
		{/if}
	{/snippet}
	{#if controller.filteredSkills.length === 0}
		<p class="page-muted skill-empty">No skills match your search or tags.</p>
	{:else}
		{#each controller.filteredSkills as skill (skill.name)}
			<SkillsListRow
				name={skill.name}
				description={skill.description}
				active={controller.selectedSkillName === skill.name}
				unseen={skillUpdatesStore.unseen.includes(skill.name)}
				badges={badgesFor(skill)}
				onclick={() => void controller.openSkill(skill.name)}
			/>
		{/each}
	{/if}
	{#snippet footer()}
		{#if controller.skillErrors.length > 0}
			<div class="skill-errors">
				{#each controller.skillErrors as error, i (i)}
					<p>{error}</p>
				{/each}
			</div>
		{/if}
	{/snippet}
</SkillsListPanel>

<SkillsPreviewPanel
	loading={controller.contentBusy}
	loadingLabel="Loading skill…"
	emptyLabel="Select a skill to preview it."
	item={controller.selectedSkill?.skill ?? null}
	path={controller.selectedSkill?.skill.path}
	notice={controller.canEditSkill ? '' : 'This bundled skill is read-only.'}
	readonly={!controller.canEditSkill}
	bind:value={controller.skillContent}
>
	{#snippet actions()}
		<button
			type="button"
			class="secondary"
			disabled={controller.saveBusy ||
				controller.busy ||
				!controller.skillDirty ||
				!controller.canEditSkill}
			onclick={() => void controller.saveSkill(controller.selectedSkillId)}
		>
			{controller.saveBusy ? 'Saving...' : 'Save'}
		</button>
		{#if controller.selectedSkill?.skill.origin === 'self-improvement'}
			<button
				type="button"
				class="secondary"
				disabled={controller.busy}
				onclick={() =>
					void controller.togglePin(
						controller.selectedSkillId,
						!controller.selectedSkill?.skill.pinned
					)}
			>
				{controller.selectedSkill?.skill.pinned ? 'Unpin' : 'Pin'}
			</button>
		{/if}
		{#if controller.canDeleteSkill}
			<button
				type="button"
				class="secondary danger"
				disabled={controller.busy || controller.saveBusy}
				title={`Delete ${controller.selectedSkill?.skill.path}`}
				onclick={controller.requestDeleteSelectedSkill}
			>
				Delete
			</button>
		{/if}
	{/snippet}
</SkillsPreviewPanel>

{#if controller.archived.length > 0}
	<section class="archived-skills">
		<h2>Archived</h2>
		{#each controller.archived as skill (skill.name)}
			<div class="archived-row">
				<span>{skill.name}</span>
				<button
					type="button"
					class="secondary"
					onclick={() => void controller.restoreArchived(skill.name)}
				>
					Restore
				</button>
			</div>
		{/each}
	</section>
{/if}

<style>
	.archived-skills {
		grid-column: 1 / -1;
		margin-top: 12px;
	}

	.archived-row {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 8px;
		margin-top: 6px;
		color: var(--text-muted);
		font-size: 12px;
	}

	.page-muted {
		margin: 6px 0 0;
		font-size: 12px;
		line-height: 1.5;
		color: var(--text-muted);
	}

	.tag-filters {
		--chip-row: calc(1lh + 8px);
		display: flex;
		flex: 0 0 auto;
		flex-wrap: wrap;
		align-content: flex-start;
		gap: 6px;
		min-height: var(--chip-row);
		max-height: calc(3 * var(--chip-row) + 12px);
		margin: -2px 0 8px;
		padding-right: 2px;
		overflow-y: auto;
		overscroll-behavior: contain;
		scrollbar-width: thin;
		scrollbar-color: rgba(15, 23, 42, 0.28) transparent;
	}

	.tag-filters::-webkit-scrollbar {
		width: 6px;
	}

	.tag-filters::-webkit-scrollbar-thumb {
		border-radius: 999px;
		background: rgba(15, 23, 42, 0.28);
	}

	.tag-filter {
		border: 1px solid var(--border-soft);
		border-radius: 999px;
		background: transparent;
		color: var(--text-muted);
		font: inherit;
		font-size: 11px;
		line-height: 1.2;
		font-weight: 700;
		padding: 3px 8px;
		cursor: pointer;
	}

	.tag-filter.active {
		background: rgba(0, 102, 204, 0.1);
		border-color: rgba(0, 102, 204, 0.28);
		color: var(--text-main);
	}

	.skill-search {
		flex: 0 0 auto;
		margin-bottom: 8px;
		padding: 8px 10px;
		border: 1px solid var(--border-soft);
		border-radius: 10px;
		background: var(--app-bg);
		color: var(--text-main);
		font: inherit;
		font-size: 12px;
	}

	.skill-empty,
	.skill-errors {
		padding: 10px 12px;
	}

	.skill-errors p {
		margin: 0 0 6px;
		font-size: 11px;
		color: var(--text-muted);
	}
</style>
