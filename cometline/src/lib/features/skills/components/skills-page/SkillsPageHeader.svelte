<script lang="ts">
	import type { SkillsTab } from '#lib/features/skills/skills-page-controller.svelte.js';
	import { skillDraftsStore } from '#lib/stores/skill-drafts.svelte.js';

	let {
		tab,
		status,
		busy,
		onSelectTab,
		onRefresh,
		onSync
	}: {
		tab: SkillsTab;
		status: string;
		busy: boolean;
		onSelectTab: (tab: SkillsTab) => void;
		onRefresh: () => void;
		onSync: () => void;
	} = $props();
</script>

<header class="page-header">
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
	{#if status}
		<p class="page-status">{status}</p>
	{/if}
	<div class="page-header-actions">
		<button
			class="secondary"
			class:header-button-hidden={tab !== 'skills'}
			type="button"
			onclick={onSync}
			disabled={busy || tab !== 'skills'}
			aria-hidden={tab !== 'skills'}
			tabindex={tab === 'skills' ? 0 : -1}
		>
			Sync symlinks
		</button>
		<button class="secondary" type="button" onclick={onRefresh} disabled={busy}>
			{busy ? 'Loading...' : tab === 'drafts' ? 'Refresh drafts' : 'Refresh skills'}
		</button>
	</div>
</header>

<style>
	.page-header {
		width: 100%;
		min-width: 0;
		display: flex;
		flex-wrap: nowrap;
		justify-content: space-between;
		gap: 12px 16px;
		align-items: flex-start;
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
		flex: 0 0 auto;
		gap: 8px;
		align-items: center;
		margin-left: auto;
	}

	.header-button-hidden {
		visibility: hidden;
	}

	.view-toggle {
		display: inline-flex;
		position: sticky;
		top: 12px;
		z-index: 2;
		flex: 0 0 auto;
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
		flex: 1;
		margin: 0;
		padding-left: 14px;
	}

	@media (max-width: 980px) {
		.page-header {
			flex-wrap: wrap;
		}

		.page-header-actions {
			margin-left: 0;
		}
	}

	@container main-pane (max-width: 760px) {
		.page-header {
			flex-wrap: wrap;
		}

		.page-header-actions {
			margin-left: 0;
		}
	}
</style>
