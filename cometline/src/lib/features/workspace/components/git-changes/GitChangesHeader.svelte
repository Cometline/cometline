<script lang="ts">
	import { RefreshCw } from '@lucide/svelte';
	import type { WorkspaceGitStatus } from '#lib/client/cometmind.js';

	let {
		status,
		loading,
		mutating,
		workspaceAvailable,
		isClean,
		stagedCount,
		changesCount,
		commitMessage = $bindable(),
		commitFlash,
		filter = $bindable(),
		actionError,
		reviewCount = 0,
		onRefresh,
		onCommit,
		onReview
	}: {
		status: WorkspaceGitStatus | null;
		loading: boolean;
		mutating: boolean;
		workspaceAvailable: boolean;
		isClean: boolean;
		stagedCount: number;
		changesCount: number;
		commitMessage: string;
		commitFlash: string;
		filter: string;
		actionError: string | null;
		reviewCount?: number;
		onRefresh: () => unknown;
		onCommit: () => unknown;
		onReview?: () => void;
	} = $props();
</script>

<div class="git-changes-header">
	<div class="git-meta-row">
		<div class="git-meta">
			{#if status?.is_repo && status.branch}
				<span class="git-branch" title={status.upstream || status.branch}
					>{status.branch}</span
				>
				{#if !isClean}
					<span class="git-summary">{stagedCount} staged · {changesCount} changes</span>
				{:else}
					<span class="git-summary">No changes</span>
				{/if}
			{:else if status && !status.is_repo}
				<span class="git-summary">{status.message || 'Not a git repository'}</span>
			{:else}
				<span class="git-summary">{loading ? 'Loading…' : ''}</span>
			{/if}
		</div>
		<button
			type="button"
			class="git-refresh"
			onclick={() => void onRefresh()}
			disabled={loading || mutating || !workspaceAvailable}
			aria-label="Refresh git status"
			title="Refresh"
		>
			<RefreshCw size={14} class={loading ? 'spin' : ''} />
		</button>
	</div>

	{#if status?.is_repo}
		<div class="git-commit-box">
			<input
				class="git-commit-input"
				type="text"
				placeholder={stagedCount
					? `Message (↵ to commit on “${status.branch || 'HEAD'}”)`
					: 'Stage files to commit…'}
				bind:value={commitMessage}
				disabled={mutating || stagedCount === 0}
				onkeydown={(e) => {
					if (e.key === 'Enter') {
						e.preventDefault();
						void onCommit();
					}
				}}
				aria-label="Commit message"
			/>
			<button
				type="button"
				class="git-commit-btn"
				disabled={mutating || stagedCount === 0 || !commitMessage.trim()}
				onclick={() => void onCommit()}
			>
				Commit
			</button>
		</div>
		{#if commitFlash}
			<p class="git-flash" role="status">{commitFlash}</p>
		{/if}
	{/if}

	<div class="git-filter-row">
		<input
			class="git-filter"
			type="text"
			placeholder="Filter changed files…"
			bind:value={filter}
			aria-label="Filter changed files"
		/>
		{#if onReview}
			<button
				type="button"
				class="git-review-btn"
				disabled={mutating || reviewCount === 0}
				onclick={onReview}
				title={reviewCount
					? `Open diffs for ${reviewCount} file${reviewCount === 1 ? '' : 's'}`
					: 'No files to review'}
			>
				Review{reviewCount ? ` ${reviewCount}` : ''}
			</button>
		{/if}
	</div>

	{#if actionError}
		<p class="git-action-error" role="alert">{actionError}</p>
	{/if}
</div>

<style>
	.git-changes-header {
		display: flex;
		flex-direction: column;
		gap: 8px;
		padding: 10px 12px;
		border-bottom: 1px solid var(--border-subtle, rgba(0, 0, 0, 0.08));
	}

	.git-meta-row {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 8px;
	}

	.git-meta {
		display: flex;
		flex-wrap: wrap;
		align-items: baseline;
		gap: 8px;
		min-width: 0;
	}

	.git-branch {
		font-size: 12px;
		font-weight: 650;
		color: var(--text-primary, var(--color-111111));
		font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
	}

	.git-summary {
		font-size: 12px;
		color: var(--text-muted);
	}

	.git-refresh {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 28px;
		height: 28px;
		border: 1px solid var(--border-subtle, rgba(0, 0, 0, 0.1));
		border-radius: 7px;
		background: var(--surface-elevated, var(--panel-bg));
		color: var(--text-muted);
		cursor: pointer;
	}

	.git-refresh:disabled {
		opacity: 0.5;
		cursor: not-allowed;
	}

	.git-refresh :global(.spin) {
		animation: git-spin 0.7s linear infinite;
	}

	@keyframes git-spin {
		to {
			transform: rotate(360deg);
		}
	}

	.git-commit-box {
		display: flex;
		align-items: center;
		gap: 6px;
	}

	.git-commit-input {
		flex: 1;
		min-width: 0;
		box-sizing: border-box;
		height: 34px;
		border: 1px solid var(--border-subtle, rgba(0, 0, 0, 0.1));
		border-radius: 8px;
		padding: 0 10px;
		font-size: 13px;
		font-family: inherit;
		line-height: 1.4;
		background: var(--surface-elevated, var(--panel-bg));
		color: var(--text-primary, var(--color-111111));
	}

	.git-commit-input:focus {
		outline: none;
		border-color: var(--accent, var(--color-3b82f6));
	}

	.git-commit-input:disabled {
		opacity: 0.6;
	}

	.git-commit-btn {
		flex: 0 0 auto;
		border: none;
		border-radius: 8px;
		height: 34px;
		padding: 0 14px;
		/* Follows Settings hero glow so Commit tracks the active composer accent. */
		background: color-mix(
			in srgb,
			var(--hero-composer-glow-color, var(--accent)) 72%,
			var(--text-main)
		);
		color: var(--panel-bg);
		font-size: 13px;
		font-weight: 650;
		cursor: pointer;
		transition:
			background var(--duration-fast, 150ms) var(--ease-smooth, ease),
			opacity var(--duration-fast, 150ms) var(--ease-smooth, ease);
	}

	.git-commit-btn:hover:not(:disabled) {
		background: color-mix(
			in srgb,
			var(--hero-composer-glow-color, var(--accent)) 82%,
			var(--text-main)
		);
	}

	.git-commit-btn:disabled {
		opacity: 0.4;
		cursor: not-allowed;
	}

	.git-filter-row {
		display: flex;
		align-items: center;
		gap: 6px;
	}

	.git-filter {
		flex: 1;
		min-width: 0;
		width: auto;
		box-sizing: border-box;
		border: 1px solid var(--border-subtle, rgba(0, 0, 0, 0.1));
		border-radius: 8px;
		padding: 7px 10px;
		font-size: 13px;
		background: var(--surface-elevated, var(--panel-bg));
		color: var(--text-primary, var(--color-111111));
	}

	.git-filter:focus {
		outline: none;
		border-color: var(--accent, var(--color-3b82f6));
	}

	.git-review-btn {
		flex: 0 0 auto;
		height: 34px;
		border: none;
		border-radius: 8px;
		padding: 0 14px;
		background: color-mix(
			in srgb,
			var(--hero-composer-glow-color, var(--accent)) 72%,
			var(--text-main)
		);
		color: var(--panel-bg);
		font-size: 13px;
		font-weight: 650;
		cursor: pointer;
	}

	.git-review-btn:hover:not(:disabled) {
		background: color-mix(
			in srgb,
			var(--hero-composer-glow-color, var(--accent)) 82%,
			var(--text-main)
		);
	}

	.git-review-btn:disabled {
		opacity: 0.4;
		cursor: not-allowed;
	}

	.git-flash {
		margin: 0;
		font-size: 12px;
		color: var(--status-success);
	}

	.git-action-error {
		margin: 0;
		font-size: 12px;
		color: var(--status-error, var(--color-b91c1c));
	}
</style>
