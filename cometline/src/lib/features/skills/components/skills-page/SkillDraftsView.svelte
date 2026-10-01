<script lang="ts">
	import type { SkillsPageController } from '$lib/features/skills/skills-page-controller.svelte';
	import SkillsListPanel from './SkillsListPanel.svelte';
	import SkillsListRow from './SkillsListRow.svelte';
	import SkillsPreviewPanel from './SkillsPreviewPanel.svelte';

	let { controller }: { controller: SkillsPageController } = $props();
</script>

<SkillsListPanel title="Pending" countLabel={String(controller.drafts.length)}>
	{#each controller.drafts as draft (draft.name)}
		<SkillsListRow
			name={draft.name}
			description={draft.description}
			active={controller.selectedDraftName === draft.name}
			onclick={() => void controller.openDraft(draft.name)}
		/>
	{/each}
</SkillsListPanel>

<SkillsPreviewPanel
	loading={controller.contentBusy}
	loadingLabel="Loading draft…"
	emptyLabel="Select a draft to preview it."
	item={controller.selectedDraft?.draft ?? null}
	bind:value={controller.draftContent}
>
	{#snippet actions()}
		<button
			type="button"
			class="secondary"
			disabled={controller.saveBusy || controller.busy || !controller.draftDirty}
			onclick={() => void controller.saveDraft(controller.selectedDraftId)}
		>
			{controller.saveBusy ? 'Saving...' : 'Save'}
		</button>
		<button
			type="button"
			class="secondary"
			disabled={controller.busy || controller.saveBusy}
			onclick={() => void controller.rejectDraft(controller.selectedDraftId)}
		>
			Reject
		</button>
		<button
			type="button"
			class="primary"
			disabled={controller.busy || controller.saveBusy || controller.draftDirty}
			onclick={() => void controller.promoteDraft(controller.selectedDraftId)}
		>
			Promote
		</button>
	{/snippet}
</SkillsPreviewPanel>
