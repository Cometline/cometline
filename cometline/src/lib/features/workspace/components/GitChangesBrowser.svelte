<script lang="ts">
	import { Loader } from '@lucide/svelte';
	import ConfirmActionModal from '$lib/components/ConfirmActionModal.svelte';
	import GitChangesHeader from '$lib/features/workspace/components/git-changes/GitChangesHeader.svelte';
	import GitChangesSection from '$lib/features/workspace/components/git-changes/GitChangesSection.svelte';
	import GitFileRow from '$lib/features/workspace/components/git-changes/GitFileRow.svelte';
	import { createGitChangesBrowserController } from '$lib/features/workspace/git-changes-browser.svelte';
	import type {
		GitChangesSectionKind,
		GitFile
	} from '$lib/features/workspace/git-changes-browser';
	import { workspaceChangeVersion } from '$lib/features/workspace/workspace-change.svelte';

	let {
		workspacePath
	}: {
		workspacePath: string;
	} = $props();

	const changes = createGitChangesBrowserController({ getWorkspacePath: () => workspacePath });

	let stagedOpen = $state(true);
	let changesOpen = $state(true);

	$effect(() => {
		void [changes.normalizedWorkspace, workspaceChangeVersion(changes.normalizedWorkspace)];
		void changes.load();
	});
</script>

{#snippet fileRow(file: GitFile, section: GitChangesSectionKind)}
	<GitFileRow
		{file}
		{section}
		mutating={changes.mutating}
		copied={changes.copiedPath === file.path}
		onOpen={changes.openDiff}
		onStage={changes.stagePath}
		onUnstage={changes.unstagePath}
		onDiscard={changes.requestDiscard}
		onCopy={changes.copyPath}
		onAddToChat={changes.addPathToChat}
	/>
{/snippet}

<div class="git-changes">
	<GitChangesHeader
		status={changes.status}
		loading={changes.loading}
		mutating={changes.mutating}
		workspaceAvailable={changes.workspaceAvailable}
		isClean={changes.isClean}
		stagedCount={changes.stagedCount}
		changesCount={changes.changesCount}
		bind:commitMessage={changes.commitMessage}
		commitFlash={changes.commitFlash}
		bind:filter={changes.filter}
		actionError={changes.actionError}
		onRefresh={changes.load}
		onCommit={changes.commit}
	/>

	{#if !changes.workspaceAvailable}
		<div class="git-state">Select a workspace to see git changes.</div>
	{:else if changes.loading && !changes.status}
		<div class="git-state">
			<Loader size={16} stroke-width={2} class="git-spinner" />
			<span>Loading changes…</span>
		</div>
	{:else if changes.error}
		<div class="git-state git-error">{changes.error}</div>
	{:else if changes.status && !changes.status.is_repo}
		<div class="git-state">
			{changes.status.message || 'This workspace is not a git repository.'}
		</div>
	{:else if changes.isClean && !changes.query}
		<div class="git-state">Working tree clean.</div>
	{:else if changes.stagedFiles.length === 0 && changes.changeFiles.length === 0}
		<div class="git-state">No matching changed files.</div>
	{:else}
		<div class="git-list-scroll scrollbar-none">
			{#if changes.stagedFiles.length > 0 || changes.stagedCount > 0}
				<GitChangesSection
					section="staged"
					files={changes.stagedFiles}
					bind:open={stagedOpen}
					mutating={changes.mutating}
					onUnstageAll={changes.unstageAll}
					row={fileRow}
				/>
			{/if}

			{#if changes.changeFiles.length > 0 || changes.changesCount > 0}
				<GitChangesSection
					section="changes"
					files={changes.changeFiles}
					bind:open={changesOpen}
					mutating={changes.mutating}
					onDiscardAll={changes.requestDiscardAll}
					onStageAll={changes.stageAllChanges}
					row={fileRow}
				/>
			{/if}

			{#if changes.status?.truncated}
				<p class="git-truncated">Showing first {changes.status.files.length} files.</p>
			{/if}
		</div>
	{/if}
</div>

<ConfirmActionModal
	open={Boolean(changes.discardConfirm)}
	title={changes.discardConfirm?.kind === 'all'
		? 'Discard all local changes?'
		: 'Discard local changes?'}
	description={changes.discardDescription}
	confirmLabel={changes.discardConfirm?.kind === 'all' ? 'Discard all' : 'Discard'}
	onCancel={changes.cancelDiscard}
	onConfirm={() => void changes.confirmDiscard()}
/>

<style>
	.git-changes {
		display: flex;
		flex-direction: column;
		height: 100%;
		min-height: 0;
		background: var(--panel-bg);
	}

	.git-state {
		display: flex;
		align-items: center;
		justify-content: center;
		gap: 8px;
		padding: 24px;
		color: var(--text-muted);
		font-size: 13px;
		text-align: center;
	}

	.git-error {
		color: var(--status-error, var(--color-b91c1c));
	}

	.git-state :global(.git-spinner) {
		animation: git-spin 0.7s linear infinite;
	}

	@keyframes git-spin {
		to {
			transform: rotate(360deg);
		}
	}

	.git-list-scroll {
		flex: 1;
		min-height: 0;
		overflow: auto;
		padding: 6px 8px 12px;
		display: flex;
		flex-direction: column;
		gap: 6px;
	}

	.git-truncated {
		margin: 8px 12px 0;
		font-size: 12px;
		color: var(--text-muted);
	}
</style>
