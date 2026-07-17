import { tick } from 'svelte';
import { shellStore } from '$lib/stores/shell.svelte';
import {
	resolveMentionSourcePaths,
	shouldRunMentionServerSearch
} from '$lib/components/composer/composer-mention-search';
import {
	filterFileIndex,
	getFileIndex,
	isFileIndexFresh,
	isFileIndexReady,
	normalizeWorkspacePath,
	refreshFileIndex,
	searchWorkspaceFiles
} from '$lib/workspace/file-index';
import type { ComposerInputRef } from '$lib/components/composer/composer-input-ref';

type IdleHandle =
	| { type: 'idle'; id: number }
	| { type: 'timeout'; id: ReturnType<typeof setTimeout> };

function scheduleIdle(cb: () => void): IdleHandle {
	const ric = (
		window as unknown as {
			requestIdleCallback?: (cb: () => void, opts?: { timeout: number }) => number;
		}
	).requestIdleCallback;
	if (typeof ric === 'function') {
		return { type: 'idle', id: ric(cb, { timeout: 1500 }) };
	}
	return { type: 'timeout', id: setTimeout(cb, 400) };
}

function cancelIdle(handle: IdleHandle) {
	if (handle.type === 'idle') {
		const cic = (window as unknown as { cancelIdleCallback?: (id: number) => void })
			.cancelIdleCallback;
		cic?.(handle.id);
	} else {
		clearTimeout(handle.id);
	}
}

export function createComposerMentionsController(deps: {
	getInput: () => ComposerInputRef | null;
	getMentionMenuRef: () => HTMLDivElement | null;
}) {
	let mentionQuery = $state('');
	let mentionMenuOpen = $state(false);
	let mentionHighlight = $state(0);
	let mentionIndexVersion = $state(0);
	let mentionServerResults = $state<string[]>([]);
	let mentionServerQuery = $state('');
	let mentionServerLoading = $state(false);
	let mentionSearchPending = $state(false);
	let mentionSearchTimer: ReturnType<typeof setTimeout> | null = null;
	let mentionSearchSeq = 0;

	const activeWorkspacePath = $derived(normalizeWorkspacePath(shellStore.workspacePath));
	const hasWorkspace = $derived(Boolean(activeWorkspacePath) && activeWorkspacePath !== '/');
	const mentionsEnabled = $derived(hasWorkspace);

	const fileIndex = $derived.by(() => {
		void mentionIndexVersion;
		return hasWorkspace ? getFileIndex(activeWorkspacePath) : null;
	});

	const fileIndexReady = $derived.by(() => {
		void mentionIndexVersion;
		return hasWorkspace ? isFileIndexReady(activeWorkspacePath) : false;
	});

	const mentionTruncated = $derived(Boolean(fileIndex?.truncated));

	const localMatches = $derived.by(() =>
		filterFileIndex(fileIndex?.files ?? [], mentionQuery)
	);

	const queryTrimmed = $derived(mentionQuery.trim());

	const needsServerSearch = $derived(
		shouldRunMentionServerSearch(Boolean(fileIndex?.truncated), queryTrimmed)
	);

	const useServerSearch = $derived(needsServerSearch);

	const filteredMentionFiles = $derived.by(() =>
		resolveMentionSourcePaths(
			localMatches,
			mentionServerResults,
			mentionServerQuery,
			queryTrimmed,
			needsServerSearch
		)
	);

	$effect(() => {
		const workspacePath = activeWorkspacePath;
		if (!workspacePath || workspacePath === '/') return;
		if (isFileIndexReady(workspacePath)) return;
		const handle = scheduleIdle(() => {
			if (
				normalizeWorkspacePath(shellStore.workspacePath) === workspacePath &&
				!isFileIndexReady(workspacePath)
			) {
				void loadMentionIndex(workspacePath);
			}
		});
		return () => cancelIdle(handle);
	});

	$effect(() => {
		if (!mentionMenuOpen) return;
		if (mentionHighlight >= filteredMentionFiles.length) {
			mentionHighlight = Math.max(0, filteredMentionFiles.length - 1);
		}
	});

	$effect(() => {
		const workspacePath = activeWorkspacePath;
		const query = queryTrimmed;
		if (!mentionMenuOpen || !needsServerSearch) {
			if (mentionSearchTimer) clearTimeout(mentionSearchTimer);
			mentionSearchTimer = null;
			mentionServerLoading = false;
			mentionSearchPending = false;
			return;
		}
		if (mentionSearchTimer) clearTimeout(mentionSearchTimer);
		const seq = ++mentionSearchSeq;
		mentionSearchPending = true;
		mentionServerLoading = false;
		mentionSearchTimer = setTimeout(() => {
			mentionSearchPending = false;
			mentionServerLoading = true;
			void searchWorkspaceFiles(workspacePath, query)
				.then((files) => {
					if (seq !== mentionSearchSeq) return;
					mentionServerResults = files ?? [];
					mentionServerQuery = query;
				})
				.catch(() => {
					if (seq !== mentionSearchSeq) return;
					mentionServerResults = [];
					mentionServerQuery = query;
				})
				.finally(() => {
					if (seq === mentionSearchSeq) mentionServerLoading = false;
				});
		}, 150);
		return () => {
			if (mentionSearchTimer) clearTimeout(mentionSearchTimer);
			mentionSearchTimer = null;
			mentionSearchPending = false;
		};
	});

	function closeMentionMenu() {
		mentionMenuOpen = false;
		mentionQuery = '';
		mentionServerResults = [];
		mentionServerQuery = '';
		mentionServerLoading = false;
		mentionSearchPending = false;
		mentionSearchSeq += 1;
		if (mentionSearchTimer) clearTimeout(mentionSearchTimer);
		mentionSearchTimer = null;
	}

	async function scrollHighlightedMentionIntoView() {
		await tick();
		const option = deps
			.getMentionMenuRef()
			?.querySelector(`[data-mention-index="${mentionHighlight}"]`);
		if (option instanceof HTMLElement) {
			option.scrollIntoView({ block: 'nearest' });
		}
	}

	function selectMentionFile(path: string) {
		deps.getInput()?.insertFileMention(path);
		closeMentionMenu();
	}

	function onMentionQuery(payload: { query: string; active: boolean }) {
		if (!payload.active) {
			closeMentionMenu();
			return;
		}
		if (!hasWorkspace) return;
		if (!isFileIndexFresh(activeWorkspacePath)) {
			void loadMentionIndex(activeWorkspacePath);
		}
		mentionQuery = payload.query;
		mentionMenuOpen = true;
		mentionHighlight = 0;
	}

	async function loadMentionIndex(workspacePath: string) {
		try {
			await refreshFileIndex(workspacePath);
		} finally {
			mentionIndexVersion += 1;
		}
	}

	function handleMentionMenuKeydown(e: KeyboardEvent): boolean {
		if (!mentionMenuOpen) return false;
		if (e.key === 'Escape') {
			e.preventDefault();
			closeMentionMenu();
			return true;
		}
		if (e.key === 'ArrowDown') {
			e.preventDefault();
			if (filteredMentionFiles.length > 0) {
				mentionHighlight = (mentionHighlight + 1) % filteredMentionFiles.length;
				void scrollHighlightedMentionIntoView();
			}
			return true;
		}
		if (e.key === 'ArrowUp') {
			e.preventDefault();
			if (filteredMentionFiles.length > 0) {
				mentionHighlight =
					(mentionHighlight - 1 + filteredMentionFiles.length) %
					filteredMentionFiles.length;
				void scrollHighlightedMentionIntoView();
			}
			return true;
		}
		if (e.key === 'Tab' || e.key === 'Enter') {
			const path = filteredMentionFiles[mentionHighlight];
			if (!path) {
				if (e.key === 'Tab') {
					e.preventDefault();
					return true;
				}
				return false;
			}
			e.preventDefault();
			selectMentionFile(path);
			return true;
		}
		return false;
	}

	return {
		get hasWorkspace() {
			return hasWorkspace;
		},
		get mentionsEnabled() {
			return mentionsEnabled;
		},
		get mentionMenuOpen() {
			return mentionMenuOpen;
		},
		get mentionHighlight() {
			return mentionHighlight;
		},
		set mentionHighlight(index: number) {
			mentionHighlight = index;
		},
		get fileIndex() {
			return fileIndex;
		},
		get fileIndexReady() {
			return fileIndexReady;
		},
		get mentionTruncated() {
			return mentionTruncated;
		},
		get useServerSearch() {
			return useServerSearch;
		},
		get mentionServerLoading() {
			return mentionServerLoading;
		},
		get mentionSearchPending() {
			return mentionSearchPending;
		},
		get mentionQuery() {
			return mentionQuery;
		},
		get filteredMentionFiles() {
			return filteredMentionFiles;
		},
		handleMentionMenuKeydown,
		onMentionQuery,
		selectMentionFile
	};
}
