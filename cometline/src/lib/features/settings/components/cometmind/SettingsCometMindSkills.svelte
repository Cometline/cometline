<script lang="ts">
	import { Search } from '@lucide/svelte';
	import SettingsToggle from '../SettingsToggle.svelte';
	import type { CometMindSettings } from '$lib/cometmind-settings';
	import type { SkillResource } from '$lib/types';

	type SkillSourceFilter =
		| 'all'
		| 'cometmind'
		| 'global'
		| 'workspace'
		| 'opencode'
		| 'claude'
		| 'other';

	let {
		cometmind = $bindable(),
		skillSearch = $bindable(),
		skillSourceFilter = $bindable(),
		skills,
		filteredSkills,
		skillErrors,
		skillsBusy,
		skillsStatus,
		skillSourceCounts,
		filters,
		sourceLabels,
		skillSourceCategory,
		refreshSkills,
		onSyncSkills,
		onExportSkill,
		requestDeleteSkill
	}: {
		cometmind: CometMindSettings;
		skillSearch: string;
		skillSourceFilter: SkillSourceFilter;
		skills: SkillResource[];
		filteredSkills: SkillResource[];
		skillErrors: string[];
		skillsBusy: boolean;
		skillsStatus: string;
		skillSourceCounts: Record<SkillSourceFilter, number>;
		filters: { id: SkillSourceFilter; label: string }[];
		sourceLabels: Record<Exclude<SkillSourceFilter, 'all'>, string>;
		skillSourceCategory: (skill: SkillResource) => Exclude<SkillSourceFilter, 'all'>;
		refreshSkills: () => void;
		onSyncSkills: () => void;
		onExportSkill: (name: string) => void;
		requestDeleteSkill: (skill: SkillResource) => void;
	} = $props();
</script>

<div class="settings-section">
	<div class="settings-section-heading">
		<h3>Skills</h3>
		<p>
			CometMind reads Agent Skills from <code>~/.cometmind/skills</code>, workspace
			<code>.agents/skills</code>/<code>.claude/skills</code>, OpenCode, and Claude Code skill
			folders.
		</p>
	</div>
	<SettingsToggle
		label="Enable skills"
		description="Expose a compact skill index to CometMind and allow read-only loading via load_skill."
		bind:checked={cometmind.skills.enabled}
	/>
	<SettingsToggle
		label="Synthesize skill drafts from completed jobs"
		description="After a job completes, an LLM proposes a reusable skill draft for review. Choose its model in Models → Skill synthesis."
		bind:checked={cometmind.skills.synthesisEnabled}
		disabled={!cometmind.skills.enabled}
	/>
	<div class="skills-actions">
		<button class="secondary" type="button" onclick={refreshSkills} disabled={skillsBusy}>
			{skillsBusy ? 'Loading...' : 'Refresh skills'}
		</button>
		<button class="secondary" type="button" onclick={onSyncSkills} disabled={skillsBusy}>
			Sync symlinks
		</button>
	</div>
	{#if skillsStatus}
		<p class="settings-field-hint">{skillsStatus}</p>
	{/if}
	<div class="skills-toolbar">
		<div class="skills-search">
			<Search size={14} aria-hidden="true" />
			<input
				type="search"
				bind:value={skillSearch}
				placeholder="Search skills by name, description, or path…"
				spellcheck="false"
			/>
		</div>
		<div class="skills-filters" role="group" aria-label="Filter skills by source">
			{#each filters as filter (filter.id)}
				<button
					type="button"
					class="skills-filter-chip"
					class:active={skillSourceFilter === filter.id}
					aria-pressed={skillSourceFilter === filter.id}
					onclick={() => (skillSourceFilter = filter.id)}
				>
					{filter.label}
					<span class="skills-filter-count">{skillSourceCounts[filter.id]}</span>
				</button>
			{/each}
		</div>
	</div>
	<div class="skills-list scrollbar-none">
		<div class="skills-list-header">
			<span>Available skills</span>
			<strong>
				{#if filteredSkills.length === skills.length}
					{skills.length}
				{:else}
					{filteredSkills.length} / {skills.length}
				{/if}
			</strong>
		</div>
		{#if skills.length === 0}
			<p class="settings-field-hint skills-empty">
				No skills discovered yet. Try <code>npx skills add ...</code> or add a custom root.
			</p>
		{:else if filteredSkills.length === 0}
			<p class="settings-field-hint skills-empty">No skills match your search or filter.</p>
		{:else}
			{#each filteredSkills as skill (skill.name)}
				<div class="skill-row" title={skill.path}>
					<div class="skill-row-main">
						<div class="skill-row-title">
							<strong>{skill.name}</strong>
							<span class="skill-badge"
								>{sourceLabels[skillSourceCategory(skill)]}</span
							>
							{#if skill.is_symlink}
								<span class="skill-badge">symlink</span>
							{/if}
						</div>
						<p>{skill.description}</p>
					</div>
					<div class="skill-row-actions">
						{#if skill.can_export}
							<button
								class="secondary"
								type="button"
								disabled={skillsBusy}
								onclick={() => onExportSkill(skill.name)}
							>
								Export
							</button>
						{/if}
						{#if skill.can_delete}
							<button
								class="secondary danger"
								type="button"
								disabled={skillsBusy}
								title={`Delete ${skill.path}`}
								onclick={() => requestDeleteSkill(skill)}
							>
								Delete
							</button>
						{/if}
					</div>
				</div>
			{/each}
		{/if}
	</div>
	{#if skillErrors.length > 0}
		<div class="skill-errors">
			{#each skillErrors as error, i (i)}
				<p>{error}</p>
			{/each}
		</div>
	{/if}
</div>

<style>
	.skills-actions {
		display: flex;
		flex-wrap: wrap;
		gap: 10px 14px;
	}

	.skills-toolbar {
		display: flex;
		flex-direction: column;
		gap: 10px;
	}

	.skills-search {
		display: flex;
		align-items: center;
		gap: 8px;
		border: 1px solid var(--border-soft);
		border-radius: 10px;
		padding: 8px 10px;
		background: rgba(255, 255, 255, 0.82);
		color: var(--text-muted);
	}

	.skills-search input {
		flex: 1;
		min-width: 0;
		border: 0;
		background: transparent;
		padding: 0;
		font-size: 12px;
		color: var(--text-main);
		outline: none;
	}

	.skills-search input::placeholder {
		color: var(--text-muted);
	}

	.skills-filters {
		display: flex;
		flex-wrap: wrap;
		gap: 6px;
	}

	.skills-filter-chip {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		border: 1px solid var(--border-soft);
		background: rgba(255, 255, 255, 0.72);
		color: var(--text-muted);
		border-radius: 999px;
		padding: 5px 10px;
		font-size: 11px;
		font-weight: 600;
		cursor: pointer;
		-webkit-tap-highlight-color: transparent;
	}

	.skills-filter-chip:hover {
		background: rgba(15, 23, 42, 0.06);
		border-color: rgba(15, 23, 42, 0.16);
		color: var(--text-main);
	}

	.skills-filter-chip.active {
		border-color: rgba(0, 102, 204, 0.28);
		background: rgba(0, 102, 204, 0.08);
		color: var(--text-main);
	}

	.skills-filter-count {
		min-width: 1.2em;
		text-align: center;
		padding: 1px 5px;
		border-radius: 999px;
		background: rgba(15, 23, 42, 0.06);
		font-size: 10px;
		font-weight: 700;
	}

	.skills-list {
		border: 1px solid var(--border-soft);
		border-radius: 12px;
		background: rgba(255, 255, 255, 0.58);
		max-height: 260px;
		overflow: auto;
	}

	.skills-list-header {
		position: sticky;
		top: 0;
		display: flex;
		justify-content: space-between;
		padding: 9px 11px;
		border-bottom: 1px solid var(--border-soft);
		background: rgba(250, 248, 244, 0.94);
		font-size: 12px;
		font-weight: 650;
		color: var(--text-main);
	}

	.skill-row {
		display: flex;
		align-items: flex-start;
		justify-content: space-between;
		gap: 12px;
		padding: 10px 11px;
		border-bottom: 1px solid rgba(0, 0, 0, 0.06);
	}

	.skill-row-main {
		min-width: 0;
		flex: 1;
	}

	.skill-row-title {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: 6px;
	}

	.skill-row-actions {
		display: flex;
		align-items: center;
		gap: 6px;
		flex-shrink: 0;
	}

	.skill-badge {
		display: inline-block;
		padding: 1px 6px;
		border-radius: 999px;
		font-size: 10px;
		font-weight: 600;
		color: var(--text-muted);
		background: rgba(15, 23, 42, 0.06);
		vertical-align: middle;
	}

	.skills-empty {
		padding: 10px 11px;
	}

	.skill-row .secondary.danger {
		color: var(--status-error);
	}

	.skill-row:last-child {
		border-bottom: 0;
	}

	.skill-row strong {
		font-size: 12px;
		color: var(--text-main);
	}

	.skill-row p,
	.skill-errors p {
		margin: 3px 0 0;
		font-size: 11px;
		line-height: 1.45;
		color: var(--text-muted);
	}

	.skill-errors {
		border: 1px solid rgba(190, 90, 60, 0.25);
		border-radius: 10px;
		padding: 8px 10px;
		background: rgba(255, 236, 224, 0.45);
	}
</style>
