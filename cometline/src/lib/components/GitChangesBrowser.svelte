<script lang="ts">
	import {
		ChevronDown,
		ChevronRight,
		Copy,
		Loader,
		MessageSquarePlus,
		Minus,
		Plus,
		RefreshCw,
		RotateCcw
	} from '@lucide/svelte';
	import {
		commitWorkspaceGit,
		discardWorkspaceGitPaths,
		getWorkspaceGitStatus,
		stageWorkspaceGitPaths,
		unstageWorkspaceGitPaths,
		type WorkspaceGitStatus
	} from '$lib/client/cometmind';
	import ConfirmActionModal from '$lib/components/ConfirmActionModal.svelte';
	import FileTypeIcon from '$lib/components/FileTypeIcon.svelte';
	import { shellStore } from '$lib/stores/shell.svelte';
	import { normalizeWorkspacePath } from '$lib/workspace/file-index';
	import { hasUnstagedSide } from '$lib/workspace/git-file-state';

	type GitFile = WorkspaceGitStatus['files'][number];
	type DiscardConfirm = { kind: 'one'; path: string } | { kind: 'all' };

	let {
		workspacePath
	}: {
		workspacePath: string;
	} = $props();

	let loading = $state(false);
	let mutating = $state(false);
	let error = $state<string | null>(null);
	let actionError = $state<string | null>(null);
	let status = $state<WorkspaceGitStatus | null>(null);
	let filter = $state('');
	let loadSeq = 0;
	let copiedPath = $state<string | null>(null);
	let commitMessage = $state('');
	let commitFlash = $state('');
	let stagedOpen = $state(true);
	let changesOpen = $state(true);
	let discardConfirm = $state<DiscardConfirm | null>(null);

	const normalizedWorkspace = $derived(normalizeWorkspacePath(workspacePath));
	const workspaceAvailable = $derived(
		Boolean(normalizedWorkspace && normalizedWorkspace !== '/')
	);

	const allFiles = $derived(status?.files ?? []);

	const query = $derived(filter.trim().toLowerCase());

	function matchesFilter(file: GitFile): boolean {
		if (!query) return true;
		return file.path.toLowerCase().includes(query);
	}

	/** Staged section (VS Code "Staged Changes"). */
	const stagedFiles = $derived(
		allFiles.filter((f) => f.staged && matchesFilter(f))
	);

	/** Working tree section (VS Code "Changes") — unstaged + untracked. */
	const changeFiles = $derived(
		allFiles.filter((f) => hasUnstagedSide(f) && matchesFilter(f))
	);

	const stagedCount = $derived(allFiles.filter((f) => f.staged).length);
	const changesCount = $derived(allFiles.filter((f) => hasUnstagedSide(f)).length);

	const isClean = $derived(status?.is_repo === true && stagedCount === 0 && changesCount === 0);

	const discardConfirmDescription = $derived(
		discardConfirm?.kind === 'one'
			? `Discard changes to “${discardConfirm.path}”? Tracked files restore to HEAD. Untracked files are deleted. This cannot be undone.`
			: discardConfirm?.kind === 'all'
				? `Discard all ${changeFiles.length} unstaged change${changeFiles.length === 1 ? '' : 's'}? Tracked files restore to HEAD. Untracked files are deleted. This cannot be undone.`
				: 'Tracked files restore to HEAD. Untracked files are deleted. This cannot be undone.'
	);

	async function load() {
		const seq = ++loadSeq;
		if (!workspaceAvailable) {
			status = null;
			error = null;
			loading = false;
			return;
		}
		loading = true;
		error = null;
		try {
			const result = await getWorkspaceGitStatus(normalizedWorkspace, 'all');
			if (seq !== loadSeq) return;
			status = result;
		} catch (err) {
			if (seq !== loadSeq) return;
			status = null;
			error = err instanceof Error ? err.message : 'Failed to load git status';
		} finally {
			if (seq === loadSeq) loading = false;
		}
	}

	function openDiff(path: string) {
		shellStore.openGitDiffForActive(path);
	}

	function statusBadge(file: GitFile): string {
		if (file.untracked) return 'U';
		switch (file.status) {
			case 'modified':
				return 'M';
			case 'added':
				return 'A';
			case 'deleted':
				return 'D';
			case 'renamed':
				return 'R';
			case 'conflict':
				return '!';
			default:
				return file.status.slice(0, 1).toUpperCase() || '?';
		}
	}

	function fileName(path: string): string {
		return path.split(/[/\\]/).filter(Boolean).pop() || path;
	}

	function fileDir(path: string): string {
		const parts = path.split(/[/\\]/).filter(Boolean);
		if (parts.length <= 1) return '';
		return parts.slice(0, -1).join('/');
	}

	async function copyPath(path: string) {
		try {
			await navigator.clipboard.writeText(path);
			copiedPath = path;
			setTimeout(() => {
				if (copiedPath === path) copiedPath = null;
			}, 1200);
		} catch {
			// ignore
		}
	}

	function addPathToChat(path: string) {
		shellStore.addWebContextForActive({
			kind: 'file',
			title: fileName(path),
			source: `workspace-file:${path}`,
			content: ''
		});
		shellStore.requestComposerFocus();
	}

	async function runMutation(action: () => Promise<unknown>) {
		if (mutating) return;
		mutating = true;
		actionError = null;
		commitFlash = '';
		try {
			await action();
			await load();
		} catch (err) {
			actionError = err instanceof Error ? err.message : 'Git action failed';
		} finally {
			mutating = false;
		}
	}

	function stagePath(path: string) {
		return runMutation(() => stageWorkspaceGitPaths(normalizedWorkspace, [path]));
	}

	function unstagePath(path: string) {
		return runMutation(() => unstageWorkspaceGitPaths(normalizedWorkspace, [path]));
	}

	function requestDiscard(path: string) {
		if (mutating) return;
		discardConfirm = { kind: 'one', path };
	}

	function requestDiscardAll() {
		if (mutating || changeFiles.length === 0) return;
		discardConfirm = { kind: 'all' };
	}

	function confirmDiscard() {
		const pending = discardConfirm;
		discardConfirm = null;
		if (!pending) return;
		if (pending.kind === 'one') {
			return runMutation(() =>
				discardWorkspaceGitPaths(normalizedWorkspace, [pending.path])
			);
		}
		const paths = changeFiles.map((f) => f.path);
		if (!paths.length) return;
		return runMutation(() => discardWorkspaceGitPaths(normalizedWorkspace, paths));
	}

	function stageAllChanges() {
		const paths = changeFiles.map((f) => f.path);
		if (!paths.length) return;
		return runMutation(() => stageWorkspaceGitPaths(normalizedWorkspace, paths));
	}

	function unstageAll() {
		const paths = stagedFiles.map((f) => f.path);
		if (!paths.length) return;
		return runMutation(() => unstageWorkspaceGitPaths(normalizedWorkspace, paths));
	}

	async function commit() {
		const message = commitMessage.trim();
		if (!message || mutating || stagedCount === 0) return;
		await runMutation(async () => {
			const result = await commitWorkspaceGit(normalizedWorkspace, message);
			commitMessage = '';
			commitFlash = result.sha ? `Committed ${result.sha}` : 'Committed';
		});
	}

	$effect(() => {
		void normalizedWorkspace;
		void load();
	});
</script>

{#snippet fileRow(file: GitFile, section: 'staged' | 'changes')}
	<li class="git-file-row" title={file.path}>
		<button type="button" class="git-file-main" onclick={() => openDiff(file.path)}>
			<span class="git-file-icon" aria-hidden="true">
				<FileTypeIcon path={file.path} size={16} />
			</span>
			<span class="git-file-labels">
				<span class="git-file-name">{fileName(file.path)}</span>
				{#if fileDir(file.path)}
					<span class="git-file-dir">{fileDir(file.path)}</span>
				{/if}
			</span>
		</button>
		<span class="git-row-actions">
			{#if section === 'changes'}
				<button
					type="button"
					class="git-icon-btn"
					title="Stage"
					aria-label="Stage"
					disabled={mutating}
					onclick={() => void stagePath(file.path)}
				>
					<Plus size={13} />
				</button>
				<button
					type="button"
					class="git-icon-btn danger"
					title="Discard"
					aria-label="Discard"
					disabled={mutating}
					onclick={() => requestDiscard(file.path)}
				>
					<RotateCcw size={13} />
				</button>
			{:else}
				<button
					type="button"
					class="git-icon-btn"
					title="Unstage"
					aria-label="Unstage"
					disabled={mutating}
					onclick={() => void unstagePath(file.path)}
				>
					<Minus size={13} />
				</button>
			{/if}
			<button
				type="button"
				class="git-icon-btn"
				title={copiedPath === file.path ? 'Copied' : 'Copy path'}
				aria-label="Copy path"
				onclick={() => void copyPath(file.path)}
			>
				<Copy size={13} />
			</button>
			<button
				type="button"
				class="git-icon-btn"
				title="Add path to chat"
				aria-label="Add path to chat"
				onclick={() => addPathToChat(file.path)}
			>
				<MessageSquarePlus size={13} />
			</button>
		</span>
		<span
			class="git-badge"
			class:untracked={file.untracked}
			class:deleted={file.status === 'deleted'}
			class:added={file.status === 'added' || file.untracked}
			aria-label={file.status}
		>
			{statusBadge(file)}
		</span>
	</li>
{/snippet}

<div class="git-changes">
	<div class="git-changes-header">
		<div class="git-meta-row">
			<div class="git-meta">
				{#if status?.is_repo && status.branch}
					<span class="git-branch" title={status.upstream || status.branch}
						>{status.branch}</span
					>
					{#if !isClean}
						<span class="git-summary"
							>{stagedCount} staged · {changesCount} changes</span
						>
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
				onclick={() => void load()}
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
							void commit();
						}
					}}
					aria-label="Commit message"
				/>
				<button
					type="button"
					class="git-commit-btn"
					disabled={mutating || stagedCount === 0 || !commitMessage.trim()}
					onclick={() => void commit()}
				>
					Commit
				</button>
			</div>
			{#if commitFlash}
				<p class="git-flash" role="status">{commitFlash}</p>
			{/if}
		{/if}

		<input
			class="git-filter"
			type="text"
			placeholder="Filter changed files…"
			bind:value={filter}
			aria-label="Filter changed files"
		/>

		{#if actionError}
			<p class="git-action-error" role="alert">{actionError}</p>
		{/if}
	</div>

	{#if !workspaceAvailable}
		<div class="git-state">Select a workspace to see git changes.</div>
	{:else if loading && !status}
		<div class="git-state">
			<Loader size={16} stroke-width={2} class="git-spinner" />
			<span>Loading changes…</span>
		</div>
	{:else if error}
		<div class="git-state git-error">{error}</div>
	{:else if status && !status.is_repo}
		<div class="git-state">{status.message || 'This workspace is not a git repository.'}</div>
	{:else if isClean && !query}
		<div class="git-state">Working tree clean.</div>
	{:else if stagedFiles.length === 0 && changeFiles.length === 0}
		<div class="git-state">No matching changed files.</div>
	{:else}
		<div class="git-list-scroll scrollbar-none">
			{#if stagedFiles.length > 0 || stagedCount > 0}
				<section class="git-section git-section-staged">
					<div class="git-section-header">
						<button
							type="button"
							class="git-section-toggle"
							onclick={() => (stagedOpen = !stagedOpen)}
							aria-expanded={stagedOpen}
						>
							<span class="git-section-chevron">
								{#if stagedOpen}
									<ChevronDown size={13} stroke-width={2} />
								{:else}
									<ChevronRight size={13} stroke-width={2} />
								{/if}
							</span>
							<span class="git-section-title">Staged Changes</span>
							<span class="git-section-count">{stagedFiles.length}</span>
						</button>
						{#if stagedFiles.length > 0}
							<button
								type="button"
								class="git-section-action"
								title="Unstage all"
								disabled={mutating}
								onclick={() => void unstageAll()}
							>
								<Minus size={13} />
							</button>
						{/if}
					</div>
					{#if stagedOpen && stagedFiles.length > 0}
						<ul class="git-file-list">
							{#each stagedFiles as file (file.path + ':staged')}
								{@render fileRow(file, 'staged')}
							{/each}
						</ul>
					{/if}
				</section>
			{/if}

			{#if changeFiles.length > 0 || changesCount > 0}
				<section class="git-section">
					<div class="git-section-header">
						<button
							type="button"
							class="git-section-toggle"
							onclick={() => (changesOpen = !changesOpen)}
							aria-expanded={changesOpen}
						>
							<span class="git-section-chevron">
								{#if changesOpen}
									<ChevronDown size={13} stroke-width={2} />
								{:else}
									<ChevronRight size={13} stroke-width={2} />
								{/if}
							</span>
							<span class="git-section-title">Changes</span>
							<span class="git-section-count">{changeFiles.length}</span>
						</button>
						{#if changeFiles.length > 0}
							<button
								type="button"
								class="git-section-action danger"
								title="Discard all"
								disabled={mutating}
								onclick={requestDiscardAll}
							>
								<RotateCcw size={13} />
							</button>
							<button
								type="button"
								class="git-section-action"
								title="Stage all"
								disabled={mutating}
								onclick={() => void stageAllChanges()}
							>
								<Plus size={13} />
							</button>
						{/if}
					</div>
					{#if changesOpen && changeFiles.length > 0}
						<ul class="git-file-list">
							{#each changeFiles as file (file.path + ':changes')}
								{@render fileRow(file, 'changes')}
							{/each}
						</ul>
					{/if}
				</section>
			{/if}

			{#if status?.truncated}
				<p class="git-truncated">Showing first {status.files.length} files.</p>
			{/if}
		</div>
	{/if}
</div>

<ConfirmActionModal
	open={Boolean(discardConfirm)}
	title={discardConfirm?.kind === 'all' ? 'Discard all local changes?' : 'Discard local changes?'}
	description={discardConfirmDescription}
	confirmLabel={discardConfirm?.kind === 'all' ? 'Discard all' : 'Discard'}
	onCancel={() => (discardConfirm = null)}
	onConfirm={() => void confirmDiscard()}
/>

<style>
	.git-changes {
		display: flex;
		flex-direction: column;
		height: 100%;
		min-height: 0;
		background: #fff;
	}

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
		color: var(--text-primary, #111);
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
		background: var(--surface-elevated, #fff);
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
		background: var(--surface-elevated, #fff);
		color: var(--text-primary, #111);
	}

	.git-commit-input:focus {
		outline: none;
		border-color: var(--accent, #3b82f6);
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
			#1f2933
		);
		color: #fff;
		font-size: 13px;
		font-weight: 650;
		cursor: pointer;
		box-shadow: 0 6px 18px var(--hero-composer-glow-soft, transparent);
		transition:
			background var(--duration-fast, 150ms) var(--ease-smooth, ease),
			box-shadow var(--duration-fast, 150ms) var(--ease-smooth, ease),
			opacity var(--duration-fast, 150ms) var(--ease-smooth, ease);
	}

	.git-commit-btn:hover:not(:disabled) {
		background: color-mix(
			in srgb,
			var(--hero-composer-glow-color, var(--accent)) 82%,
			#1f2933
		);
		box-shadow: 0 8px 22px var(--hero-composer-glow-strong, transparent);
	}

	.git-commit-btn:disabled {
		opacity: 0.4;
		cursor: not-allowed;
		box-shadow: none;
	}

	.git-filter {
		width: 100%;
		box-sizing: border-box;
		border: 1px solid var(--border-subtle, rgba(0, 0, 0, 0.1));
		border-radius: 8px;
		padding: 7px 10px;
		font-size: 13px;
		background: var(--surface-elevated, #fff);
		color: var(--text-primary, #111);
	}

	.git-filter:focus {
		outline: none;
		border-color: var(--accent, #3b82f6);
	}

	.git-flash {
		margin: 0;
		font-size: 12px;
		color: #15803d;
	}

	.git-action-error {
		margin: 0;
		font-size: 12px;
		color: var(--status-error, #b91c1c);
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
		color: var(--status-error, #b91c1c);
	}

	.git-state :global(.git-spinner) {
		animation: git-spin 0.7s linear infinite;
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

	/* Mirrors sidebar WorkspaceGroup: soft pill group + nested session-like rows. */
	.git-section {
		display: flex;
		flex-direction: column;
		gap: 4px;
		border-radius: 8px;
		padding: 2px;
		border: 1px solid
			color-mix(in srgb, var(--workspace-inactive-color, #9a9a9f) 14%, transparent);
		background: linear-gradient(
			135deg,
			color-mix(in srgb, var(--workspace-inactive-color, #9a9a9f) 16%, transparent),
			color-mix(in srgb, var(--workspace-inactive-color, #9a9a9f) 6%, transparent)
		);
		transition:
			background var(--duration-fast, 150ms) var(--ease-smooth, ease),
			border-color var(--duration-fast, 150ms) var(--ease-smooth, ease),
			box-shadow var(--duration-fast, 150ms) var(--ease-smooth, ease);
	}

	/* Staged = “active” workspace group: hero glow surface like the focused sidebar group. */
	.git-section-staged {
		--git-section-accent: var(--hero-composer-glow-color, var(--accent));
		/* Recompute session-row tokens so nested file hovers pick up the active tint. */
		--session-row-bg-hover: color-mix(in srgb, var(--git-section-accent) 11%, transparent);
		--session-row-ring: color-mix(in srgb, var(--git-section-accent) 36%, var(--border-soft));
		background: linear-gradient(
			135deg,
			color-mix(in srgb, var(--git-section-accent) 31%, transparent),
			color-mix(in srgb, var(--git-section-accent) 12%, transparent)
		);
		border-color: color-mix(in srgb, var(--git-section-accent) 26%, transparent);
		box-shadow: 0 8px 22px color-mix(in srgb, var(--git-section-accent) 8%, transparent);
	}

	.git-section-staged:hover {
		background: linear-gradient(
			135deg,
			color-mix(in srgb, var(--git-section-accent) 38%, transparent),
			color-mix(in srgb, var(--git-section-accent) 16%, transparent)
		);
	}

	.git-section-header {
		display: flex;
		align-items: center;
		gap: 2px;
		padding: 0 2px 0 0;
	}

	.git-section-toggle {
		display: flex;
		align-items: center;
		gap: 6px;
		flex: 1;
		min-width: 0;
		border: none;
		border-radius: 7px;
		padding: 6px 8px;
		background: transparent;
		color: var(--workspace-inactive-color, #9a9a9f);
		font-size: 11px;
		font-weight: 600;
		text-transform: uppercase;
		letter-spacing: 0.02em;
		text-align: left;
		cursor: pointer;
	}

	.git-section:hover .git-section-toggle {
		color: var(--text-muted);
	}

	.git-section-staged .git-section-toggle {
		color: var(--text-main);
	}

	.git-section-staged .git-section-chevron {
		color: var(--git-section-accent);
	}

	.git-section-chevron {
		display: grid;
		place-items: center;
		flex-shrink: 0;
		color: inherit;
	}

	.git-section-title {
		min-width: 0;
		flex: 1;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.git-section-count {
		flex-shrink: 0;
		font-size: 10px;
		font-weight: 600;
		color: var(--text-soft);
		background: rgba(15, 23, 42, 0.06);
		border-radius: 999px;
		padding: 1px 6px;
	}

	.git-section-staged .git-section-count {
		color: color-mix(in srgb, var(--git-section-accent) 55%, #1f2933);
		background: color-mix(in srgb, var(--git-section-accent) 18%, transparent);
	}

	.git-section-action {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 26px;
		height: 26px;
		margin-right: 2px;
		border: none;
		border-radius: 6px;
		background: transparent;
		color: var(--text-muted);
		cursor: pointer;
	}

	.git-section-action:hover:not(:disabled) {
		background: color-mix(in srgb, var(--workspace-inactive-color, #9a9a9f) 18%, transparent);
		color: var(--text-primary, #111);
	}

	.git-section-action.danger:hover:not(:disabled) {
		background: rgba(185, 28, 28, 0.1);
		color: #b91c1c;
	}

	.git-section-staged .git-section-action:hover:not(:disabled) {
		background: color-mix(in srgb, var(--git-section-accent) 20%, transparent);
		color: var(--git-section-accent);
	}

	.git-section-action:disabled {
		opacity: 0.4;
		cursor: not-allowed;
	}

	/* Nested under group header — same visual tab as sidebar sessions under a workspace. */
	.git-file-list {
		list-style: none;
		margin: 0;
		padding: 0 2px 2px;
		display: flex;
		flex-direction: column;
		gap: 2px;
		/* Small indent so files sit under the group, like SessionRow under WorkspaceGroup. */
		padding-left: 10px;
	}

	.git-file-row {
		display: flex;
		align-items: center;
		gap: 2px;
		width: 100%;
		border-radius: 8px;
		padding: 0 4px 0 0;
		color: var(--text-primary, #111);
		font-size: 13px;
		transition:
			background-color var(--duration-fast, 150ms) var(--ease-smooth, ease),
			box-shadow var(--duration-fast, 150ms) var(--ease-smooth, ease);
	}

	.git-file-row:hover {
		background: var(--session-row-bg-hover, rgba(0, 0, 0, 0.04));
		box-shadow: inset 0 0 0 1px var(--session-row-ring, rgba(0, 0, 0, 0.06));
	}

	.git-file-row:hover .git-row-actions {
		opacity: 1;
	}

	.git-file-main {
		display: flex;
		align-items: center;
		gap: 8px;
		flex: 1;
		min-width: 0;
		border: none;
		border-radius: 8px;
		padding: 6px 6px 6px 8px;
		background: transparent;
		color: inherit;
		font: inherit;
		text-align: left;
		cursor: pointer;
	}

	.git-file-icon {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		flex: 0 0 16px;
		width: 16px;
		height: 16px;
	}

	.git-file-labels {
		display: flex;
		align-items: baseline;
		gap: 6px;
		flex: 1;
		min-width: 0;
		overflow: hidden;
	}

	.git-file-name {
		flex: 0 1 auto;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		font-size: 13px;
		color: var(--text-primary, #111);
	}

	.git-file-dir {
		flex: 1 1 auto;
		min-width: 0;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		font-size: 11px;
		color: var(--text-muted);
		font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
	}

	/* Rightmost column — after action icons. */
	.git-badge {
		flex: 0 0 auto;
		min-width: 18px;
		margin-left: 2px;
		padding: 0 2px;
		text-align: center;
		font-size: 11px;
		font-weight: 700;
		font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
		color: #b45309;
	}

	.git-badge.untracked,
	.git-badge.added {
		color: #15803d;
	}

	.git-badge.deleted {
		color: #b91c1c;
	}

	.git-row-actions {
		display: inline-flex;
		gap: 1px;
		opacity: 0;
		flex: 0 0 auto;
	}

	.git-icon-btn {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 24px;
		height: 24px;
		border: none;
		border-radius: 5px;
		background: transparent;
		color: var(--text-muted);
		cursor: pointer;
	}

	.git-icon-btn:hover:not(:disabled) {
		background: rgba(0, 0, 0, 0.06);
		color: var(--text-primary, #111);
	}

	.git-icon-btn.danger:hover:not(:disabled) {
		color: #b91c1c;
		background: rgba(239, 68, 68, 0.1);
	}

	.git-icon-btn:disabled {
		opacity: 0.4;
		cursor: not-allowed;
	}

	.git-truncated {
		margin: 8px 12px 0;
		font-size: 12px;
		color: var(--text-muted);
	}
</style>
