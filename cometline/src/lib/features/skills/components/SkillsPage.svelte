<script lang="ts">
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import { onMount } from 'svelte';
	import ConfirmActionModal from '#lib/components/ConfirmActionModal.svelte';
	import {
		createSkillsPageController,
		type SkillsTab
	} from '#lib/features/skills/skills-page-controller.svelte.js';
	import SkillDraftsView from './skills-page/SkillDraftsView.svelte';
	import SkillsBrowseView from './skills-page/SkillsBrowseView.svelte';
	import SkillsEmptyState from './skills-page/SkillsEmptyState.svelte';
	import SkillsPageHeader from './skills-page/SkillsPageHeader.svelte';

	let tab = $derived<SkillsTab>(
		page.url.searchParams.get('tab') === 'skills' ? 'skills' : 'drafts'
	);
	const controller = createSkillsPageController({ getTab: () => tab });

	onMount(() => {
		void controller.load();
	});

	function setTab(next: SkillsTab) {
		const params = [...page.url.searchParams].filter(([key]) => key !== 'tab');
		if (next === 'skills') params.push(['tab', 'skills']);
		const search = new URLSearchParams(params).toString();
		// eslint-disable-next-line svelte/no-navigation-without-resolve -- the pathname is resolved; the rule cannot follow the appended query string
		void goto(`${resolve('skills')}${search ? `?${search}` : ''}`, {
			replaceState: true,
			reset: false
		});
	}
</script>

<div class="skills-page settings-ui">
	<SkillsPageHeader
		{tab}
		status={controller.status}
		busy={controller.busy}
		onSelectTab={setTab}
		onRefresh={() => void controller.refreshCurrent({ keepSelection: true })}
	/>

	{#if tab === 'drafts'}
		{#if controller.drafts.length === 0}
			<SkillsEmptyState title="No pending drafts">
				Use <code>/create-skill</code> or enable skill synthesis for completed jobs to create
				drafts here for manual review.
			</SkillsEmptyState>
		{:else}
			<div class="page-layout"><SkillDraftsView {controller} /></div>
		{/if}
	{:else if controller.skills.length === 0}
		<SkillsEmptyState title="No skills discovered">
			Cometline reads skills from <code>~/.cometmind/skills</code>, workspace
			<code>.agents/skills</code>, and other configured roots.
		</SkillsEmptyState>
	{:else}
		<div class="page-layout"><SkillsBrowseView {controller} /></div>
	{/if}
</div>

<ConfirmActionModal
	open={Boolean(controller.pendingSkillName)}
	title="Discard unsaved changes?"
	description={`Switching to ${controller.pendingSkillName || 'another skill'} will discard your unsaved changes.`}
	confirmLabel="Discard"
	onConfirm={controller.discardSkillChanges}
	onCancel={controller.cancelSkillSwitch}
/>

<ConfirmActionModal
	open={Boolean(controller.deletePending)}
	title={`Delete "${controller.deletePending?.name ?? ''}"?`}
	description={controller.deletePending
		? `This removes the original files at ${controller.deletePending.path}. This cannot be undone.`
		: ''}
	confirmLabel="Delete"
	onConfirm={() => void controller.confirmDeleteSkill()}
	onCancel={controller.cancelDeleteSkill}
/>

<style>
	.skills-page {
		display: flex;
		flex-direction: column;
		box-sizing: border-box;
		height: 100%;
		min-height: 0;
		min-width: 0;
		width: 100%;
		max-width: 100%;
		padding: 20px 24px;
		gap: 16px;
		overflow: hidden;
	}

	.page-layout {
		width: 100%;
		min-width: 0;
		display: grid;
		grid-template-columns: minmax(260px, 320px) minmax(0, 1fr);
		gap: 16px;
		min-height: 0;
		flex: 1;
	}

	@media (max-width: 980px) {
		.skills-page {
			padding: 16px;
		}

		.page-layout {
			grid-template-columns: 1fr;
		}
	}

	@container main-pane (max-width: 760px) {
		.skills-page {
			padding: 16px;
		}

		.page-layout {
			grid-template-columns: minmax(0, 1fr);
		}
	}
</style>
