<script lang="ts">
	import {
		ChevronDown,
		ChevronRight,
		Loader,
		MessageSquarePlus,
		Minus,
		Plus,
		RotateCcw
	} from '@lucide/svelte';
	import ConfirmActionModal from '#lib/components/ConfirmActionModal.svelte';
	import SelectionAddToChat from '#lib/components/SelectionAddToChat.svelte';
	import FileTypeIcon from '#lib/features/workspace/components/FileTypeIcon.svelte';
	import GitDiffLines from '#lib/features/workspace/components/git-diff/GitDiffLines.svelte';
	import {
		gitFileDir,
		gitFileName,
		type GitReviewFile
	} from '#lib/features/workspace/git-changes-browser.js';
	import { createGitReviewFileController } from '#lib/features/workspace/git-review-file.svelte.js';

	let {
		workspacePath,
		file,
		active,
		onMutated
	}: {
		workspacePath: string;
		file: GitReviewFile;
		active: boolean;
		onMutated?: () => unknown;
	} = $props();

	const review = createGitReviewFileController({
		getWorkspacePath: () => workspacePath,
		getFile: () => file,
		getActive: () => active,
		getOnMutated: () => onMutated
	});
</script>

<article
	class="review-file"
	class:review-file-staged={file.section === 'staged'}
	class:collapsed={!review.open}
>
	<header class="review-file-header">
		<button
			type="button"
			class="review-toggle"
			onclick={review.toggle}
			aria-expanded={review.open}
		>
			<span class="review-chevron">
				{#if review.open}
					<ChevronDown size={13} />
				{:else}
					<ChevronRight size={13} />
				{/if}
			</span>
			<span class="review-icon" aria-hidden="true">
				<FileTypeIcon path={file.path} size={16} />
			</span>
			<span class="review-name" title={file.path}>{gitFileName(file.path)}</span>
			{#if gitFileDir(file.path)}
				<span class="review-dir" title={file.path}>{gitFileDir(file.path)}</span>
			{/if}
		</button>
		<div class="review-actions">
			<button
				type="button"
				class="review-action"
				title="Add diff to chat"
				aria-label="Add diff to chat"
				disabled={!review.hasDiff}
				onclick={review.addDiffToChat}
			>
				<MessageSquarePlus size={13} />
			</button>
			{#if file.section === 'staged'}
				<button
					type="button"
					class="review-action"
					title="Unstage"
					disabled={review.mutating}
					onclick={() => void review.unstage()}
				>
					<Minus size={13} />
				</button>
			{:else}
				<button
					type="button"
					class="review-action"
					title="Stage"
					disabled={review.mutating}
					onclick={() => void review.stage()}
				>
					<Plus size={13} />
				</button>
				<button
					type="button"
					class="review-action danger"
					title="Discard"
					disabled={review.mutating}
					onclick={review.requestDiscard}
				>
					<RotateCcw size={13} />
				</button>
			{/if}
		</div>
	</header>

	{#if review.open}
		{#if !active}
			<p class="review-note">Scroll to load this diff.</p>
		{:else if review.loading && !review.loaded}
			<p class="review-note">
				<Loader size={13} class="spin" />
				Loading diff…
			</p>
		{:else if review.error}
			<p class="review-note error">{review.error}</p>
		{:else if review.binary || review.empty || review.lines.length === 0}
			<p class="review-note">
				{review.message || (review.binary ? 'Binary file.' : 'No diff in this scope.')}
			</p>
		{:else}
			<!-- svelte-ignore a11y_no_static_element_interactions -->
			<div class="review-diff" onmouseup={review.onDiffMouseUp}>
				<GitDiffLines lines={review.lines} language={review.language} />
				{#if review.truncated}
					<p class="review-note">{review.message || 'Diff truncated.'}</p>
				{/if}
			</div>
		{/if}
		{#if review.actionError}
			<p class="review-note error">{review.actionError}</p>
		{/if}
	{/if}
	{#if review.selectionPopup}
		<SelectionAddToChat
			position={{ top: review.selectionPopup.top, left: review.selectionPopup.left }}
			onAdd={review.addSelection}
			onDismiss={review.clearSelection}
		/>
	{/if}
</article>

<ConfirmActionModal
	open={review.discardOpen}
	title="Discard local changes?"
	description={`Discard changes to “${file.path}”? Tracked files restore to HEAD. Untracked files are deleted. This cannot be undone.`}
	confirmLabel="Discard"
	onCancel={review.cancelDiscard}
	onConfirm={() => void review.confirmDiscard()}
/>

<style>
	.review-file {
		display: flex;
		flex-direction: column;
		border-radius: 8px;
		border: 1px solid var(--border-subtle, rgba(0, 0, 0, 0.08));
		background: var(--panel-bg);
		overflow: hidden;
	}

	.review-file-staged {
		--review-accent: var(--hero-composer-glow-color, var(--accent));
	}

	.review-file-header {
		position: sticky;
		top: 0;
		z-index: 1;
		display: flex;
		align-items: center;
		gap: 2px;
		padding: 0 2px 0 0;
		background: var(--panel-bg);
		border-radius: 7px 7px 0 0;
	}

	.collapsed .review-file-header {
		border-radius: 7px;
	}

	.review-toggle {
		display: flex;
		align-items: center;
		gap: 6px;
		flex: 1;
		min-width: 0;
		border: none;
		border-radius: 7px;
		background: transparent;
		padding: 6px 8px;
		cursor: pointer;
		color: var(--text-primary, var(--color-111111));
		text-align: left;
	}

	.review-chevron {
		display: grid;
		place-items: center;
		flex: 0 0 auto;
		color: var(--workspace-inactive-color, var(--workspace-group-color));
	}

	.review-file-staged .review-chevron {
		color: var(--review-accent);
	}

	.review-icon {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		flex: 0 0 16px;
		width: 16px;
		height: 16px;
	}

	.review-name {
		flex: 0 1 auto;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		font-size: 13px;
		color: var(--text-primary, var(--color-111111));
	}

	.review-dir {
		flex: 1 1 auto;
		min-width: 0;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		font-size: 11px;
		color: var(--text-muted);
		font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
	}

	.review-actions {
		display: flex;
		gap: 1px;
		flex: 0 0 auto;
		margin-right: 2px;
	}

	.review-action {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 26px;
		height: 26px;
		border: none;
		border-radius: 6px;
		background: transparent;
		color: var(--text-muted);
		cursor: pointer;
	}

	.review-action:hover:not(:disabled) {
		background: color-mix(
			in srgb,
			var(--workspace-inactive-color, var(--workspace-group-color)) 18%,
			transparent
		);
		color: var(--text-primary, var(--color-111111));
	}

	.review-file-staged .review-action:hover:not(:disabled) {
		background: color-mix(in srgb, var(--review-accent) 20%, transparent);
		color: var(--review-accent);
	}

	.review-action.danger:hover:not(:disabled) {
		background: rgba(185, 28, 28, 0.1);
		color: var(--color-b91c1c);
	}

	.review-action:disabled {
		opacity: 0.4;
		cursor: not-allowed;
	}

	.review-diff :global(.git-diff-body) {
		flex: none;
		overflow: visible;
		max-height: none;
		padding-bottom: 8px;
	}

	.review-note {
		display: flex;
		align-items: center;
		gap: 6px;
		margin: 0;
		padding: 8px 14px 12px;
		font-size: 12px;
		color: var(--text-muted);
	}

	.review-note.error {
		color: var(--status-error, var(--color-b91c1c));
	}

	.review-note :global(.spin) {
		animation: review-spin 0.7s linear infinite;
	}

	@keyframes review-spin {
		to {
			transform: rotate(360deg);
		}
	}
</style>
