<script lang="ts">
	import { tick, untrack } from 'svelte';
	import { ChevronDown, ChevronRight, Folder, FolderOpen, Loader } from '@lucide/svelte';
	import { listWikiFileChildren, listWorkspaceFileChildren } from '$lib/client/cometmind';
	import FileTypeIcon from '$lib/components/FileTypeIcon.svelte';
	import { shellStore, type FileTreeExpandSource } from '$lib/stores/shell.svelte';
	import { toWikiUiPath } from '$lib/wiki/paths';
	import { getCachedWikiFiles, refreshWikiFileIndex } from '$lib/wiki/wiki-file-index';
	import { rankMatchingFiles } from '$lib/workspace/file-search';
	import {
		getFileIndex,
		isFileIndexTruncated,
		normalizeWorkspacePath,
		refreshFileIndex,
		searchWorkspaceFiles
	} from '$lib/workspace/file-index';
	import {
		buildFileTree,
		dirKeysToExpandForPaths,
		flattenVisibleFileTreeRows,
		type FileTreeNode
	} from '$lib/workspace/file-tree';
	import {
		FILE_TREE_SEARCH_LIMIT,
		FILE_TREE_SEARCH_ROW_HEIGHT,
		virtualWindow
	} from '$lib/workspace/virtual-list';
	import { workspaceChangeVersion } from '$lib/workspace/workspace-change.svelte';

	const LIST_LIMIT = 10000;

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

	let loading = $state(false);
	let error = $state<string | null>(null);
	let files = $state<string[]>([]);
	let expanded = $state<Record<string, boolean>>({});
	let loadedDirectories = $state<Record<string, boolean>>({});
	let loadingDirectories = $state<Record<string, boolean>>({});
	let selectedKey = $state<string | null>(null);
	let loadSeq = 0;
	let browserEl = $state<HTMLDivElement | null>(null);
	let searchResults = $state<string[]>([]);
	let searchScrollEl = $state<HTMLDivElement | null>(null);
	let searchScrollTop = $state(0);
	let searchViewportHeight = $state(320);

	const normalizedWorkspace = $derived(normalizeWorkspacePath(workspacePath));
	const workspaceAvailable = $derived(
		Boolean(normalizedWorkspace && normalizedWorkspace !== '/')
	);
	const searching = $derived(Boolean(filter.trim()));
	const tree = $derived(buildFileTree(files));
	const visibleRows = $derived(
		searching
			? searchResults.map((path) => ({
					kind: 'file' as const,
					key: path,
					name: fileName(path),
					path
				}))
			: flattenVisibleFileTreeRows(tree, expanded)
	);
	const searchWindow = $derived(
		virtualWindow(
			searchResults.length,
			searchScrollTop,
			searchViewportHeight,
			FILE_TREE_SEARCH_ROW_HEIGHT
		)
	);
	const visibleSearchResults = $derived(searchResults.slice(searchWindow.start, searchWindow.end));

	function fileName(path: string): string {
		return path.split(/[/\\]/).filter(Boolean).pop() || path;
	}

	function fileDir(path: string): string {
		const parts = path.split(/[/\\]/).filter(Boolean);
		if (parts.length <= 1) return '';
		return parts.slice(0, -1).join('/');
	}

	function persistExpanded(next: Record<string, boolean>) {
		shellStore.setFileTreeExpanded(source, next);
	}

	function setDirExpanded(key: string, nextExpanded: boolean) {
		if ((expanded[key] ?? false) === nextExpanded) return;
		const next = { ...expanded, [key]: nextExpanded };
		expanded = next;
		persistExpanded(next);
		if (nextExpanded && !filter.trim()) void loadDirectory(key, loadSeq);
	}

	function toggleDir(key: string) {
		const next = { ...expanded, [key]: !expanded[key] };
		expanded = next;
		persistExpanded(next);
		if (next[key] && !filter.trim()) void loadDirectory(key, loadSeq);
	}

	function dirKey(parentKey: string, name: string): string {
		return parentKey ? `${parentKey}/${name}` : name;
	}

	function isExpanded(key: string): boolean {
		return expanded[key] ?? false;
	}

	function selectRelative(relativePath: string) {
		// Remember open folders + expand parents of the file we open.
		const next = { ...expanded, ...dirKeysToExpandForPaths([relativePath]) };
		expanded = next;
		persistExpanded(next);
		if (source === 'wiki') {
			onSelectFile(toWikiUiPath(relativePath));
			return;
		}
		onSelectFile(relativePath);
	}

	function scrollSearchIndexIntoView(index: number) {
		if (!searchScrollEl) return;
		const top = index * FILE_TREE_SEARCH_ROW_HEIGHT;
		const bottom = top + FILE_TREE_SEARCH_ROW_HEIGHT;
		const viewTop = searchScrollEl.scrollTop;
		const viewBottom = viewTop + searchScrollEl.clientHeight;
		if (top < viewTop) searchScrollEl.scrollTop = top;
		else if (bottom > viewBottom) searchScrollEl.scrollTop = bottom - searchScrollEl.clientHeight;
	}

	async function scrollSelectedIntoView() {
		const key = selectedKey;
		if (!key) return;
		if (searching) {
			const index = searchResults.indexOf(key);
			if (index >= 0) scrollSearchIndexIntoView(index);
			return;
		}
		if (!browserEl) return;
		await tick();
		const el = browserEl.querySelector(`[data-tree-key="${CSS.escape(key)}"]`);
		el?.scrollIntoView({ block: 'nearest' });
	}

	export function moveSelection(delta: number): boolean {
		if (visibleRows.length === 0) return false;
		const currentIndex = selectedKey
			? visibleRows.findIndex((row) => row.key === selectedKey)
			: -1;
		let nextIndex: number;
		if (currentIndex < 0) {
			nextIndex = delta > 0 ? 0 : visibleRows.length - 1;
		} else {
			nextIndex = Math.max(0, Math.min(visibleRows.length - 1, currentIndex + delta));
		}
		selectedKey = visibleRows[nextIndex]!.key;
		void scrollSelectedIntoView();
		return true;
	}

	export function activateSelection(): boolean {
		const row = visibleRows.find((r) => r.key === selectedKey);
		if (!row) return false;
		if (row.kind === 'file') {
			selectRelative(row.path);
			return true;
		}
		toggleDir(row.key);
		return true;
	}

	export function handleTreeKey(event: KeyboardEvent): boolean {
		switch (event.key) {
			case 'ArrowDown': {
				if (!moveSelection(1)) return false;
				event.preventDefault();
				return true;
			}
			case 'ArrowUp': {
				if (!moveSelection(-1)) return false;
				event.preventDefault();
				return true;
			}
			case 'Enter': {
				if (!activateSelection()) return false;
				event.preventDefault();
				return true;
			}
			case 'ArrowRight': {
				const row = visibleRows.find((r) => r.key === selectedKey);
				if (!row || row.kind !== 'dir' || isExpanded(row.key)) return false;
				setDirExpanded(row.key, true);
				event.preventDefault();
				return true;
			}
			case 'ArrowLeft': {
				const row = visibleRows.find((r) => r.key === selectedKey);
				if (!row) return false;
				if (row.kind === 'dir' && isExpanded(row.key)) {
					setDirExpanded(row.key, false);
					event.preventDefault();
					return true;
				}
				const slash = row.key.lastIndexOf('/');
				if (slash < 0) return false;
				const parentKey = row.key.slice(0, slash);
				if (!visibleRows.some((r) => r.key === parentKey)) return false;
				selectedKey = parentKey;
				void scrollSelectedIntoView();
				event.preventDefault();
				return true;
			}
			default:
				return false;
		}
	}

	async function loadDirectory(directory: string, seq: number) {
		if (loadedDirectories[directory] || loadingDirectories[directory]) return;
		loadingDirectories = { ...loadingDirectories, [directory]: true };
		try {
			const result =
				source === 'wiki'
					? await listWikiFileChildren(directory, LIST_LIMIT)
					: await listWorkspaceFileChildren(normalizedWorkspace, directory, LIST_LIMIT);
			if (seq !== loadSeq) return;
			files = [...new Set([...files, ...result.files])];
			loadedDirectories = { ...loadedDirectories, [directory]: true };
		} catch (err) {
			if (seq === loadSeq && directory === '') {
				error = err instanceof Error ? err.message : 'Failed to load files';
			}
		} finally {
			if (seq === loadSeq) {
				const { [directory]: _, ...remaining } = loadingDirectories;
				loadingDirectories = remaining;
			}
		}
	}

	async function loadSearch(seq: number) {
		const query = filter.trim();
		error = null;
		if (source === 'workspace' && !workspaceAvailable) {
			searchResults = [];
			loading = false;
			return;
		}
		try {
			if (source === 'wiki') {
				let wikiFiles = getCachedWikiFiles();
				if (wikiFiles.length === 0) {
					loading = true;
					wikiFiles = await refreshWikiFileIndex();
					if (seq !== loadSeq) return;
				}
				searchResults = rankMatchingFiles(wikiFiles, query, FILE_TREE_SEARCH_LIMIT);
				return;
			}

			let index = getFileIndex(normalizedWorkspace);
			if (!index?.loaded) {
				loading = true;
				index = await refreshFileIndex(normalizedWorkspace);
				if (seq !== loadSeq) return;
			}
			let matches = rankMatchingFiles(index.files, query, FILE_TREE_SEARCH_LIMIT);
			if (isFileIndexTruncated(normalizedWorkspace)) {
				const extra = await searchWorkspaceFiles(normalizedWorkspace, query);
				if (seq !== loadSeq) return;
				matches = rankMatchingFiles([...index.files, ...extra], query, FILE_TREE_SEARCH_LIMIT);
			}
			searchResults = matches;
		} catch (err) {
			if (seq !== loadSeq) return;
			searchResults = [];
			error = err instanceof Error ? err.message : 'Failed to search files';
		} finally {
			if (seq === loadSeq) loading = false;
		}
	}

	async function loadFiles() {
		const seq = ++loadSeq;
		loading = true;
		error = null;
		files = [];
		searchResults = [];
		loadedDirectories = {};
		loadingDirectories = {};

		if (source === 'workspace' && !workspaceAvailable) {
			expanded = {};
			loading = false;
			return;
		}

		try {
			expanded = { ...shellStore.getFileTreeExpanded(source) };
			await loadDirectory('', seq);
			if (seq !== loadSeq) return;

			const openDirectories = Object.keys(expanded)
				.filter((directory) => expanded[directory])
				.sort((a, b) => a.split('/').length - b.split('/').length);
			for (const directory of openDirectories) {
				await loadDirectory(directory, seq);
				if (seq !== loadSeq) return;
			}
		} catch (err) {
			if (seq !== loadSeq) return;
			files = [];
			loadedDirectories = {};
			expanded = { ...shellStore.getFileTreeExpanded(source) };
			error = err instanceof Error ? err.message : 'Failed to load files';
		} finally {
			if (seq === loadSeq) loading = false;
		}
	}

	$effect(() => {
		if (source === 'workspace' && workspaceAvailable) {
			void [normalizedWorkspace, workspaceChangeVersion(normalizedWorkspace)];
			untrack(() => void refreshFileIndex(normalizedWorkspace));
		} else if (source === 'wiki') {
			untrack(() => void refreshWikiFileIndex());
		}
	});

	$effect(() => {
		void [filter, normalizedWorkspace, workspaceChangeVersion(normalizedWorkspace), source];
		// Loading mutates the lazy cache; do not make those mutations dependencies of this effect.
		untrack(() => {
			if (filter.trim()) void loadSearch(++loadSeq);
			else void loadFiles();
		});
	});

	function onSearchScroll(event: Event) {
		const el = event.currentTarget as HTMLDivElement;
		searchScrollTop = el.scrollTop;
		searchViewportHeight = el.clientHeight;
	}

	$effect(() => {
		if (!searchScrollEl) return;
		searchViewportHeight = searchScrollEl.clientHeight || 320;
	});

	$effect(() => {
		const rows = visibleRows;
		void filter;
		if (rows.length === 0) {
			selectedKey = null;
			return;
		}
		if (!selectedKey || !rows.some((row) => row.key === selectedKey)) {
			selectedKey = rows[0]!.key;
		}
	});
</script>

{#snippet treeNodes(nodes: FileTreeNode[], parentKey: string)}
	<ul class="file-tree-list" role="tree">
		{#each nodes as node (dirKey(parentKey, node.name))}
			{@const key = dirKey(parentKey, node.name)}
						{@const hasChildren = node.children !== undefined}
			{@const rowExpanded = hasChildren && isExpanded(key)}
			<li
				class="file-tree-item"
				class:is-dir={hasChildren}
				class:is-expanded={rowExpanded}
				role="treeitem"
				aria-selected={selectedKey === key}
				aria-expanded={hasChildren ? rowExpanded : undefined}
			>
				{#if hasChildren}
					<button
						type="button"
						class="file-tree-row file-tree-dir"
						class:selected={selectedKey === key}
						data-tree-key={key}
						onclick={() => {
							selectedKey = key;
							toggleDir(key);
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
						{#if loadingDirectories[key]}
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
						class:selected={selectedKey === key}
						data-tree-key={key}
						onclick={() => {
							selectedKey = key;
							selectRelative(node.path!);
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
	bind:this={browserEl}
	tabindex="-1"
	role="region"
	aria-label={source === 'wiki' ? 'Wiki file tree' : 'Workspace file tree'}
	onkeydown={(event) => handleTreeKey(event)}
>
	{#if source === 'workspace' && !workspaceAvailable}
		<div class="file-tree-state">Select a workspace to browse its files.</div>
	{:else if searching && loading && searchResults.length === 0}
		<div class="file-tree-state">
			<Loader size={16} stroke-width={2} class="file-tree-spinner" />
			<span>Loading files…</span>
		</div>
	{:else if !searching && loading && files.length === 0}
		<div class="file-tree-state">
			<Loader size={16} stroke-width={2} class="file-tree-spinner" />
			<span>Loading files…</span>
		</div>
	{:else if error}
		<div class="file-tree-state file-tree-error">{error}</div>
	{:else if searching && searchResults.length === 0}
		<div class="file-tree-state">No matching files.</div>
	{:else if !searching && tree.length === 0}
		<div class="file-tree-state">No files found.</div>
	{:else if searching}
		<div
			class="file-tree-scroll scrollbar-none"
			bind:this={searchScrollEl}
			onscroll={onSearchScroll}
		>
			<div class="file-search-virtual" style:height="{searchWindow.height}px">
				<div class="file-search-virtual-inner" style:transform="translateY({searchWindow.offset}px)">
					{#each visibleSearchResults as path, offset (path)}
						{@const index = searchWindow.start + offset}
						<button
							type="button"
							class="file-tree-row file-tree-file file-search-row"
							class:selected={selectedKey === path}
							data-tree-key={path}
							data-result-index={index}
							onclick={() => {
								selectedKey = path;
								selectRelative(path);
							}}
							title={path}
						>
							<span class="file-tree-icon" aria-hidden="true">
								<FileTypeIcon {path} size={14} />
							</span>
							<span class="file-search-labels">
								<span class="file-search-name">{fileName(path)}</span>
								{#if fileDir(path)}
									<span class="file-search-dir">{fileDir(path)}</span>
								{/if}
							</span>
						</button>
					{/each}
				</div>
			</div>
		</div>
	{:else}
		<div class="file-tree-scroll scrollbar-none">
			{@render treeNodes(tree, '')}
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
			color-mix(in srgb, var(--workspace-inactive-color, #9a9a9f) 42%, transparent);
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
		color: var(--text-primary, #111);
		font-size: 13px;
		text-align: left;
		cursor: pointer;
	}

	.file-tree-row:hover {
		background: color-mix(
			in srgb,
			var(--workspace-inactive-color, #9a9a9f) 12%,
			transparent
		);
	}

	.file-tree-row.selected {
		background: color-mix(
			in srgb,
			var(--workspace-inactive-color, #9a9a9f) 18%,
			transparent
		);
	}

	.file-tree-row.file-tree-dir {
		color: var(--text-primary, #111);
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
		color: var(--workspace-inactive-color, #9a9a9f);
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
