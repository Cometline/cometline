<script lang="ts">
	import { ChevronDown, ChevronRight, Folder, FolderOpen, Loader } from '@lucide/svelte';
	import FileTypeIcon from '#lib/features/workspace/components/FileTypeIcon.svelte';
	import {
		createFileTreeBrowser,
		type FileTreeBrowserController
	} from '#lib/features/workspace/file-tree-browser.svelte.js';
	import type { FileTreeNode } from '#lib/features/workspace/file-tree.js';
	import type { FileTreeExpandSource } from '#lib/stores/shell.svelte.js';

	let {
		workspacePath,
		onSelectFile,
		source,
		filter = $bindable('')
	}: {
		workspacePath: string;
		onSelectFile: (path: string) => void;
		source: FileTreeExpandSource;
		filter?: string;
	} = $props();

	const panel: FileTreeBrowserController = createFileTreeBrowser({
		getWorkspacePath: () => workspacePath,
		getSource: () => source,
		getFilter: () => filter,
		onSelectFile: (path) => onSelectFile(path)
	});

	export function moveSelection(delta: number): boolean {
		return panel.moveSelection(delta);
	}

	export function activateSelection(): boolean {
		return panel.activateSelection();
	}

	export function handleTreeKey(event: KeyboardEvent): boolean {
		return panel.handleTreeKey(event);
	}
</script>

{#snippet treeNodes(nodes: FileTreeNode[], parentKey: string)}
	<ul class="file-tree-list" role="tree">
		{#each nodes as node (panel.dirKey(parentKey, node.name))}
			{@const key = panel.dirKey(parentKey, node.name)}
			{@const hasChildren = node.children !== undefined}
			{@const rowExpanded = hasChildren && panel.isExpanded(key)}
			<li
				class="file-tree-item"
				class:is-dir={hasChildren}
				class:is-expanded={rowExpanded}
				role="treeitem"
				aria-selected={panel.selectedKey === key}
				aria-expanded={hasChildren ? rowExpanded : undefined}
			>
				{#if hasChildren}
					<button
						type="button"
						class="file-tree-row file-tree-dir"
						class:selected={panel.selectedKey === key}
						data-tree-key={key}
						onmousedown={panel.keepPaneFocus}
						onclick={() => {
							panel.selectKey(key);
							panel.toggleDir(key);
						}}
					>
						<span class="file-tree-chevron" aria-hidden="true">
							{#if rowExpanded}
								<ChevronDown size={13} stroke-width={2} />
							{:else}
								<ChevronRight size={13} stroke-width={2} />
							{/if}
						</span>
						<span class="file-tree-icon file-tree-folder-icon" aria-hidden="true">
							{#if rowExpanded}
								<FolderOpen size={13} stroke-width={1.8} />
							{:else}
								<Folder size={13} stroke-width={1.8} />
							{/if}
						</span>
						<span class="file-tree-label">{node.name}</span>
						{#if panel.s.loadingDirectories[key]}
							<Loader size={12} stroke-width={2} class="file-tree-spinner" />
						{/if}
					</button>
					{#if rowExpanded && node.children}
						<div class="file-tree-children">
							{@render treeNodes(node.children, key)}
						</div>
					{/if}
				{:else if node.path}
					<button
						type="button"
						class="file-tree-row file-tree-file"
						class:selected={panel.selectedKey === key}
						data-tree-key={key}
						onmousedown={panel.keepPaneFocus}
						onclick={() => {
							panel.selectKey(key);
							panel.selectRelative(node.path!);
						}}
						title={node.path}
					>
						<span class="file-tree-icon" aria-hidden="true">
							<FileTypeIcon path={node.path} size={14} />
						</span>
						<span class="file-tree-label">{node.name}</span>
					</button>
				{/if}
			</li>
		{/each}
	</ul>
{/snippet}

<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
<div
	class="file-tree-browser"
	bind:this={panel.s.browserEl}
	tabindex="-1"
	role="region"
	aria-label={source === 'wiki' ? 'Wiki file tree' : 'Workspace file tree'}
	onkeydown={(event) => panel.handleTreeKey(event)}
>
	{#if source === 'workspace' && !panel.workspaceAvailable}
		<div class="file-tree-state">Select a workspace to browse its files.</div>
	{:else if panel.searching && panel.s.loading && panel.s.searchResults.length === 0}
		<div class="file-tree-state">
			<Loader size={16} stroke-width={2} class="file-tree-spinner" />
			<span>Loading files…</span>
		</div>
	{:else if !panel.searching && panel.s.loading && panel.s.files.length === 0}
		<div class="file-tree-state">
			<Loader size={16} stroke-width={2} class="file-tree-spinner" />
			<span>Loading files…</span>
		</div>
	{:else if panel.s.error}
		<div class="file-tree-state file-tree-error">{panel.s.error}</div>
	{:else if panel.searching && panel.s.searchResults.length === 0}
		<div class="file-tree-state">No matching files.</div>
	{:else if !panel.searching && panel.tree.length === 0}
		<div class="file-tree-state">No files found.</div>
	{:else if panel.searching}
		<div
			class="file-tree-scroll scrollbar-none"
			bind:this={panel.s.searchScrollEl}
			onscroll={panel.onSearchScroll}
		>
			<div class="file-search-virtual" style:height="{panel.searchWindow.height}px">
				<div
					class="file-search-virtual-inner"
					style:transform="translateY({panel.searchWindow.offset}px)"
				>
					{#each panel.visibleSearchResults as path, offset (path)}
						{@const index = panel.searchWindow.start + offset}
						<button
							type="button"
							class="file-tree-row file-tree-file file-search-row"
							class:selected={panel.selectedKey === path}
							data-tree-key={path}
							data-result-index={index}
							onmousedown={panel.keepPaneFocus}
							onclick={() => {
								panel.selectKey(path);
								panel.selectRelative(path);
							}}
							title={path}
						>
							<span class="file-tree-icon" aria-hidden="true">
								<FileTypeIcon {path} size={14} />
							</span>
							<span class="file-search-labels">
								<span class="file-search-name">{panel.fileName(path)}</span>
								{#if panel.fileDir(path)}
									<span class="file-search-dir">{panel.fileDir(path)}</span>
								{/if}
							</span>
						</button>
					{/each}
				</div>
			</div>
		</div>
	{:else}
		<div class="file-tree-scroll scrollbar-none">
			{@render treeNodes(panel.tree, '')}
		</div>
	{/if}
</div>

<style>
	.file-tree-browser {
		display: flex;
		flex-direction: column;
		height: 100%;
		min-height: 0;
		background: var(--panel-bg);
	}

	.file-tree-browser:focus {
		outline: none;
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

	.file-tree-item.is-dir {
		/* Keep dir + nested children as one column; no group surface chrome. */
		display: flex;
		flex-direction: column;
	}

	/*
	 * Nesting guide: VS Code / Cursor-style left border only, inactive group color.
	 * Nested lists stack these so each depth gets its own vertical guide.
	 */
	.file-tree-children {
		margin: 0;
		padding: 0 0 0 6px;
		border-left: 1px solid
			color-mix(
				in srgb,
				var(--workspace-inactive-color, var(--workspace-group-color)) 42%,
				transparent
			);
		margin-left: 8px; /* align under chevron/folder column */
	}

	.file-tree-row {
		display: flex;
		align-items: center;
		gap: 4px;
		width: 100%;
		border: none;
		border-radius: 6px;
		padding: 3px 6px;
		background: transparent;
		color: var(--text-primary, var(--color-111111));
		font-size: 13px;
		text-align: left;
		cursor: pointer;
	}

	.file-tree-row:hover {
		background: color-mix(
			in srgb,
			var(--workspace-inactive-color, var(--workspace-group-color)) 12%,
			transparent
		);
	}

	.file-tree-row.selected {
		background: color-mix(
			in srgb,
			var(--workspace-inactive-color, var(--workspace-group-color)) 18%,
			transparent
		);
	}

	.file-tree-row.file-tree-dir {
		color: var(--text-primary, var(--color-111111));
		font-weight: 500;
	}

	.file-tree-chevron,
	.file-tree-icon {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 16px;
		height: 16px;
		flex: 0 0 16px;
		color: var(--workspace-inactive-color, var(--workspace-group-color));
	}

	.file-tree-label {
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.file-search-virtual {
		position: relative;
		width: 100%;
	}

	.file-search-virtual-inner {
		position: absolute;
		top: 0;
		left: 0;
		right: 0;
	}

	.file-search-row {
		box-sizing: border-box;
		height: 22px;
		min-width: 0;
		overflow: hidden;
	}

	.file-search-labels {
		display: flex;
		align-items: baseline;
		gap: 8px;
		min-width: 0;
		flex: 1 1 auto;
	}

	.file-search-name {
		flex: 0 1 auto;
		min-width: 0;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		font-weight: 550;
	}

	.file-search-dir {
		flex: 1 1 auto;
		min-width: 0;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		color: var(--text-muted);
		font-size: 12px;
		font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
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
