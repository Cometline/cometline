<script lang="ts">
	import { ChevronDown, ChevronRight, File, Folder, FolderOpen, Loader } from '@lucide/svelte';
	import { tick } from 'svelte';
	import { listWikiFiles, listWorkspaceFiles } from '$lib/client/cometmind';
	import GitChangesBrowser from '$lib/components/GitChangesBrowser.svelte';
	import { shellStore } from '$lib/stores/shell.svelte';
	import { toWikiUiPath } from '$lib/wiki/paths';
	import { buildFileTree, dirKeysToExpandForPaths, type FileTreeNode } from '$lib/workspace/file-tree';
	import { type WebPanelTreeSource } from '$lib/workspace/web-panel-prefs';
	import { normalizeWorkspacePath } from '$lib/workspace/file-index';

	const LIST_LIMIT = 500;

	let {
		workspacePath,
		onSelectFile
	}: {
		workspacePath: string;
		onSelectFile: (path: string) => void;
	} = $props();

	let filter = $state('');
	let loading = $state(false);
	let error = $state<string | null>(null);
	let truncated = $state(false);
	let files = $state<string[]>([]);
	let expanded = $state<Record<string, boolean>>({});
	let loadSeq = 0;
	let filterInputEl = $state<HTMLInputElement | null>(null);
	let satisfiedFilterFocusRequestId = 0;

	const source = $derived(shellStore.webPanelBrowseSource);
	const normalizedWorkspace = $derived(normalizeWorkspacePath(workspacePath));
	const workspaceAvailable = $derived(
		Boolean(normalizedWorkspace && normalizedWorkspace !== '/')
	);
	const tree = $derived(buildFileTree(files));
	const showFileTree = $derived(source === 'wiki' || source === 'workspace');

	function setSource(next: WebPanelTreeSource) {
		if ((next === 'workspace' || next === 'changes') && !workspaceAvailable) return;
		// Records panel history so ←/→ moves between Wiki / Workspace / Changes.
		shellStore.setWebPanelBrowseSource(next);
	}

	function toggleDir(key: string) {
		expanded = { ...expanded, [key]: !expanded[key] };
	}

	function dirKey(parentKey: string, name: string): string {
		return parentKey ? `${parentKey}/${name}` : name;
	}

	function isExpanded(key: string): boolean {
		return expanded[key] ?? false;
	}

	function selectRelative(relativePath: string) {
		if (source === 'wiki') {
			onSelectFile(toWikiUiPath(relativePath));
			return;
		}
		onSelectFile(relativePath);
	}

	async function loadFiles() {
		const seq = ++loadSeq;
		const activeSource = source;
		if (activeSource === 'changes') {
			files = [];
			truncated = false;
			loading = false;
			error = null;
			return;
		}
		const query = filter.trim();
		loading = true;
		error = null;

		if (activeSource === 'workspace' && !workspaceAvailable) {
			files = [];
			truncated = false;
			expanded = {};
			loading = false;
			return;
		}

		try {
			const result =
				activeSource === 'wiki'
					? await listWikiFiles(query, LIST_LIMIT)
					: await listWorkspaceFiles(normalizedWorkspace, query, LIST_LIMIT);
			if (seq !== loadSeq) return;
			files = result.files;
			truncated = result.truncated;
			expanded = query ? dirKeysToExpandForPaths(files) : {};
		} catch (err) {
			if (seq !== loadSeq) return;
			files = [];
			truncated = false;
			if (!query) expanded = {};
			error = err instanceof Error ? err.message : 'Failed to load files';
		} finally {
			if (seq === loadSeq) loading = false;
		}
	}

	function applyFilterFocus() {
		const requestId = shellStore.fileTreeFilterFocusRequestId;
		if (!requestId || requestId === satisfiedFilterFocusRequestId) return;
		const el = filterInputEl;
		if (!el) return;
		satisfiedFilterFocusRequestId = requestId;
		focusFilter();
	}

	export function focusFilter() {
		const el = filterInputEl;
		if (!el) return;
		shellStore.setFocusedPane('web');
		el.focus({ preventScroll: true });
		el.select();
	}

	function trackFilterInput(node: HTMLInputElement) {
		filterInputEl = node;
		applyFilterFocus();
		return {
			destroy() {
				if (filterInputEl === node) filterInputEl = null;
			}
		};
	}

	$effect(() => {
		void [source, filter, normalizedWorkspace];
		void loadFiles();
	});

	$effect(() => {
		const requestId = shellStore.fileTreeFilterFocusRequestId;
		if (!requestId) return;
		void tick().then(applyFilterFocus);
	});

</script>

{#snippet treeNodes(nodes: FileTreeNode[], parentKey: string)}
	<ul class="file-tree-list" role="tree">
		{#each nodes as node (dirKey(parentKey, node.name))}
			{@const key = dirKey(parentKey, node.name)}
			{@const hasChildren = Boolean(node.children?.length)}
			<li class="file-tree-item" role="treeitem" aria-selected="false" aria-expanded={hasChildren ? isExpanded(key) : undefined}>
				{#if hasChildren}
					<button
						type="button"
						class="file-tree-row"
						onclick={() => toggleDir(key)}
					>
						<span class="file-tree-chevron" aria-hidden="true">
							{#if isExpanded(key)}
								<ChevronDown size={14} />
							{:else}
								<ChevronRight size={14} />
							{/if}
						</span>
						<span class="file-tree-icon" aria-hidden="true">
							{#if isExpanded(key)}
								<FolderOpen size={14} />
							{:else}
								<Folder size={14} />
							{/if}
						</span>
						<span class="file-tree-label">{node.name}</span>
					</button>
					{#if isExpanded(key) && node.children}
						<div class="file-tree-children">
							{@render treeNodes(node.children, key)}
						</div>
					{/if}
				{:else if node.path}
					<button
						type="button"
						class="file-tree-row file-tree-file"
						onclick={() => selectRelative(node.path!)}
						title={node.path}
					>
						<span class="file-tree-chevron file-tree-chevron-spacer" aria-hidden="true"></span>
						<span class="file-tree-icon" aria-hidden="true">
							<File size={14} />
						</span>
						<span class="file-tree-label">{node.name}</span>
					</button>
				{/if}
			</li>
		{/each}
	</ul>
{/snippet}

<div class="file-tree-browser">
	<div class="file-tree-header">
		<div class="source-toggle" role="group" aria-label="File tree source">
			<button
				type="button"
				class="source-toggle-btn"
				class:active={source === 'wiki'}
				onclick={() => setSource('wiki')}
			>
				Wiki
			</button>
			<button
				type="button"
				class="source-toggle-btn"
				class:active={source === 'workspace'}
				disabled={!workspaceAvailable}
				title={workspaceAvailable ? 'Workspace files' : 'Select a workspace to browse files'}
				onclick={() => setSource('workspace')}
			>
				Workspace
			</button>
			<button
				type="button"
				class="source-toggle-btn"
				class:active={source === 'changes'}
				disabled={!workspaceAvailable}
				title={workspaceAvailable ? 'Git changes' : 'Select a workspace to see git changes'}
				onclick={() => setSource('changes')}
			>
				Changes
			</button>
		</div>
		{#if showFileTree}
			<input
				use:trackFilterInput
				class="file-tree-filter"
				type="search"
				placeholder={source === 'wiki' ? 'Filter wiki files…' : 'Filter workspace files…'}
				bind:value={filter}
				aria-label="Filter files"
			/>
		{/if}
	</div>

	{#if source === 'changes'}
		<div class="file-tree-changes">
			<GitChangesBrowser workspacePath={normalizedWorkspace} />
		</div>
	{:else if source === 'workspace' && !workspaceAvailable}
		<div class="file-tree-state">Select a workspace to browse its files.</div>
	{:else if loading && files.length === 0}
		<div class="file-tree-state">
			<Loader size={16} stroke-width={2} class="file-tree-spinner" />
			<span>Loading files…</span>
		</div>
	{:else if error}
		<div class="file-tree-state file-tree-error">{error}</div>
	{:else if tree.length === 0}
		<div class="file-tree-state">
			{filter.trim() ? 'No matching files.' : 'No files found.'}
		</div>
	{:else}
		<div class="file-tree-scroll scrollbar-none">
			{@render treeNodes(tree, '')}
			{#if truncated}
				<p class="file-tree-truncated">
					Showing first {LIST_LIMIT} matches. Refine the filter to find more.
				</p>
			{/if}
		</div>
	{/if}
</div>

<style>
	.file-tree-browser {
		display: flex;
		flex-direction: column;
		height: 100%;
		min-height: 0;
		background: #fff;
	}

	.file-tree-changes {
		flex: 1;
		min-height: 0;
		display: flex;
		flex-direction: column;
		overflow: hidden;
	}

	.file-tree-changes :global(.git-changes),
	.file-tree-changes :global(.git-diff-view) {
		flex: 1;
		min-height: 0;
	}

	.file-tree-header {
		display: flex;
		flex-direction: column;
		gap: 8px;
		padding: 10px 12px;
		border-bottom: 1px solid var(--border-subtle, rgba(0, 0, 0, 0.08));
	}

	.source-toggle {
		display: flex;
		gap: 2px;
		padding: 2px;
		border-radius: 8px;
		background: var(--surface-muted, rgba(0, 0, 0, 0.04));
	}

	.source-toggle-btn {
		flex: 1;
		border: none;
		border-radius: 6px;
		padding: 6px 10px;
		background: transparent;
		color: var(--text-muted);
		font-size: 12px;
		font-weight: 550;
		cursor: pointer;
	}

	.source-toggle-btn:disabled {
		opacity: 0.45;
		cursor: not-allowed;
	}

	.source-toggle-btn.active {
		background: var(--surface-elevated, #fff);
		color: var(--text-primary, #111);
		box-shadow: 0 0 0 1px rgba(0, 0, 0, 0.06);
	}

	.file-tree-filter {
		width: 100%;
		box-sizing: border-box;
		border: 1px solid var(--border-subtle, rgba(0, 0, 0, 0.1));
		border-radius: 8px;
		padding: 7px 10px;
		font-size: 13px;
		background: var(--surface-elevated, #fff);
		color: var(--text-primary, #111);
	}

	.file-tree-filter:focus {
		outline: none;
		border-color: var(--accent, #3b82f6);
	}

	.file-tree-scroll {
		flex: 1;
		min-height: 0;
		overflow: auto;
		padding: 6px 8px 12px;
	}

	.file-tree-list {
		list-style: none;
		margin: 0;
		padding: 0;
	}

	.file-tree-children {
		padding-left: 12px;
	}

	.file-tree-row {
		display: flex;
		align-items: center;
		gap: 4px;
		width: 100%;
		border: none;
		border-radius: 6px;
		padding: 4px 6px;
		background: transparent;
		color: var(--text-primary, #111);
		font-size: 13px;
		text-align: left;
		cursor: pointer;
	}

	.file-tree-row:hover {
		background: var(--surface-muted, rgba(0, 0, 0, 0.04));
	}

	.file-tree-chevron,
	.file-tree-icon {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 16px;
		height: 16px;
		flex: 0 0 16px;
		color: var(--text-muted);
	}

	.file-tree-chevron-spacer {
		visibility: hidden;
	}

	.file-tree-label {
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.file-tree-state {
		display: flex;
		align-items: center;
		justify-content: center;
		gap: 8px;
		min-height: 120px;
		padding: 24px;
		color: var(--text-muted);
		font-size: 13px;
		text-align: center;
	}

	.file-tree-error {
		color: var(--status-error);
	}

	.file-tree-state :global(.file-tree-spinner) {
		animation: file-tree-spin 0.7s linear infinite;
	}

	.file-tree-truncated {
		margin: 10px 6px 0;
		padding: 0;
		color: var(--text-muted);
		font-size: 12px;
	}

	@keyframes file-tree-spin {
		to {
			transform: rotate(360deg);
		}
	}

	@media (prefers-reduced-motion: reduce) {
		.file-tree-state :global(.file-tree-spinner) {
			animation: none;
		}
	}
</style>
