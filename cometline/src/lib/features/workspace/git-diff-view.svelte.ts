import {
	discardWorkspaceGitPaths,
	getWorkspaceGitDiff,
	getWorkspaceGitStatus,
	stageWorkspaceGitPaths,
	unstageWorkspaceGitPaths,
	type GitScope
} from '#lib/client/cometmind.js';
import {
	highlightGitDiffLines,
	type HighlightedDiffLine
} from '#lib/features/workspace/git-diff-highlight.js';
import { parseGitDiffLines } from '#lib/features/workspace/git-diff-lines.js';
import {
	canStageGitFile,
	canUnstageGitFile,
	type GitFileStageState
} from '#lib/features/workspace/git-file-state.js';
import { languageFromPath } from '#lib/features/workspace/file-preview.js';
import {
	addDiffSelectionToChat,
	addGitDiffToChat,
	diffSelectionPopup
} from '#lib/features/workspace/diff-chat.js';

export function createGitDiffViewController(deps: {
	getWorkspacePath: () => string;
	getFilePath: () => string;
	getScope: () => GitScope;
	getOnBack: () => (() => void) | undefined;
	getOnMutated: () => (() => void) | undefined;
}) {
	let loading = $state(true);
	let error = $state<string | null>(null);
	let diffText = $state('');
	let binary = $state(false);
	let empty = $state(false);
	let truncated = $state(false);
	let message = $state('');
	let loadSeq = 0;
	let mutating = $state(false);
	let actionError = $state<string | null>(null);
	let discardConfirmOpen = $state(false);
	let fileState = $state<GitFileStageState | null>(null);
	let highlightedLines = $state<HighlightedDiffLine[]>([]);
	let selectionPopup = $state<{
		top: number;
		left: number;
		text: string;
	} | null>(null);

	const language = $derived(languageFromPath(deps.getFilePath()));
	const canStage = $derived(canStageGitFile(fileState));
	const canUnstage = $derived(canUnstageGitFile(fileState));
	const canDiscard = $derived(canStage);

	async function load() {
		const seq = ++loadSeq;
		loading = true;
		error = null;
		diffText = '';
		binary = false;
		empty = false;
		truncated = false;
		message = '';
		fileState = null;
		highlightedLines = [];
		selectionPopup = null;
		try {
			const [result, status] = await Promise.all([
				getWorkspaceGitDiff(deps.getWorkspacePath(), deps.getFilePath(), deps.getScope()),
				getWorkspaceGitStatus(deps.getWorkspacePath(), 'all').catch(() => null)
			]);
			if (seq !== loadSeq) return;
			binary = Boolean(result.binary);
			empty = Boolean(result.empty);
			truncated = Boolean(result.truncated);
			message = result.message ?? '';
			diffText = result.diff ?? '';
			const match = status?.files?.find((f) => f.path === deps.getFilePath());
			fileState = match
				? { staged: match.staged, untracked: match.untracked, xy: match.xy }
				: null;
			if (diffText.trim()) {
				const parsed = parseGitDiffLines(diffText);
				highlightedLines = await highlightGitDiffLines(
					parsed,
					languageFromPath(deps.getFilePath())
				);
			}
		} catch (err) {
			if (seq !== loadSeq) return;
			error = err instanceof Error ? err.message : 'Failed to load diff';
		} finally {
			if (seq === loadSeq) loading = false;
		}
	}

	function addFullDiffToChat() {
		addGitDiffToChat(deps.getFilePath(), diffText);
	}

	async function runMutation(action: () => Promise<unknown>) {
		if (mutating) return;
		mutating = true;
		actionError = null;
		try {
			await action();
			deps.getOnMutated()?.();
			await load();
		} catch (err) {
			actionError = err instanceof Error ? err.message : 'Git action failed';
		} finally {
			mutating = false;
		}
	}

	function stageFile() {
		if (!canStage) return;
		return runMutation(() =>
			stageWorkspaceGitPaths(deps.getWorkspacePath(), [deps.getFilePath()])
		);
	}

	function unstageFile() {
		if (!canUnstage) return;
		return runMutation(() =>
			unstageWorkspaceGitPaths(deps.getWorkspacePath(), [deps.getFilePath()])
		);
	}

	function requestDiscard() {
		if (!canDiscard || mutating) return;
		discardConfirmOpen = true;
	}

	function cancelDiscard() {
		discardConfirmOpen = false;
	}

	function confirmDiscard() {
		discardConfirmOpen = false;
		return runMutation(async () => {
			await discardWorkspaceGitPaths(deps.getWorkspacePath(), [deps.getFilePath()]);
			deps.getOnBack()?.();
		});
	}

	function clearSelectionPopup() {
		selectionPopup = null;
	}

	function onDiffMouseUp(event: MouseEvent) {
		const root = event.currentTarget;
		if (!(root instanceof HTMLElement)) return;
		selectionPopup = diffSelectionPopup(root);
	}

	function addSelectionToChat() {
		if (!selectionPopup) return;
		addDiffSelectionToChat(deps.getFilePath(), selectionPopup.text);
		clearSelectionPopup();
	}

	return {
		get loading() {
			return loading;
		},
		get error() {
			return error;
		},
		get diffText() {
			return diffText;
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
		get message() {
			return message;
		},
		get mutating() {
			return mutating;
		},
		get actionError() {
			return actionError;
		},
		get discardConfirmOpen() {
			return discardConfirmOpen;
		},
		get highlightedLines() {
			return highlightedLines;
		},
		get selectionPopup() {
			return selectionPopup;
		},
		get language() {
			return language;
		},
		get canStage() {
			return canStage;
		},
		get canUnstage() {
			return canUnstage;
		},
		get canDiscard() {
			return canDiscard;
		},
		load,
		addFullDiffToChat,
		stageFile,
		unstageFile,
		requestDiscard,
		cancelDiscard,
		confirmDiscard,
		clearSelectionPopup,
		onDiffMouseUp,
		addSelectionToChat
	};
}
