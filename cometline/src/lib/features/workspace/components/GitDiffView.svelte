<script lang="ts">
	import { Loader } from '@lucide/svelte';
	import type { GitScope } from '#lib/client/cometmind.js';
	import ConfirmActionModal from '#lib/components/ConfirmActionModal.svelte';
	import SelectionAddToChat from '#lib/components/SelectionAddToChat.svelte';
	import GitDiffHeader from '#lib/features/workspace/components/git-diff/GitDiffHeader.svelte';
	import GitDiffLines from '#lib/features/workspace/components/git-diff/GitDiffLines.svelte';
	import { createGitDiffViewController } from '#lib/features/workspace/git-diff-view.svelte.js';
	import { workspaceChangeVersion } from '#lib/features/workspace/workspace-change.svelte.js';

	let {
		workspacePath,
		filePath,
		scope = 'working' as GitScope,
		onBack,
		onMutated
	}: {
		workspacePath: string;
		filePath: string;
		scope?: GitScope;
		onBack?: () => void;
		onMutated?: () => void;
	} = $props();

	const diff = createGitDiffViewController({
		getWorkspacePath: () => workspacePath,
		getFilePath: () => filePath,
		getScope: () => scope,
		getOnBack: () => onBack,
		getOnMutated: () => onMutated
	});

	$effect(() => {
		void [workspacePath, filePath, scope, workspaceChangeVersion(workspacePath)];
		void diff.load();
	});
</script>

<div class="git-diff-view">
	<GitDiffHeader
		{filePath}
		mutating={diff.mutating}
		canStage={diff.canStage}
		canUnstage={diff.canUnstage}
		canDiscard={diff.canDiscard}
		copyFlash={diff.copyFlash}
		hasDiff={Boolean(diff.diffText.trim())}
		actionError={diff.actionError}
		{onBack}
		onStage={diff.stageFile}
		onUnstage={diff.unstageFile}
		onDiscard={diff.requestDiscard}
		onCopyPath={diff.copyPath}
		onAddPath={diff.addPathToChat}
		onAddDiff={diff.addFullDiffToChat}
	/>

	{#if diff.loading}
		<div class="git-diff-state">
			<Loader size={16} stroke-width={2} class="git-diff-spinner" />
			<span>Loading diff…</span>
		</div>
	{:else if diff.error}
		<div class="git-diff-state git-diff-error">{diff.error}</div>
	{:else if diff.binary}
		<div class="git-diff-state">{diff.message || 'Binary file; diff not shown.'}</div>
	{:else if diff.empty}
		<div class="git-diff-state">
			{diff.message || 'No diff for this path in the selected scope.'}
		</div>
	{:else if !diff.diffText.trim()}
		<div class="git-diff-state">{diff.message || 'No diff available.'}</div>
	{:else}
		<!-- svelte-ignore a11y_no_static_element_interactions -->
		<div class="git-diff-body-wrap" onmouseup={diff.onDiffMouseUp}>
			<GitDiffLines lines={diff.highlightedLines} language={diff.language} />
			{#if diff.truncated}
				<p class="git-diff-truncated">{diff.message || 'Diff truncated.'}</p>
			{/if}
		</div>
	{/if}

	{#if diff.selectionPopup}
		<SelectionAddToChat
			position={{ top: diff.selectionPopup.top, left: diff.selectionPopup.left }}
			onAdd={diff.addSelectionToChat}
			onDismiss={diff.clearSelectionPopup}
		/>
	{/if}
</div>

<ConfirmActionModal
	open={diff.discardConfirmOpen}
	title="Discard local changes?"
	description={`Discard changes to “${filePath}”? Tracked files restore to HEAD. Untracked files are deleted. This cannot be undone.`}
	confirmLabel="Discard"
	onCancel={diff.cancelDiscard}
	onConfirm={() => void diff.confirmDiscard()}
/>

<style>
	.git-diff-view {
		position: relative;
		display: flex;
		flex-direction: column;
		height: 100%;
		min-height: 0;
		background: var(--panel-bg);
	}

	.git-diff-state {
		display: flex;
		align-items: center;
		justify-content: center;
		gap: 8px;
		padding: 24px;
		color: var(--text-muted);
		font-size: 13px;
		text-align: center;
	}

	.git-diff-error {
		color: var(--status-error, var(--color-b91c1c));
	}

	.git-diff-state :global(.git-diff-spinner) {
		animation: git-diff-spin 0.7s linear infinite;
	}

	@keyframes git-diff-spin {
		to {
			transform: rotate(360deg);
		}
	}

	.git-diff-body-wrap {
		flex: 1;
		min-height: 0;
		display: flex;
		flex-direction: column;
		overflow: hidden;
	}

	.git-diff-truncated {
		margin: 0;
		padding: 8px 12px 12px;
		font-size: 12px;
		color: var(--text-muted);
	}
</style>
