import {
	commitWorkspaceGit,
	discardWorkspaceGitPaths,
	getWorkspaceGitStatus,
	stageWorkspaceGitPaths,
	unstageWorkspaceGitPaths,
	type WorkspaceGitStatus
} from '$lib/client/cometmind';
import { shellStore } from '$lib/stores/shell.svelte';
import { normalizeWorkspacePath } from '$lib/features/workspace/file-index';
import { hasUnstagedSide } from '$lib/features/workspace/git-file-state';
import {
	discardConfirmDescription,
	gitFileName,
	type DiscardConfirm,
	type GitFile
} from '$lib/features/workspace/git-changes-browser';

export function createGitChangesBrowserController(deps: { getWorkspacePath: () => string }) {
	let loading = $state(false);
	let mutating = $state(false);
	let error = $state<string | null>(null);
	let actionError = $state<string | null>(null);
	let status = $state<WorkspaceGitStatus | null>(null);
	let filter = $state('');
	let loadSeq = 0;
	let copiedPath = $state<string | null>(null);
	let commitMessage = $state('');
	let commitFlash = $state('');
	let discardConfirm = $state<DiscardConfirm | null>(null);

	const normalizedWorkspace = $derived(normalizeWorkspacePath(deps.getWorkspacePath()));
	const workspaceAvailable = $derived(
		Boolean(normalizedWorkspace && normalizedWorkspace !== '/')
	);

	const allFiles = $derived(status?.files ?? []);

	const query = $derived(filter.trim().toLowerCase());

	function matchesFilter(file: GitFile): boolean {
		if (!query) return true;
		return file.path.toLowerCase().includes(query);
	}

	/** Staged section (VS Code "Staged Changes"). */
	const stagedFiles = $derived(allFiles.filter((f) => f.staged && matchesFilter(f)));

	/** Working tree section (VS Code "Changes") — unstaged + untracked. */
	const changeFiles = $derived(allFiles.filter((f) => hasUnstagedSide(f) && matchesFilter(f)));

	const stagedCount = $derived(allFiles.filter((f) => f.staged).length);
	const changesCount = $derived(allFiles.filter((f) => hasUnstagedSide(f)).length);

	const isClean = $derived(status?.is_repo === true && stagedCount === 0 && changesCount === 0);

	const discardDescription = $derived(
		discardConfirmDescription(discardConfirm, changeFiles.length)
	);

	async function load() {
		const seq = ++loadSeq;
		if (!workspaceAvailable) {
			status = null;
			error = null;
			loading = false;
			return;
		}
		loading = true;
		error = null;
		try {
			const result = await getWorkspaceGitStatus(normalizedWorkspace, 'all');
			if (seq !== loadSeq) return;
			status = result;
		} catch (err) {
			if (seq !== loadSeq) return;
			status = null;
			error = err instanceof Error ? err.message : 'Failed to load git status';
		} finally {
			if (seq === loadSeq) loading = false;
		}
	}

	function openDiff(path: string) {
		shellStore.openGitDiffForActive(path);
	}

	async function copyPath(path: string) {
		try {
			await navigator.clipboard.writeText(path);
			copiedPath = path;
			setTimeout(() => {
				if (copiedPath === path) copiedPath = null;
			}, 1200);
		} catch {
			// ignore
		}
	}

	function addPathToChat(path: string) {
		shellStore.addWebContextForActive({
			kind: 'file',
			title: gitFileName(path),
			source: `workspace-file:${path}`,
			content: ''
		});
		shellStore.requestComposerFocus();
	}

	async function runMutation(action: () => Promise<unknown>) {
		if (mutating) return;
		mutating = true;
		actionError = null;
		commitFlash = '';
		try {
			await action();
			await load();
		} catch (err) {
			actionError = err instanceof Error ? err.message : 'Git action failed';
		} finally {
			mutating = false;
		}
	}

	function stagePath(path: string) {
		return runMutation(() => stageWorkspaceGitPaths(normalizedWorkspace, [path]));
	}

	function unstagePath(path: string) {
		return runMutation(() => unstageWorkspaceGitPaths(normalizedWorkspace, [path]));
	}

	function requestDiscard(path: string) {
		if (mutating) return;
		discardConfirm = { kind: 'one', path };
	}

	function requestDiscardAll() {
		if (mutating || changeFiles.length === 0) return;
		discardConfirm = { kind: 'all' };
	}

	function cancelDiscard() {
		discardConfirm = null;
	}

	function confirmDiscard() {
		const pending = discardConfirm;
		discardConfirm = null;
		if (!pending) return;
		if (pending.kind === 'one') {
			return runMutation(() => discardWorkspaceGitPaths(normalizedWorkspace, [pending.path]));
		}
		const paths = changeFiles.map((f) => f.path);
		if (!paths.length) return;
		return runMutation(() => discardWorkspaceGitPaths(normalizedWorkspace, paths));
	}

	function stageAllChanges() {
		const paths = changeFiles.map((f) => f.path);
		if (!paths.length) return;
		return runMutation(() => stageWorkspaceGitPaths(normalizedWorkspace, paths));
	}

	function unstageAll() {
		const paths = stagedFiles.map((f) => f.path);
		if (!paths.length) return;
		return runMutation(() => unstageWorkspaceGitPaths(normalizedWorkspace, paths));
	}

	async function commit() {
		const message = commitMessage.trim();
		if (!message || mutating || stagedCount === 0) return;
		await runMutation(async () => {
			const result = await commitWorkspaceGit(normalizedWorkspace, message);
			commitMessage = '';
			commitFlash = result.sha ? `Committed ${result.sha}` : 'Committed';
		});
	}

	return {
		get loading() {
			return loading;
		},
		get mutating() {
			return mutating;
		},
		get error() {
			return error;
		},
		get actionError() {
			return actionError;
		},
		get status() {
			return status;
		},
		get filter() {
			return filter;
		},
		set filter(value: string) {
			filter = value;
		},
		get query() {
			return query;
		},
		get copiedPath() {
			return copiedPath;
		},
		get commitMessage() {
			return commitMessage;
		},
		set commitMessage(value: string) {
			commitMessage = value;
		},
		get commitFlash() {
			return commitFlash;
		},
		get discardConfirm() {
			return discardConfirm;
		},
		get discardDescription() {
			return discardDescription;
		},
		get normalizedWorkspace() {
			return normalizedWorkspace;
		},
		get workspaceAvailable() {
			return workspaceAvailable;
		},
		get stagedFiles() {
			return stagedFiles;
		},
		get changeFiles() {
			return changeFiles;
		},
		get stagedCount() {
			return stagedCount;
		},
		get changesCount() {
			return changesCount;
		},
		get isClean() {
			return isClean;
		},
		load,
		openDiff,
		copyPath,
		addPathToChat,
		stagePath,
		unstagePath,
		requestDiscard,
		requestDiscardAll,
		cancelDiscard,
		confirmDiscard,
		stageAllChanges,
		unstageAll,
		commit
	};
}
