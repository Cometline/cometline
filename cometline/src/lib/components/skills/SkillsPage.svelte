<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { onMount } from 'svelte';
	import {
		deleteSkill,
		getSkill,
		getSkillDraft,
		listSkillDrafts,
		listSkills,
		promoteSkillDraft,
		rejectSkillDraft,
		updateSkill,
		updateSkillDraft,
		type SkillDetailResponse,
		type SkillDraft,
		type SkillDraftDetailResponse
	} from '$lib/client/cometmind';
	import ConfirmActionModal from '$lib/components/ConfirmActionModal.svelte';
	import { skillDraftsStore } from '$lib/stores/skill-drafts.svelte';
	import { shellStore } from '$lib/stores/shell.svelte';
	import type { SkillResource } from '$lib/types';

	type SkillsTab = 'skills' | 'drafts';

	let drafts = $state<SkillDraft[]>([]);
	let selectedDraft = $state<SkillDraftDetailResponse | null>(null);
	let selectedDraftName = $state('');
	let draftContent = $state('');
	let skills = $state<SkillResource[]>([]);
	let skillErrors = $state<string[]>([]);
	let selectedSkill = $state<SkillDetailResponse | null>(null);
	let selectedSkillName = $state('');
	let skillContent = $state('');
	let skillSearch = $state('');
	let busy = $state(false);
	let contentBusy = $state(false);
	let saveBusy = $state(false);
	let deletePending = $state<SkillResource | null>(null);
	let pendingSkillName = $state('');
	let status = $state('');
	let skillRequestId = 0;
	let selectedDraftId = $derived(selectedDraft?.draft.name ?? '');
	let draftDirty = $derived(selectedDraft !== null && draftContent !== selectedDraft.content);
	let selectedSkillId = $derived(selectedSkill?.skill.name ?? '');
	let skillDirty = $derived(selectedSkill !== null && skillContent !== selectedSkill.content);
	let canEditSkill = $derived(selectedSkill?.skill.can_edit ?? false);
	let canDeleteSkill = $derived(selectedSkill?.skill.can_delete ?? false);
	let tab = $derived<SkillsTab>(
		page.url.searchParams.get('tab') === 'skills' ? 'skills' : 'drafts'
	);
	let filteredSkills = $derived.by(() => {
		const q = skillSearch.trim().toLowerCase();
		if (!q) return skills;
		return skills.filter((skill) => {
			return (
				skill.name.toLowerCase().includes(q) ||
				skill.description.toLowerCase().includes(q) ||
				skill.path.toLowerCase().includes(q)
			);
		});
	});

	onMount(() => {
		void (async () => {
			await refreshDrafts();
			await refreshSkills();
		})();
	});

	function setTab(next: SkillsTab) {
		const params = new URLSearchParams(page.url.searchParams);
		if (next === 'skills') params.set('tab', 'skills');
		else params.delete('tab');
		const search = params.toString();
		void goto(`/skills${search ? `?${search}` : ''}`, {
			replaceState: true,
			noScroll: true,
			keepFocus: true
		});
	}

	async function refreshDrafts(options: { keepSelection?: boolean } = {}) {
		busy = true;
		status = '';
		try {
			const nextDrafts = await listSkillDrafts();
			drafts = nextDrafts;
			skillDraftsStore.setCount(nextDrafts.length);
			if (nextDrafts.length === 0) {
				selectedDraft = null;
				selectedDraftName = '';
				return;
			}
			const preferred =
				options.keepSelection && selectedDraftName
					? nextDrafts.find((draft) => draft.name === selectedDraftName)?.name
					: '';
			await openDraft(preferred || nextDrafts[0].name);
		} catch (err) {
			status = err instanceof Error ? err.message : 'Failed to load skill drafts';
		} finally {
			busy = false;
		}
	}

	async function refreshSkills(options: { keepSelection?: boolean } = {}) {
		busy = true;
		status = '';
		try {
			const result = await listSkills(shellStore.workspacePath);
			skills = result.skills ?? [];
			skillErrors = result.errors ?? [];
			if (skills.length === 0) {
				selectedSkill = null;
				selectedSkillName = '';
				return;
			}
			const preferred =
				options.keepSelection && selectedSkillName
					? skills.find((skill) => skill.name === selectedSkillName)?.name
					: '';
			if (preferred && skillDirty) return;
			await openSkill(preferred || skills[0].name, { force: true });
		} catch (err) {
			status = err instanceof Error ? err.message : 'Failed to load skills';
		} finally {
			busy = false;
		}
	}

	async function refreshCurrent(options: { keepSelection?: boolean } = {}) {
		if (tab === 'drafts') {
			await refreshDrafts(options);
			return;
		}
		await refreshSkills(options);
	}

	async function openDraft(name: string) {
		selectedDraftName = name;
		contentBusy = true;
		try {
			selectedDraft = await getSkillDraft(name);
			draftContent = selectedDraft.content;
		} catch (err) {
			selectedDraft = null;
			draftContent = '';
			status = err instanceof Error ? err.message : 'Failed to load draft';
		} finally {
			contentBusy = false;
		}
	}

	async function openSkill(name: string, options: { force?: boolean } = {}) {
		if (!options.force && name === selectedSkillName && selectedSkill) return;
		if (!options.force && skillDirty) {
			pendingSkillName = name;
			return;
		}
		const requestId = ++skillRequestId;
		selectedSkillName = name;
		deletePending = null;
		contentBusy = true;
		try {
			const detail = await getSkill(name, shellStore.workspacePath);
			if (requestId !== skillRequestId || selectedSkillName !== name) return;
			selectedSkill = detail;
			skillContent = detail.content;
		} catch (err) {
			if (requestId !== skillRequestId || selectedSkillName !== name) return;
			selectedSkill = null;
			skillContent = '';
			status = err instanceof Error ? err.message : 'Failed to load skill';
		} finally {
			if (requestId === skillRequestId) contentBusy = false;
		}
	}

	function discardSkillChanges() {
		const name = pendingSkillName;
		pendingSkillName = '';
		if (name) void openSkill(name, { force: true });
	}

	function requestDeleteSelectedSkill() {
		if (selectedSkill) deletePending = selectedSkill.skill;
	}

	async function saveDraft(name: string) {
		saveBusy = true;
		status = '';
		try {
			selectedDraft = await updateSkillDraft(name, draftContent);
			draftContent = selectedDraft.content;
			status = `Saved draft ${name}.`;
			await refreshDrafts({ keepSelection: true });
		} catch (err) {
			status = err instanceof Error ? err.message : 'Failed to save draft';
		} finally {
			saveBusy = false;
		}
	}

	async function saveSkill(name: string) {
		saveBusy = true;
		status = '';
		const submittedContent = skillContent;
		try {
			const updated = await updateSkill(name, submittedContent, shellStore.workspacePath);
			if (selectedSkillName === name) {
				selectedSkill = updated;
				if (skillContent === submittedContent) skillContent = updated.content;
			}
			status = `Saved skill ${name}.`;
			await refreshSkills({ keepSelection: true });
		} catch (err) {
			status = err instanceof Error ? err.message : 'Failed to save skill';
		} finally {
			saveBusy = false;
		}
	}

	async function promoteDraft(name: string) {
		busy = true;
		status = '';
		try {
			await promoteSkillDraft(name);
			status = `Promoted draft ${name}.`;
			await refreshDrafts({ keepSelection: true });
			void refreshSkills({ keepSelection: true });
		} catch (err) {
			status = err instanceof Error ? err.message : 'Failed to promote draft';
		} finally {
			busy = false;
		}
	}

	async function confirmDeleteSkill() {
		const skill = deletePending;
		if (!skill) return;
		busy = true;
		status = '';
		try {
			await deleteSkill(skill.name, shellStore.workspacePath);
			deletePending = null;
			status = `Deleted skill ${skill.name}.`;
			if (selectedSkillName === skill.name) {
				selectedSkill = null;
				selectedSkillName = '';
				skillContent = '';
			}
			await refreshSkills({ keepSelection: true });
		} catch (err) {
			status = err instanceof Error ? err.message : 'Failed to delete skill';
		} finally {
			busy = false;
		}
	}

	async function rejectDraft(name: string) {
		busy = true;
		status = '';
		try {
			await rejectSkillDraft(name);
			status = `Rejected draft ${name}.`;
			await refreshDrafts({ keepSelection: true });
		} catch (err) {
			status = err instanceof Error ? err.message : 'Failed to reject draft';
		} finally {
			busy = false;
		}
	}
</script>

<div class="skills-page settings-ui">
	<header class="page-header">
		<div class="page-copy">
			{#if tab === 'drafts'}
				<p>
					Review and edit reusable skills drafted from <code>/create-skill</code> or
					completed jobs. Drafts stay inactive until you promote them into
					<code>~/.cometmind/skills</code>.
				</p>
			{:else}
				<p>
					Browse and edit Agent Skills Cometline can use. Changes write to the skill's
					<code>SKILL.md</code> in place. Delete removes managed skills and global
					originals under <code>~/.agents</code>, OpenCode, or Claude. Workspace and
					bundled skills stay.
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
					onclick={() => setTab('drafts')}
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
					onclick={() => setTab('skills')}
				>
					Skills
				</button>
			</div>
			<button
				class="secondary"
				type="button"
				onclick={() => void refreshCurrent({ keepSelection: true })}
			>
				{busy ? 'Loading...' : tab === 'drafts' ? 'Refresh drafts' : 'Refresh skills'}
			</button>
		</div>
	</header>

	{#if tab === 'drafts'}
		{#if drafts.length === 0}
			<section class="empty-state settings-panel-frame">
				<h2>No pending drafts</h2>
				<p>
					Use <code>/create-skill</code> or enable skill synthesis for completed jobs to create
					drafts here for manual review.
				</p>
			</section>
		{:else}
			<div class="page-layout">
				<section class="item-list settings-panel-frame">
					<div class="section-header">
						<h2>Pending</h2>
						<span>{drafts.length}</span>
					</div>
					<div class="item-list-scroll scrollbar-none">
						{#each drafts as draft (draft.name)}
							<button
								type="button"
								class="item-row"
								class:active={selectedDraftName === draft.name}
								onclick={() => void openDraft(draft.name)}
							>
								<strong>{draft.name}</strong>
								<p>{draft.description}</p>
							</button>
						{/each}
					</div>
				</section>

				<section class="item-preview settings-panel-frame">
					{#if contentBusy}
						<p class="page-muted">Loading draft…</p>
					{:else if selectedDraft}
						<header class="preview-header">
							<div>
								<h2>{selectedDraft.draft.name}</h2>
								<p>{selectedDraft.draft.description}</p>
							</div>
							<div class="preview-actions">
								<button
									type="button"
									class="secondary"
									disabled={saveBusy || busy || !draftDirty}
									onclick={() => void saveDraft(selectedDraftId)}
								>
									{saveBusy ? 'Saving...' : 'Save'}
								</button>
								<button
									type="button"
									class="secondary"
									disabled={busy || saveBusy}
									onclick={() => void rejectDraft(selectedDraftId)}
								>
									Reject
								</button>
								<button
									type="button"
									class="primary"
									disabled={busy || saveBusy || draftDirty}
									onclick={() => void promoteDraft(selectedDraftId)}
								>
									Promote
								</button>
							</div>
						</header>
						<textarea class="item-markdown" bind:value={draftContent} spellcheck="false"
						></textarea>
					{:else}
						<p class="page-muted">Select a draft to preview it.</p>
					{/if}
				</section>
			</div>
		{/if}
	{:else if skills.length === 0}
		<section class="empty-state settings-panel-frame">
			<h2>No skills discovered</h2>
			<p>
				Cometline reads skills from <code>~/.cometmind/skills</code>, workspace
				<code>.agents/skills</code>, and other configured roots.
			</p>
		</section>
	{:else}
		<div class="page-layout">
			<section class="item-list settings-panel-frame">
				<div class="section-header">
					<h2>Available</h2>
					<span>
						{#if filteredSkills.length === skills.length}
							{skills.length}
						{:else}
							{filteredSkills.length} / {skills.length}
						{/if}
					</span>
				</div>
				<input
					class="skill-search"
					type="search"
					bind:value={skillSearch}
					placeholder="Search skills by name, description, or path…"
					spellcheck="false"
				/>
				<div class="item-list-scroll scrollbar-none">
					{#if filteredSkills.length === 0}
						<p class="page-muted skill-empty">No skills match your search.</p>
					{:else}
						{#each filteredSkills as skill (skill.name)}
							<button
								type="button"
								class="item-row"
								class:active={selectedSkillName === skill.name}
								onclick={() => void openSkill(skill.name)}
							>
								<strong>
									{skill.name}
									{#if skill.is_symlink}
										<span class="skill-badge">symlink</span>
									{/if}
									{#if !skill.can_edit}
										<span class="skill-badge">read-only</span>
									{/if}
								</strong>
								<p>{skill.description}</p>
							</button>
						{/each}
					{/if}
				</div>
				{#if skillErrors.length > 0}
					<div class="skill-errors">
						{#each skillErrors as error}
							<p>{error}</p>
						{/each}
					</div>
				{/if}
			</section>

			<section class="item-preview settings-panel-frame">
				{#if contentBusy}
					<p class="page-muted">Loading skill…</p>
				{:else if selectedSkill}
					<header class="preview-header">
						<div>
							<h2>{selectedSkill.skill.name}</h2>
							<p>{selectedSkill.skill.description}</p>
							<p class="skill-path">{selectedSkill.skill.path}</p>
							{#if !canEditSkill}
								<p>This bundled skill is read-only.</p>
							{/if}
						</div>
						<div class="preview-actions">
							<button
								type="button"
								class="secondary"
								disabled={saveBusy || busy || !skillDirty || !canEditSkill}
								onclick={() => void saveSkill(selectedSkillId)}
							>
								{saveBusy ? 'Saving...' : 'Save'}
							</button>
							{#if canDeleteSkill}
								<button
									type="button"
									class="secondary danger"
									disabled={busy || saveBusy}
									title={`Delete ${selectedSkill.skill.path}`}
									onclick={requestDeleteSelectedSkill}
								>
									Delete
								</button>
							{/if}
						</div>
					</header>
					<textarea
						class="item-markdown"
						bind:value={skillContent}
						spellcheck="false"
						readonly={!canEditSkill}
					></textarea>
				{:else}
					<p class="page-muted">Select a skill to preview it.</p>
				{/if}
			</section>
		</div>
	{/if}
</div>

<ConfirmActionModal
	open={Boolean(pendingSkillName)}
	title="Discard unsaved changes?"
	description={`Switching to ${pendingSkillName || 'another skill'} will discard your unsaved changes.`}
	confirmLabel="Discard"
	onConfirm={discardSkillChanges}
	onCancel={() => (pendingSkillName = '')}
/>

<ConfirmActionModal
	open={Boolean(deletePending)}
	title={`Delete "${deletePending?.name ?? ''}"?`}
	description={deletePending
		? `This removes the original files at ${deletePending.path}. This cannot be undone.`
		: ''}
	confirmLabel="Delete"
	onConfirm={() => void confirmDeleteSkill()}
	onCancel={() => (deletePending = null)}
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

	.page-header,
	.page-layout,
	.empty-state {
		width: 100%;
		min-width: 0;
	}

	.page-header {
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

	.preview-header > div:first-child {
		min-width: 0;
	}

	.section-header h2,
	.preview-header h2,
	.empty-state h2 {
		margin: 0;
		color: var(--text-main);
	}

	.preview-header p,
	.empty-state p,
	.page-muted {
		margin: 6px 0 0;
		font-size: 12px;
		line-height: 1.5;
		color: var(--text-muted);
	}

	.page-status {
		margin-top: 6px;
	}

	.page-layout {
		display: grid;
		grid-template-columns: minmax(260px, 320px) minmax(0, 1fr);
		gap: 16px;
		min-height: 0;
		flex: 1;
	}

	.section-header,
	.preview-header {
		display: flex;
		justify-content: space-between;
		gap: 12px;
		align-items: flex-start;
		margin-bottom: 12px;
	}

	.section-header span {
		font-size: 11px;
		font-weight: 700;
		padding: 3px 7px;
		border-radius: 999px;
		background: rgba(15, 23, 42, 0.06);
		color: var(--text-muted);
	}

	.item-list,
	.item-preview,
	.empty-state {
		display: flex;
		flex-direction: column;
		min-height: 0;
	}

	.skill-search {
		margin-bottom: 8px;
		padding: 8px 10px;
		border: 1px solid var(--border-soft);
		border-radius: 10px;
		background: var(--app-bg);
		color: var(--text-main);
		font: inherit;
		font-size: 12px;
	}

	.item-list-scroll {
		overflow: auto;
		border: 1px solid var(--border-soft);
		border-radius: 12px;
		background: rgba(255, 255, 255, 0.58);
	}

	.item-row {
		width: 100%;
		text-align: left;
		padding: 10px 12px;
		border: none;
		border-bottom: 1px solid rgba(0, 0, 0, 0.06);
		background: transparent;
		cursor: pointer;
	}

	.item-row:last-child {
		border-bottom: 0;
	}

	.item-row.active {
		background: rgba(0, 102, 204, 0.08);
	}

	.item-row strong {
		display: flex;
		flex-wrap: wrap;
		gap: 6px;
		align-items: center;
		font-size: 12px;
		color: var(--text-main);
	}

	.item-row p {
		margin: 4px 0 0;
		font-size: 11px;
		line-height: 1.45;
		color: var(--text-muted);
	}

	.skill-badge {
		font-size: 10px;
		font-weight: 700;
		padding: 1px 6px;
		border-radius: 999px;
		background: rgba(15, 23, 42, 0.08);
		color: var(--text-muted);
	}

	.skill-path {
		word-break: break-all;
		font-family: var(--font-mono, 'SFMono-Regular', ui-monospace, monospace);
		font-size: 11px;
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

	.preview-actions {
		display: flex;
		gap: 8px;
		flex-shrink: 0;
	}

	.item-markdown {
		margin: 0;
		padding: 12px;
		border-radius: 12px;
		border: 1px solid var(--border-soft);
		background: var(--app-bg);
		white-space: pre-wrap;
		font-size: 12px;
		line-height: 1.5;
		color: var(--text-main);
		overflow: auto;
		flex: 1;
		width: 100%;
		resize: none;
		font-family: var(--font-mono, 'SFMono-Regular', ui-monospace, monospace);
	}

	.item-markdown[readonly] {
		opacity: 0.82;
	}

	@media (max-width: 980px) {
		.skills-page {
			padding: 16px;
		}

		.page-header,
		.preview-header {
			flex-direction: column;
		}

		.page-layout {
			grid-template-columns: 1fr;
		}
	}

	@container main-pane (max-width: 760px) {
		.skills-page {
			padding: 16px;
		}

		.page-header,
		.preview-header {
			flex-direction: column;
		}

		.page-layout {
			grid-template-columns: minmax(0, 1fr);
		}

		.preview-actions {
			width: 100%;
			flex-wrap: wrap;
		}
	}
</style>
