<script lang="ts">
	import type { SkillsTab } from '#lib/features/skills/skills-page-controller.svelte.js';
	import { skillDraftsStore } from '#lib/stores/skill-drafts.svelte.js';

	let {
		tab,
		status,
		busy,
		onSelectTab,
		onRefresh
	}: {
		tab: SkillsTab;
		status: string;
		busy: boolean;
		onSelectTab: (tab: SkillsTab) => void;
		onRefresh: () => void;
	} = $props();
</script>

<header class="page-header">
	<div class="page-copy">
		{#if tab === 'drafts'}
			<p>
				Review and edit reusable skills drafted from <code>/create-skill</code> or completed
				jobs. Drafts stay inactive until you promote them into
				<code>~/.cometmind/skills</code>.
			</p>
		{:else}
			<p>
				Browse and edit Agent Skills Cometline can use. Changes write to the skill's
				<code>SKILL.md</code> in place. Delete removes managed skills and global originals
				under <code>~/.agents</code>, OpenCode, or Claude. Workspace and bundled skills
				stay.
			</p>
		{/if}
		{#if status}
			<p class="page-status">{status}</p>
		{/if}
	</div>
	<div class="page-header-actions">
		<div class="view-toggle" role="group" aria-label="Switch view">
			<button
				type="button"
				class="view-btn"
				class:active={tab === 'drafts'}
				aria-pressed={tab === 'drafts'}
				onclick={() => onSelectTab('drafts')}
			>
				Drafts
				{#if skillDraftsStore.hasDrafts}
					<span>{skillDraftsStore.count}</span>
				{/if}
			</button>
			<button
				type="button"
				class="view-btn"
				class:active={tab === 'skills'}
				aria-pressed={tab === 'skills'}
				onclick={() => onSelectTab('skills')}
			>
				Skills
			</button>
		</div>
		<button class="secondary" type="button" onclick={onRefresh}>
			{busy ? 'Loading...' : tab === 'drafts' ? 'Refresh drafts' : 'Refresh skills'}
		</button>
	</div>
</header>

<style>
	.page-header {
		width: 100%;
		min-width: 0;
		display: flex;
		flex-wrap: wrap;
		justify-content: space-between;
		gap: 12px 16px;
		align-items: flex-start;
	}

	.page-copy {
		min-width: 0;
		flex: 1;
	}

	.page-header p {
		min-width: 0;
		margin: 0;
		padding-left: 14px;
		font-size: 12px;
		line-height: 1.5;
		color: var(--text-muted);
	}

	.page-header-actions {
		display: flex;
		flex-wrap: wrap;
		gap: 8px;
		align-items: center;
	}

	.view-toggle {
		display: inline-flex;
		align-items: center;
		gap: 4px;
		padding: 3px;
		border-radius: 999px;
		background: rgba(15, 23, 42, 0.05);
	}

	.view-btn {
		border: none;
		background: transparent;
		color: var(--text-muted);
		font: inherit;
		font-size: 11px;
		font-weight: 600;
		padding: 5px 10px;
		border-radius: 999px;
		cursor: pointer;
	}

	.view-btn.active {
		background: var(--panel-bg);
		color: var(--text-main);
		box-shadow: 0 1px 2px rgba(15, 23, 42, 0.08);
	}

	.view-btn:hover:not(.active) {
		color: var(--text-main);
	}

	.view-btn span {
		margin-left: 4px;
		font-size: 10px;
		font-weight: 700;
		padding: 1px 6px;
		border-radius: 999px;
		background: rgba(15, 23, 42, 0.08);
	}

	.page-status {
		margin-top: 6px;
	}

	@media (max-width: 980px) {
		.page-header {
			flex-direction: column;
		}
	}

	@container main-pane (max-width: 760px) {
		.page-header {
			flex-direction: column;
		}
	}
</style>
