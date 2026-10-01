import { tick, untrack } from 'svelte';
import { listWikiFileChildren, listWorkspaceFileChildren } from '$lib/client/cometmind';
import { shellStore, type FileTreeExpandSource } from '$lib/stores/shell.svelte';
import { toWikiUiPath } from '$lib/wiki/paths';
import { getCachedWikiFiles, refreshWikiFileIndex } from '$lib/wiki/wiki-file-index';
import { rankMatchingFiles } from '$lib/features/workspace/file-search';
import {
	getFileIndex,
	isFileIndexTruncated,
	normalizeWorkspacePath,
	refreshFileIndex,
	searchWorkspaceFiles
} from '$lib/features/workspace/file-index';
import {
	buildFileTree,
	dirKeysToExpandForPaths,
	flattenVisibleFileTreeRows
} from '$lib/features/workspace/file-tree';
import {
	FILE_TREE_SEARCH_LIMIT,
	FILE_TREE_SEARCH_ROW_HEIGHT,
	virtualWindow
} from '$lib/features/workspace/virtual-list';
import { workspaceChangeVersion } from '$lib/features/workspace/workspace-change.svelte';

export function createFileTreeBrowser(deps: {
	getWorkspacePath: () => string;
	getSource: () => FileTreeExpandSource;
	getFilter: () => string;
	onSelectFile: (path: string) => void;
}) {
	const s = $state({
		loading: false,
		error: null as string | null,
		files: [] as string[],
		expanded: {} as Record<string, boolean>,
		loadedDirectories: {} as Record<string, boolean>,
		loadingDirectories: {} as Record<string, boolean>,
		pickedKey: null as string | null,
		browserEl: null as HTMLDivElement | null,
		searchResults: [] as string[],
		searchScrollEl: null as HTMLDivElement | null,
		searchScrollTop: 0,
		searchViewportHeight: 320
	});
	const LIST_LIMIT = 10000;

	let loadSeq = 0;

	const normalizedWorkspace = $derived(normalizeWorkspacePath(deps.getWorkspacePath()));
	const workspaceAvailable = $derived(
		Boolean(normalizedWorkspace && normalizedWorkspace !== '/')
	);
	const searching = $derived(Boolean(deps.getFilter().trim()));
	const tree = $derived(buildFileTree(s.files));
	const visibleRows = $derived(
		searching
			? s.searchResults.map((path) => ({
					kind: 'file' as const,
					key: path,
					name: fileName(path),
					path
				}))
			: flattenVisibleFileTreeRows(tree, s.expanded)
	);
	const selectedKey = $derived.by(() => {
		const rows = visibleRows;
		if (rows.length === 0) return null;
		if (s.pickedKey && rows.some((row) => row.key === s.pickedKey)) return s.pickedKey;
		return rows[0]!.key;
	});

	function selectKey(key: string) {
		s.pickedKey = key;
	}
	const searchWindow = $derived(
		virtualWindow(
			s.searchResults.length,
			s.searchScrollTop,
			s.searchViewportHeight,
			FILE_TREE_SEARCH_ROW_HEIGHT
		)
	);
	const visibleSearchResults = $derived(
		s.searchResults.slice(searchWindow.start, searchWindow.end)
	);

	function fileName(path: string): string {
		return path.split(/[/\\]/).filter(Boolean).pop() || path;
	}

	function fileDir(path: string): string {
		const parts = path.split(/[/\\]/).filter(Boolean);
		if (parts.length <= 1) return '';
		return parts.slice(0, -1).join('/');
	}

	function persistExpanded(next: Record<string, boolean>) {
		shellStore.setFileTreeExpanded(deps.getSource(), next);
	}

	function setDirExpanded(key: string, nextExpanded: boolean) {
		if ((s.expanded[key] ?? false) === nextExpanded) return;
		const next = { ...s.expanded, [key]: nextExpanded };
		s.expanded = next;
		persistExpanded(next);
		if (nextExpanded && !deps.getFilter().trim()) void loadDirectory(key, loadSeq);
	}

	function toggleDir(key: string) {
		const next = { ...s.expanded, [key]: !s.expanded[key] };
		s.expanded = next;
		persistExpanded(next);
		if (next[key] && !deps.getFilter().trim()) void loadDirectory(key, loadSeq);
	}

	function dirKey(parentKey: string, name: string): string {
		return parentKey ? `${parentKey}/${name}` : name;
	}

	function isExpanded(key: string): boolean {
		return s.expanded[key] ?? false;
	}

	function keepPaneFocus(event: MouseEvent) {
		event.preventDefault();
	}

	function selectRelative(relativePath: string) {
		// Remember open folders + expand parents of the file we open.
		const next = { ...s.expanded, ...dirKeysToExpandForPaths([relativePath]) };
		s.expanded = next;
		persistExpanded(next);
		if (deps.getSource() === 'wiki') {
			deps.onSelectFile(toWikiUiPath(relativePath));
			return;
		}
		deps.onSelectFile(relativePath);
	}

	function scrollSearchIndexIntoView(index: number) {
		if (!s.searchScrollEl) return;
		const top = index * FILE_TREE_SEARCH_ROW_HEIGHT;
		const bottom = top + FILE_TREE_SEARCH_ROW_HEIGHT;
		const viewTop = s.searchScrollEl.scrollTop;
		const viewBottom = viewTop + s.searchScrollEl.clientHeight;
		if (top < viewTop) s.searchScrollEl.scrollTop = top;
		else if (bottom > viewBottom)
			s.searchScrollEl.scrollTop = bottom - s.searchScrollEl.clientHeight;
	}

	async function scrollSelectedIntoView() {
		const key = selectedKey;
		if (!key) return;
		if (searching) {
			const index = s.searchResults.indexOf(key);
			if (index >= 0) scrollSearchIndexIntoView(index);
			return;
		}
		if (!s.browserEl) return;
		await tick();
		const el = s.browserEl.querySelector(`[data-tree-key="${CSS.escape(key)}"]`);
		el?.scrollIntoView({ block: 'nearest' });
	}

	function moveSelection(delta: number): boolean {
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
		selectKey(visibleRows[nextIndex]!.key);
		void scrollSelectedIntoView();
		return true;
	}

	function activateSelection(): boolean {
		const row = visibleRows.find((r) => r.key === selectedKey);
		if (!row) return false;
		if (row.kind === 'file') {
			selectRelative(row.path);
			return true;
		}
		toggleDir(row.key);
		return true;
	}

	function handleTreeKey(event: KeyboardEvent): boolean {
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
				selectKey(parentKey);
				void scrollSelectedIntoView();
				event.preventDefault();
				return true;
			}
			default:
				return false;
		}
	}

	async function loadDirectory(directory: string, seq: number) {
		if (s.loadedDirectories[directory] || s.loadingDirectories[directory]) return;
		s.loadingDirectories = { ...s.loadingDirectories, [directory]: true };
		try {
			const result =
				deps.getSource() === 'wiki'
					? await listWikiFileChildren(directory, LIST_LIMIT)
					: await listWorkspaceFileChildren(normalizedWorkspace, directory, LIST_LIMIT);
			if (seq !== loadSeq) return;
			s.files = [...new Set([...s.files, ...result.files])];
			s.loadedDirectories = { ...s.loadedDirectories, [directory]: true };
		} catch (err) {
			if (seq === loadSeq && directory === '') {
				s.error = err instanceof Error ? err.message : 'Failed to load files';
			}
		} finally {
			if (seq === loadSeq) {
				const { [directory]: _, ...remaining } = s.loadingDirectories;
				s.loadingDirectories = remaining;
			}
		}
	}

	async function loadSearch(seq: number) {
		const query = deps.getFilter().trim();
		s.error = null;
		if (deps.getSource() === 'workspace' && !workspaceAvailable) {
			s.searchResults = [];
			s.loading = false;
			return;
		}
		try {
			if (deps.getSource() === 'wiki') {
				let wikiFiles = getCachedWikiFiles();
				if (wikiFiles.length === 0) {
					s.loading = true;
					wikiFiles = await refreshWikiFileIndex();
					if (seq !== loadSeq) return;
				}
				s.searchResults = rankMatchingFiles(wikiFiles, query, FILE_TREE_SEARCH_LIMIT);
				return;
			}

			let index = getFileIndex(normalizedWorkspace);
			if (!index?.loaded) {
				s.loading = true;
				index = await refreshFileIndex(normalizedWorkspace);
				if (seq !== loadSeq) return;
			}
			let matches = rankMatchingFiles(index.files, query, FILE_TREE_SEARCH_LIMIT);
			if (isFileIndexTruncated(normalizedWorkspace)) {
				const extra = await searchWorkspaceFiles(normalizedWorkspace, query);
				if (seq !== loadSeq) return;
				matches = rankMatchingFiles(
					[...index.files, ...extra],
					query,
					FILE_TREE_SEARCH_LIMIT
				);
			}
			s.searchResults = matches;
		} catch (err) {
			if (seq !== loadSeq) return;
			s.searchResults = [];
			s.error = err instanceof Error ? err.message : 'Failed to search files';
		} finally {
			if (seq === loadSeq) s.loading = false;
		}
	}

	async function loadFiles() {
		const seq = ++loadSeq;
		s.loading = true;
		s.error = null;
		s.files = [];
		s.searchResults = [];
		s.loadedDirectories = {};
		s.loadingDirectories = {};

		if (deps.getSource() === 'workspace' && !workspaceAvailable) {
			s.expanded = {};
			s.loading = false;
			return;
		}

		try {
			s.expanded = { ...shellStore.getFileTreeExpanded(deps.getSource()) };
			await loadDirectory('', seq);
			if (seq !== loadSeq) return;

			const openDirectories = Object.keys(s.expanded)
				.filter((directory) => s.expanded[directory])
				.sort((a, b) => a.split('/').length - b.split('/').length);
			for (const directory of openDirectories) {
				await loadDirectory(directory, seq);
				if (seq !== loadSeq) return;
			}
		} catch (err) {
			if (seq !== loadSeq) return;
			s.files = [];
			s.loadedDirectories = {};
			s.expanded = { ...shellStore.getFileTreeExpanded(deps.getSource()) };
			s.error = err instanceof Error ? err.message : 'Failed to load files';
		} finally {
			if (seq === loadSeq) s.loading = false;
		}
	}

	$effect(() => {
		if (deps.getSource() === 'workspace' && workspaceAvailable) {
			void [normalizedWorkspace, workspaceChangeVersion(normalizedWorkspace)];
			untrack(() => void refreshFileIndex(normalizedWorkspace));
		} else if (deps.getSource() === 'wiki') {
			untrack(() => void refreshWikiFileIndex());
		}
	});

	$effect(() => {
		void [
			deps.getFilter(),
			normalizedWorkspace,
			workspaceChangeVersion(normalizedWorkspace),
			deps.getSource()
		];
		// Loading mutates the lazy cache; do not make those mutations dependencies of this effect.
		untrack(() => {
			if (deps.getFilter().trim()) void loadSearch(++loadSeq);
			else void loadFiles();
		});
	});

	function onSearchScroll(event: Event) {
		const el = event.currentTarget as HTMLDivElement;
		s.searchScrollTop = el.scrollTop;
		s.searchViewportHeight = el.clientHeight;
	}

	$effect(() => {
		if (!s.searchScrollEl) return;
		s.searchViewportHeight = s.searchScrollEl.clientHeight || 320;
	});

	return {
		s,
		get normalizedWorkspace() {
			return normalizedWorkspace;
		},
		get workspaceAvailable() {
			return workspaceAvailable;
		},
		get searching() {
			return searching;
		},
		get tree() {
			return tree;
		},
		get visibleRows() {
			return visibleRows;
		},
		get selectedKey() {
			return selectedKey;
		},
		get searchWindow() {
			return searchWindow;
		},
		get visibleSearchResults() {
			return visibleSearchResults;
		},
		selectKey,
		fileName,
		fileDir,
		persistExpanded,
		setDirExpanded,
		toggleDir,
		dirKey,
		isExpanded,
		keepPaneFocus,
		selectRelative,
		scrollSearchIndexIntoView,
		scrollSelectedIntoView,
		moveSelection,
		activateSelection,
		handleTreeKey,
		loadDirectory,
		loadSearch,
		loadFiles,
		onSearchScroll
	};
}

export type FileTreeBrowserController = ReturnType<typeof createFileTreeBrowser>;
