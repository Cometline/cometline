import {
	discardWorkspaceGitPaths,
	getWorkspaceGitDiff,
	stageWorkspaceGitPaths,
	unstageWorkspaceGitPaths,
	type GitScope,
	type WorkspaceGitDiff
} from '#lib/client/cometmind.js';
import {
	addDiffSelectionToChat,
	addGitDiffToChat,
	diffSelectionPopup,
	type DiffSelectionPopup
} from '#lib/features/workspace/diff-chat.js';
import {
	highlightGitDiffLines,
	type HighlightedDiffLine
} from '#lib/features/workspace/git-diff-highlight.js';
import { parseGitDiffLines } from '#lib/features/workspace/git-diff-lines.js';
import { languageFromPath } from '#lib/features/workspace/file-preview.js';
import type { GitReviewFile } from '#lib/features/workspace/git-changes-browser.js';
import { workspaceChangeVersion } from '#lib/features/workspace/workspace-change.svelte.js';

export function createGitReviewFileController(deps: {
	getWorkspacePath: () => string;
	getFile: () => GitReviewFile;
	getActive: () => boolean;
	getOnMutated: () => (() => unknown) | undefined;
}) {
	let open = $state(true);
	let loading = $state(false);
	let loaded = $state(false);
	let error = $state<string | null>(null);
	let message = $state('');
	let binary = $state(false);
	let empty = $state(false);
	let truncated = $state(false);
	let diffText = $state('');
	let lines = $state<HighlightedDiffLine[]>([]);
	let selectionPopup = $state<DiffSelectionPopup | null>(null);
	let mutating = $state(false);
	let discardOpen = $state(false);
	let actionError = $state<string | null>(null);
	let loadSeq = 0;

	const file = $derived(deps.getFile());
	const scope = $derived<GitScope>(file.section === 'staged' ? 'staged' : 'working');
	const language = $derived(languageFromPath(file.path));
	const hasDiff = $derived(Boolean(diffText.trim()));

	async function load() {
		if (!deps.getActive() || !open) return;
		const seq = ++loadSeq;
		const current = file;
		loading = true;
		error = null;
		try {
			const result: WorkspaceGitDiff = await getWorkspaceGitDiff(
				deps.getWorkspacePath(),
				current.path,
				scope
			);
			if (seq !== loadSeq) return;
			binary = Boolean(result.binary);
			empty = Boolean(result.empty);
			truncated = Boolean(result.truncated);
			message = result.message ?? '';
			diffText = result.diff ?? '';
			selectionPopup = null;
			lines = diffText.trim()
				? await highlightGitDiffLines(parseGitDiffLines(diffText), language)
				: [];
			loaded = true;
		} catch (err) {
			if (seq !== loadSeq) return;
			error = err instanceof Error ? err.message : 'Failed to load diff';
		} finally {
			if (seq === loadSeq) loading = false;
		}
	}

	$effect(() => {
		void [
			deps.getWorkspacePath(),
			file.path,
			scope,
			deps.getActive(),
			open,
			workspaceChangeVersion(deps.getWorkspacePath())
		];
		if (!deps.getActive() || !open) return;
		void load();
	});

	async function mutate(action: () => Promise<unknown>) {
		if (mutating) return;
		mutating = true;
		actionError = null;
		try {
			await action();
			await deps.getOnMutated()?.();
		} catch (err) {
			actionError = err instanceof Error ? err.message : 'Git action failed';
		} finally {
			mutating = false;
		}
	}

	function toggle() {
		open = !open;
	}

	function addDiffToChat() {
		addGitDiffToChat(file.path, diffText);
	}

	function onDiffMouseUp(event: MouseEvent) {
		const root = event.currentTarget;
		if (!(root instanceof HTMLElement)) return;
		selectionPopup = diffSelectionPopup(root);
	}

	function addSelection() {
		if (!selectionPopup) return;
		addDiffSelectionToChat(file.path, selectionPopup.text);
		selectionPopup = null;
	}

	function clearSelection() {
		selectionPopup = null;
	}

	function stage() {
		return mutate(() => stageWorkspaceGitPaths(deps.getWorkspacePath(), [file.path]));
	}

	function unstage() {
		return mutate(() => unstageWorkspaceGitPaths(deps.getWorkspacePath(), [file.path]));
	}

	function requestDiscard() {
		if (mutating) return;
		discardOpen = true;
	}

	function cancelDiscard() {
		discardOpen = false;
	}

	function confirmDiscard() {
		discardOpen = false;
		return mutate(() => discardWorkspaceGitPaths(deps.getWorkspacePath(), [file.path]));
	}

	return {
		get open() {
			return open;
		},
		get loading() {
			return loading;
		},
		get loaded() {
			return loaded;
		},
		get error() {
			return error;
		},
		get message() {
			return message;
		},
		get binary() {
			return binary;
		},
		get empty() {
			return empty;
		},
		get truncated() {
			return truncated;
		},
		get lines() {
			return lines;
		},
		get language() {
			return language;
		},
		get hasDiff() {
			return hasDiff;
		},
		get selectionPopup() {
			return selectionPopup;
		},
		get mutating() {
			return mutating;
		},
		get discardOpen() {
			return discardOpen;
		},
		get actionError() {
			return actionError;
		},
		toggle,
		addDiffToChat,
		onDiffMouseUp,
		addSelection,
		clearSelection,
		stage,
		unstage,
		requestDiscard,
		cancelDiscard,
		confirmDiscard
	};
}
