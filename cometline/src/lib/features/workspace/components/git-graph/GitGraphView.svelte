<script lang="ts">
	import { Loader, RefreshCw } from '@lucide/svelte';
	import ConfirmActionModal from '#lib/components/ConfirmActionModal.svelte';
	import GitGraphDetail from '#lib/features/workspace/components/git-graph/GitGraphDetail.svelte';
	import GitGraphMenu from '#lib/features/workspace/components/git-graph/GitGraphMenu.svelte';
	import GitGraphRow from '#lib/features/workspace/components/git-graph/GitGraphRow.svelte';
	import { createGitGraphViewController } from '#lib/features/workspace/git-graph-view.svelte.js';
	import {
		commitMenuItems,
		dialogInputValue,
		graphStrokes,
		laneColor,
		laneColumnCount,
		refMenuItems
	} from '#lib/features/workspace/git-graph.js';
	import { workspaceChangeVersion } from '#lib/features/workspace/workspace-change.svelte.js';

	let {
		workspacePath,
		onShowChanges,
		onOpenCommitFile
	}: {
		workspacePath: string;
		onShowChanges?: () => void;
		onOpenCommitFile?: (sha: string, path: string) => void;
	} = $props();

	const graph = createGitGraphViewController({ getWorkspacePath: () => workspacePath });

	let detailHeight = $state(0);

	const columns = $derived(laneColumnCount(graph.commits));
	const expansion = $derived.by(() => {
		if (!graph.selectedSha || detailHeight <= 0) return undefined;
		const index = graph.commits.findIndex((commit) => commit.sha === graph.selectedSha);
		if (index < 0) return undefined;
		return { index, height: detailHeight };
	});
	const strokes = $derived(
		graphStrokes(graph.commits, graph.uncommittedCount > 0, expansion)
	);
	const graphWidth = $derived(columns * 16 + 8);
	const graphHeight = $derived(
		(graph.commits.length + (graph.uncommittedCount > 0 ? 1 : 0)) * 28 + detailHeight
	);
	const menuItems = $derived.by(() => {
		const menu = graph.menu;
		if (!menu) return [];
		if (menu.kind === 'commit') {
			const row = graph.commits.find((item) => item.sha === menu.sha);
			return row ? commitMenuItems(row, graph.graph?.head_sha ?? '') : [];
		}
		return refMenuItems(menu.ref);
	});
	const dialog = $derived(graph.dialog);
	const dialogTitle = $derived(
		dialog?.kind === 'create-branch'
			? 'Create branch'
			: dialog?.kind === 'create-tag'
				? 'Create tag'
				: dialog?.kind === 'rename-branch'
					? 'Rename branch'
					: dialog?.kind === 'rename-tag'
						? 'Rename tag'
						: dialog?.kind === 'delete-branch'
							? 'Delete branch?'
							: dialog?.kind === 'delete-tag'
								? 'Delete tag?'
								: ''
	);
	const dialogDescription = $derived(
		dialog?.kind === 'delete-branch'
			? `Delete local branch “${dialog.name}”? Unmerged branches are refused.`
			: dialog?.kind === 'delete-tag'
				? `Delete tag “${dialog.name}”? This does not delete a remote tag.`
				: 'Local only. Nothing is pushed.'
	);
	const dialogInput = $derived(dialogInputValue(dialog));

	$effect(() => {
		void [workspacePath, workspaceChangeVersion(workspacePath)];
		void graph.load();
	});
</script>

<div class="git-graph">
	<div class="git-graph-meta">
		<span class="head">
			{#if graph.graph?.head_branch}
				<span class="here-dot" aria-hidden="true"></span>
				{graph.graph.head_branch}
			{:else if graph.headLabel}
				<span class="here-dot" aria-hidden="true"></span>
				detached {graph.headLabel}
			{:else}
				Graph
			{/if}
		</span>
		<button
			type="button"
			class="refresh"
			aria-label="Refresh git graph"
			disabled={graph.loading || graph.mutating || !graph.workspaceAvailable}
			onclick={() => void graph.load()}
		>
			<RefreshCw size={14} class={graph.loading ? 'spin' : ''} />
		</button>
	</div>

	{#if !graph.workspaceAvailable}
		<div class="state">Select a workspace to see the git graph.</div>
	{:else if graph.loading && graph.commits.length === 0}
		<div class="state">
			<Loader size={16} class="spin" />
			<span>Loading graph…</span>
		</div>
	{:else if graph.error}
		<div class="state error">{graph.error}</div>
	{:else if graph.graph && !graph.graph.is_repo}
		<div class="state">{graph.graph.message || 'This workspace is not a git repository.'}</div>
	{:else}
		<div class="rows scrollbar-none">
			<svg
				class="graph-rails"
				width={graphWidth}
				height={graphHeight}
				aria-hidden="true"
			>
				{#each strokes as stroke, index (index)}
					<path d={stroke.d} fill="none" stroke={stroke.color} stroke-width="1.5" />
				{/each}
				{#if graph.uncommittedCount > 0}
					<circle
						cx={(graph.commits[0]?.lane ?? 0) * 16 + 8}
						cy="14"
						r="3.5"
						fill={laneColor(graph.commits[0]?.lane ?? 0)}
					/>
				{/if}
			</svg>
			{#if graph.uncommittedCount > 0}
				<button type="button" class="uncommitted" onclick={() => onShowChanges?.()}>
					<span class="lane-spacer" style:width="{graphWidth}px"></span>
					<strong>Uncommitted Changes ({graph.uncommittedCount})</strong>
				</button>
			{/if}
			{#each graph.commits as commit, index (commit.sha)}
				<GitGraphRow
					{commit}
					{columns}
					first={index === 0 && graph.uncommittedCount === 0}
					selected={graph.selectedSha === commit.sha}
					headSha={graph.graph?.head_sha ?? ''}
					onOpen={() => void graph.selectCommit(commit.sha)}
					onCommitMenu={graph.openCommitMenu}
					onRefMenu={graph.openRefMenu}
				/>
				{#if graph.selectedSha === commit.sha}
					<div class="detail-slot" bind:clientHeight={detailHeight}>
					<GitGraphDetail
						detail={graph.detail}
						loading={graph.detailLoading}
						error={graph.detailError}
						onClose={graph.closeDetail}
						onOpenFile={(path) => onOpenCommitFile?.(graph.selectedSha ?? '', path)}
					/>
					</div>
				{/if}
			{/each}
			{#if graph.graph?.truncated}
				<button type="button" class="more" disabled={graph.loading} onclick={() => void graph.loadMore()}>
					{graph.loading ? 'Loading…' : 'Load more'}
				</button>
			{/if}
		</div>
	{/if}

	{#if graph.actionError}
		<p class="action-error" role="alert">{graph.actionError}</p>
	{/if}
</div>

{#if graph.menu && menuItems.length > 0}
	<GitGraphMenu
		x={graph.menu.x}
		y={graph.menu.y}
		items={menuItems}
		onChoose={graph.choose}
		onClose={graph.closeMenu}
	/>
{/if}

<ConfirmActionModal
	open={Boolean(dialog)}
	title={dialogTitle}
	description={dialogDescription}
	confirmLabel={dialog?.kind.startsWith('delete') ? 'Delete' : 'Save'}
	confirmTone={dialog?.kind.startsWith('delete') ? 'danger' : 'accent'}
	showInput={Boolean(dialog) && !dialog?.kind.startsWith('delete')}
	bind:inputValue={
		() => dialogInput,
		(value) => graph.setDialogName(value)
	}
	inputPlaceholder="Name"
	onCancel={graph.cancelDialog}
	onConfirm={() => void graph.confirmDialog()}
/>

<style>
	.git-graph {
		display: flex;
		flex-direction: column;
		height: 100%;
		min-height: 0;
		background: var(--panel-bg);
	}

	.git-graph-meta {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 8px;
		padding: 8px 12px;
		border-bottom: 1px solid var(--border-subtle, rgba(0, 0, 0, 0.08));
	}

	.head {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		font-size: 12px;
		font-weight: 650;
		font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
		color: var(--text-primary, var(--color-111111));
	}

	.here-dot {
		width: 7px;
		height: 7px;
		border-radius: 50%;
		background: var(--accent);
		box-shadow: 0 0 0 3px color-mix(in srgb, var(--accent) 18%, transparent);
	}

	.refresh {
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

	.rows {
		position: relative;
		flex: 1;
		min-height: 0;
		overflow: auto;
		padding: 4px 0 12px;
	}

	.detail-slot {
		position: relative;
		z-index: 2;
		background: var(--panel-bg);
	}

	.graph-rails {
		position: absolute;
		top: 4px;
		left: 4px;
		pointer-events: none;
	}

	.state {
		display: flex;
		align-items: center;
		justify-content: center;
		gap: 8px;
		padding: 24px;
		color: var(--text-muted);
		font-size: 13px;
		text-align: center;
	}

	.error,
	.action-error {
		color: var(--status-error);
		font-size: 12px;
	}

	.action-error {
		margin: 0;
		padding: 8px 12px 12px;
	}

	.uncommitted {
		display: flex;
		align-items: center;
		gap: 8px;
		width: max-content;
		min-width: 100%;
		height: 28px;
		padding: 0 10px 0 4px;
		border: none;
		background: transparent;
		color: var(--text-main);
		font-size: 13px;
		text-align: left;
		cursor: pointer;
	}

	.uncommitted:hover {
		background: color-mix(in srgb, var(--accent) 8%, transparent);
	}

	.lane-spacer {
		display: block;
		flex: 0 0 auto;
		height: 1px;
	}

	.more {
		margin: 8px 12px;
		border: 1px solid var(--border-subtle, rgba(0, 0, 0, 0.1));
		border-radius: 8px;
		background: var(--surface-elevated, var(--panel-bg));
		color: var(--text-main);
		font-size: 13px;
		padding: 6px 10px;
		cursor: pointer;
	}

	:global(.spin) {
		animation: spin 0.7s linear infinite;
	}

	@keyframes spin {
		to {
			transform: rotate(360deg);
		}
	}
</style>
