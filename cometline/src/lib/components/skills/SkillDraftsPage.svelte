<script lang="ts">
	import { onMount } from 'svelte';
	import {
		getSkillDraft,
		listSkillDrafts,
		promoteSkillDraft,
		rejectSkillDraft,
		type SkillDraft,
		type SkillDraftDetailResponse
	} from '$lib/client/cometmind';

	let drafts = $state<SkillDraft[]>([]);
	let selectedDraft = $state<SkillDraftDetailResponse | null>(null);
	let selectedDraftName = $state('');
	let busy = $state(false);
	let contentBusy = $state(false);
	let status = $state('');
	let selectedDraftId = $derived(selectedDraft?.draft.name ?? '');

	onMount(() => {
		void refreshDrafts();
	});

	async function refreshDrafts(options: { keepSelection?: boolean } = {}) {
		busy = true;
		status = '';
		try {
			const nextDrafts = await listSkillDrafts();
			drafts = nextDrafts;
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

	async function openDraft(name: string) {
		selectedDraftName = name;
		contentBusy = true;
		try {
			selectedDraft = await getSkillDraft(name);
		} catch (err) {
			selectedDraft = null;
			status = err instanceof Error ? err.message : 'Failed to load draft';
		} finally {
			contentBusy = false;
		}
	}

	async function promoteDraft(name: string) {
		busy = true;
		status = '';
		try {
			await promoteSkillDraft(name);
			status = `Promoted draft ${name}.`;
			await refreshDrafts({ keepSelection: true });
		} catch (err) {
			status = err instanceof Error ? err.message : 'Failed to promote draft';
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

<div class="skill-drafts-page settings-ui">
	<header class="page-header">
		<div>
			<h1>Skill Drafts</h1>
			<p>
				Review reusable skills proposed from completed jobs. Drafts stay inactive until you
				promote them into <code>~/.cometmind/skills</code>.
			</p>
		</div>
		<button class="secondary" type="button" onclick={() => void refreshDrafts({ keepSelection: true })}>
			{busy ? 'Loading...' : 'Refresh drafts'}
		</button>
	</header>

	{#if status}
		<p class="page-status">{status}</p>
	{/if}

	{#if drafts.length === 0}
		<section class="empty-state settings-panel-frame">
			<h2>No pending drafts</h2>
			<p>
				When skill synthesis is enabled, completed jobs can propose drafts here for manual
				review.
			</p>
		</section>
	{:else}
		<div class="page-layout">
			<section class="draft-list settings-panel-frame">
				<div class="section-header">
					<h2>Pending</h2>
					<span>{drafts.length}</span>
				</div>
				<div class="draft-list-scroll scrollbar-none">
					{#each drafts as draft (draft.name)}
						<button
							type="button"
							class="draft-row"
							class:active={selectedDraftName === draft.name}
							onclick={() => void openDraft(draft.name)}
						>
							<strong>{draft.name}</strong>
							<p>{draft.description}</p>
						</button>
					{/each}
				</div>
			</section>

			<section class="draft-preview settings-panel-frame">
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
								disabled={busy}
								onclick={() => void rejectDraft(selectedDraftId)}
							>
								Reject
							</button>
							<button
								type="button"
								class="primary"
								disabled={busy}
								onclick={() => void promoteDraft(selectedDraftId)}
							>
								Promote
							</button>
						</div>
					</header>
					<pre class="draft-markdown">{selectedDraft.content}</pre>
				{:else}
					<p class="page-muted">Select a draft to preview it.</p>
				{/if}
			</section>
		</div>
	{/if}
</div>

<style>
	.skill-drafts-page {
		display: flex;
		flex-direction: column;
		height: 100%;
		min-height: 0;
		padding: 20px 24px;
		gap: 16px;
		overflow: hidden;
	}

	.page-header {
		display: flex;
		justify-content: space-between;
		gap: 16px;
		align-items: flex-start;
	}

	.page-header h1,
	.section-header h2,
	.preview-header h2,
	.empty-state h2 {
		margin: 0;
		color: var(--text-main);
	}

	.page-header p,
	.preview-header p,
	.empty-state p,
	.page-status,
	.page-muted {
		margin: 6px 0 0;
		font-size: 12px;
		line-height: 1.5;
		color: var(--text-muted);
	}

	.page-status {
		margin: 0;
	}

	.empty-state,
	.draft-list,
	.draft-preview {
		padding: 14px;
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

	.draft-list,
	.draft-preview,
	.empty-state {
		display: flex;
		flex-direction: column;
		min-height: 0;
	}

	.draft-list-scroll {
		overflow: auto;
		border: 1px solid var(--border-soft);
		border-radius: 12px;
		background: rgba(255, 255, 255, 0.58);
	}

	.draft-row {
		width: 100%;
		text-align: left;
		padding: 10px 12px;
		border: none;
		border-bottom: 1px solid rgba(0, 0, 0, 0.06);
		background: transparent;
		cursor: pointer;
	}

	.draft-row:last-child {
		border-bottom: 0;
	}

	.draft-row.active {
		background: rgba(0, 102, 204, 0.08);
	}

	.draft-row strong {
		font-size: 12px;
		color: var(--text-main);
	}

	.draft-row p {
		margin: 4px 0 0;
		font-size: 11px;
		line-height: 1.45;
		color: var(--text-muted);
	}

	.preview-actions {
		display: flex;
		gap: 8px;
		flex-shrink: 0;
	}

	.draft-markdown {
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
	}

	@media (max-width: 980px) {
		.skill-drafts-page {
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
</style>
