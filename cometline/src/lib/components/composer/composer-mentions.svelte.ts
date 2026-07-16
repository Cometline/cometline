import { tick } from 'svelte';
import { shellStore } from '$lib/stores/shell.svelte';
import { toWikiUiPath } from '$lib/wiki/paths';
import {
	filterWikiFileIndex,
	getWikiFileIndex,
	isWikiFileIndexFresh,
	isWikiFileIndexReady,
	isWikiFileIndexTruncated,
	refreshWikiFileIndex,
	searchWikiFiles
} from '$lib/wiki/file-index';
import {
	filterFileIndex,
	getFileIndex,
	isFileIndexFresh,
	isFileIndexReady,
	refreshFileIndex,
	searchWorkspaceFiles
} from '$lib/workspace/file-index';
import type { ComposerInputRef } from '$lib/components/composer/composer-input-ref';

export type MentionFileOption = {
	path: string;
	source: 'workspace' | 'wiki';
	label: string;
};

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

function workspaceOptions(paths: string[]): MentionFileOption[] {
	return paths.map((path) => ({ path, source: 'workspace', label: path }));
}

function wikiOptions(paths: string[]): MentionFileOption[] {
	return paths.map((path) => ({
		path: toWikiUiPath(path),
		source: 'wiki',
		label: path
	}));
}

function mergeMentionOptions(
	workspacePaths: string[],
	wikiPaths: string[],
	limit = 16
): MentionFileOption[] {
	const merged = [...wikiOptions(wikiPaths), ...workspaceOptions(workspacePaths)];
	return merged.slice(0, limit);
}

export function createComposerMentionsController(deps: {
	getInput: () => ComposerInputRef | null;
	getMentionMenuRef: () => HTMLDivElement | null;
}) {
	let mentionQuery = $state('');
	let mentionMenuOpen = $state(false);
	let mentionHighlight = $state(0);
	let mentionIndexVersion = $state(0);
	let mentionServerResults = $state<MentionFileOption[]>([]);
	let mentionServerQuery = $state('');
	let mentionServerLoading = $state(false);
	let mentionSearchTimer: ReturnType<typeof setTimeout> | null = null;
	let mentionSearchSeq = 0;

	const hasWorkspace = $derived(
		Boolean(shellStore.workspacePath) && shellStore.workspacePath !== '/'
	);
	const mentionsEnabled = $derived(true);

	const fileIndex = $derived.by(() => {
		void mentionIndexVersion;
		return hasWorkspace ? getFileIndex(shellStore.workspacePath) : null;
	});

	const wikiFileIndex = $derived.by(() => {
		void mentionIndexVersion;
		return getWikiFileIndex();
	});

	const fileIndexReady = $derived.by(() => {
		void mentionIndexVersion;
		if (!hasWorkspace) return isWikiFileIndexReady();
		return isFileIndexReady(shellStore.workspacePath) && isWikiFileIndexReady();
	});

	const mentionTruncated = $derived(
		Boolean(fileIndex?.truncated || wikiFileIndex.truncated)
	);

	const useServerSearch = $derived(mentionQuery.trim().length > 0);

	const filteredMentionFiles = $derived.by((): MentionFileOption[] => {
		if (useServerSearch) {
			if (mentionServerQuery === mentionQuery.trim()) return mentionServerResults;
			return [];
		}
		const wikiPaths = filterWikiFileIndex(wikiFileIndex.files, mentionQuery);
		const workspacePaths = hasWorkspace
			? filterFileIndex(fileIndex?.files ?? [], mentionQuery)
			: [];
		return mergeMentionOptions(workspacePaths, wikiPaths);
	});

	$effect(() => {
		if (!isWikiFileIndexReady()) {
			const handle = scheduleIdle(() => {
				if (!isWikiFileIndexReady()) void loadWikiMentionIndex();
			});
			return () => cancelIdle(handle);
		}
	});

	$effect(() => {
		const workspacePath = shellStore.workspacePath;
		if (!workspacePath || workspacePath === '/') return;
		if (isFileIndexReady(workspacePath)) return;
		const handle = scheduleIdle(() => {
			if (shellStore.workspacePath === workspacePath && !isFileIndexReady(workspacePath)) {
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
		const workspacePath = shellStore.workspacePath;
		const query = mentionQuery.trim();
		if (!mentionMenuOpen || !useServerSearch) {
			if (mentionSearchTimer) clearTimeout(mentionSearchTimer);
			mentionSearchTimer = null;
			return;
		}
		if (mentionSearchTimer) clearTimeout(mentionSearchTimer);
		const seq = ++mentionSearchSeq;
		mentionServerLoading = true;
		mentionSearchTimer = setTimeout(() => {
			const workspaceSearch =
				hasWorkspace &&
				(isFileIndexTruncated(workspacePath) || query.length > 0)
					? searchWorkspaceFiles(workspacePath, query)
					: Promise.resolve([] as string[]);
			const wikiSearch =
				isWikiFileIndexTruncated() || query.length > 0
					? searchWikiFiles(query)
					: Promise.resolve([] as string[]);

			void Promise.all([workspaceSearch, wikiSearch])
				.then(([workspacePaths, wikiPaths]) => {
					if (seq !== mentionSearchSeq) return;
					mentionServerResults = mergeMentionOptions(workspacePaths, wikiPaths);
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
		};
	});

	function closeMentionMenu() {
		mentionMenuOpen = false;
		mentionQuery = '';
		mentionServerResults = [];
		mentionServerQuery = '';
		mentionServerLoading = false;
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

	function selectMentionFile(option: MentionFileOption) {
		deps.getInput()?.insertFileMention(option.path);
		closeMentionMenu();
	}

	function onMentionQuery(payload: { query: string; active: boolean }) {
		if (!payload.active) {
			closeMentionMenu();
			return;
		}
		if (!mentionsEnabled) return;
		if (!isWikiFileIndexFresh()) {
			void loadWikiMentionIndex();
		}
		if (hasWorkspace && !isFileIndexFresh(shellStore.workspacePath)) {
			void loadMentionIndex(shellStore.workspacePath);
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

	async function loadWikiMentionIndex() {
		try {
			await refreshWikiFileIndex();
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
			const option = filteredMentionFiles[mentionHighlight];
			if (!option) {
				if (e.key === 'Tab') {
					e.preventDefault();
					return true;
				}
				return false;
			}
			e.preventDefault();
			selectMentionFile(option);
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
		get wikiFileIndex() {
			return wikiFileIndex;
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

function isFileIndexTruncated(workspacePath: string): boolean {
	return Boolean(getFileIndex(workspacePath)?.truncated);
}
