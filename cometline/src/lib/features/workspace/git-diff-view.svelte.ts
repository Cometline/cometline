import {
	discardWorkspaceGitPaths,
	getWorkspaceGitDiff,
	getWorkspaceGitStatus,
	stageWorkspaceGitPaths,
	unstageWorkspaceGitPaths,
	type GitScope
} from '$lib/client/cometmind';
import { shellStore } from '$lib/stores/shell.svelte';
import {
	highlightGitDiffLines,
	type HighlightedDiffLine
} from '$lib/features/workspace/git-diff-highlight';
import { parseGitDiffLines } from '$lib/features/workspace/git-diff-lines';
import {
	canStageGitFile,
	canUnstageGitFile,
	type GitFileStageState
} from '$lib/features/workspace/git-file-state';
import { languageFromPath } from '$lib/features/workspace/file-preview';
import { buildFileSnippetContext } from '$lib/features/workspace/selection-snippet';
import {
	firstSelectionClientRect,
	selectionPopupPosition
} from '$lib/features/workspace/selection-popup';

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
	let copyFlash = $state(false);
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

	const fileName = $derived(
		deps.getFilePath().split(/[/\\]/).filter(Boolean).pop() || deps.getFilePath()
	);
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

	async function copyPath() {
		try {
			await navigator.clipboard.writeText(deps.getFilePath());
			copyFlash = true;
			setTimeout(() => {
				copyFlash = false;
			}, 1200);
		} catch {
			// ignore clipboard failures
		}
	}

	function addPathToChat() {
		shellStore.addWebContextForActive({
			kind: 'file',
			title: fileName,
			source: `workspace-file:${deps.getFilePath()}`,
			content: ''
		});
		shellStore.requestComposerFocus();
	}

	function addFullDiffToChat() {
		const content = diffText.trim().slice(0, 50000);
		if (!content) return;
		shellStore.addWebContextForActive({
			kind: 'file',
			title: `${fileName} (diff)`,
			source: `workspace-file:${deps.getFilePath()}`,
			content
		});
		shellStore.requestComposerFocus();
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
		const sel = window.getSelection();
		if (!sel || sel.isCollapsed || sel.rangeCount === 0) {
			clearSelectionPopup();
			return;
		}
		if (!root.contains(sel.anchorNode) || !root.contains(sel.focusNode)) {
			clearSelectionPopup();
			return;
		}
		const text = sel.toString().trim();
		if (!text) {
			clearSelectionPopup();
			return;
		}
		const rect = firstSelectionClientRect(sel.getRangeAt(0));
		selectionPopup = {
			...selectionPopupPosition(rect, window.innerWidth),
			text
		};
	}

	function addSelectionToChat() {
		if (!selectionPopup) return;
		const ctx = buildFileSnippetContext({
			filePath: deps.getFilePath(),
			selectedText: selectionPopup.text
		});
		if (ctx) {
			// Prefer a diff-flavored title when the selection looks like a hunk.
			const title = selectionPopup.text.includes('\n')
				? `${fileName} (diff selection)`
				: ctx.title;
			shellStore.addWebContextForActive({
				...ctx,
				title
			});
			shellStore.requestComposerFocus();
		}
		clearSelectionPopup();
		window.getSelection()?.removeAllRanges();
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
		get copyFlash() {
			return copyFlash;
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
		copyPath,
		addPathToChat,
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
