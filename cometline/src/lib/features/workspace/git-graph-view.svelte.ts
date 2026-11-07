import {
	checkoutWorkspaceGit,
	createWorkspaceGitBranch,
	createWorkspaceGitTag,
	deleteWorkspaceGitBranch,
	deleteWorkspaceGitTag,
	getWorkspaceGitGraph,
	getWorkspaceGitCommitDetail,
	getWorkspaceGitStatus,
	renameWorkspaceGitBranch,
	renameWorkspaceGitTag,
	type WorkspaceGitGraph
} from '#lib/client/cometmind.js';
import { normalizeWorkspacePath } from '#lib/features/workspace/file-index.js';
import {
	isUnsafeRefName,
	type GitGraphCommitDetail,
	type GitGraphMenuAction,
	type GitGraphRef
} from '#lib/features/workspace/git-graph.js';
import { apiErrorMessage } from '#lib/client/cometmind.js';

const pageSize = 80;

export type GitGraphDialog =
	| { kind: 'create-branch'; sha: string; name: string }
	| { kind: 'create-tag'; sha: string; name: string }
	| { kind: 'rename-branch'; name: string; next: string }
	| { kind: 'rename-tag'; name: string; next: string }
	| { kind: 'delete-branch'; name: string }
	| { kind: 'delete-tag'; name: string }
	| null;

export type GitGraphMenu =
	| { kind: 'commit'; sha: string; x: number; y: number }
	| { kind: 'ref'; ref: GitGraphRef; sha: string; x: number; y: number }
	| null;

export function createGitGraphViewController(deps: { getWorkspacePath: () => string }) {
	let loading = $state(false);
	let mutating = $state(false);
	let error = $state<string | null>(null);
	let actionError = $state<string | null>(null);
	let graph = $state<WorkspaceGitGraph | null>(null);
	let uncommittedCount = $state(0);
	let menu = $state<GitGraphMenu>(null);
	let dialog = $state<GitGraphDialog>(null);
	let selectedSha = $state<string | null>(null);
	let detail = $state<GitGraphCommitDetail | null>(null);
	let detailLoading = $state(false);
	let detailError = $state<string | null>(null);
	let loadSeq = 0;
	let detailSeq = 0;

	const normalizedWorkspace = $derived(normalizeWorkspacePath(deps.getWorkspacePath()));
	const workspaceAvailable = $derived(
		Boolean(normalizedWorkspace && normalizedWorkspace !== '/')
	);
	const commits = $derived(graph?.commits ?? []);
	const headLabel = $derived(
		graph?.head_branch || (graph?.head_sha ? graph.head_sha.slice(0, 7) : '')
	);

	async function load() {
		const seq = ++loadSeq;
		if (!workspaceAvailable) {
			graph = null;
			error = null;
			loading = false;
			return;
		}
		loading = true;
		error = null;
		try {
			const [result, status] = await Promise.all([
				getWorkspaceGitGraph(normalizedWorkspace, { limit: pageSize, offset: 0 }),
				getWorkspaceGitStatus(normalizedWorkspace, 'all').catch(() => null)
			]);
			if (seq !== loadSeq) return;
			graph = result;
			uncommittedCount = status?.files?.length ?? 0;
		} catch (err) {
			if (seq !== loadSeq) return;
			graph = null;
			error = apiErrorMessage(err, 'Failed to load git graph');
		} finally {
			if (seq === loadSeq) loading = false;
		}
	}

	async function loadMore() {
		if (!graph?.truncated || loading || mutating || !workspaceAvailable) return;
		loading = true;
		actionError = null;
		try {
			const page = await getWorkspaceGitGraph(normalizedWorkspace, {
				limit: pageSize,
				offset: graph.commits.length
			});
			graph = {
				...page,
				head_sha: graph.head_sha,
				head_branch: graph.head_branch,
				commits: [...graph.commits, ...page.commits]
			};
		} catch (err) {
			actionError = apiErrorMessage(err, 'Failed to load more commits');
		} finally {
			loading = false;
		}
	}

	async function selectCommit(sha: string) {
		if (selectedSha === sha) {
			selectedSha = null;
			detail = null;
			detailError = null;
			return;
		}
		selectedSha = sha;
		const seq = ++detailSeq;
		detailLoading = true;
		detailError = null;
		try {
			const next = await getWorkspaceGitCommitDetail(normalizedWorkspace, sha);
			if (seq !== detailSeq) return;
			detail = next;
		} catch (err) {
			if (seq !== detailSeq) return;
			detail = null;
			detailError = apiErrorMessage(err, 'Failed to load commit');
		} finally {
			if (seq === detailSeq) detailLoading = false;
		}
	}

	function closeDetail() {
		selectedSha = null;
		detail = null;
		detailError = null;
	}

	function openCommitMenu(sha: string, event: MouseEvent) {
		event.preventDefault();
		menu = { kind: 'commit', sha, x: event.clientX, y: event.clientY };
	}

	function openRefMenu(ref: GitGraphRef, sha: string, event: MouseEvent) {
		event.preventDefault();
		event.stopPropagation();
		if (ref.kind === 'remote') return;
		menu = { kind: 'ref', ref, sha, x: event.clientX, y: event.clientY };
	}

	function closeMenu() {
		menu = null;
	}

	function cancelDialog() {
		dialog = null;
	}

	function choose(action: GitGraphMenuAction) {
		const current = menu;
		menu = null;
		if (!current) return;
		if (current.kind === 'commit') {
			if (action === 'checkout')
				void run(() => checkoutWorkspaceGit(normalizedWorkspace, current.sha));
			if (action === 'create-branch')
				dialog = { kind: 'create-branch', sha: current.sha, name: '' };
			if (action === 'create-tag')
				dialog = { kind: 'create-tag', sha: current.sha, name: '' };
			return;
		}
		const ref = current.ref;
		if (action === 'checkout')
			void run(() => checkoutWorkspaceGit(normalizedWorkspace, ref.name));
		if (action === 'rename' && ref.kind === 'branch') {
			dialog = { kind: 'rename-branch', name: ref.name, next: ref.name };
		}
		if (action === 'rename' && ref.kind === 'tag') {
			dialog = { kind: 'rename-tag', name: ref.name, next: ref.name };
		}
		if (action === 'delete' && ref.kind === 'branch')
			dialog = { kind: 'delete-branch', name: ref.name };
		if (action === 'delete' && ref.kind === 'tag')
			dialog = { kind: 'delete-tag', name: ref.name };
	}

	function setDialogName(value: string) {
		if (!dialog) return;
		if (dialog.kind === 'create-branch' || dialog.kind === 'create-tag')
			dialog = { ...dialog, name: value };
		if (dialog.kind === 'rename-branch' || dialog.kind === 'rename-tag')
			dialog = { ...dialog, next: value };
	}

	async function confirmDialog() {
		const current = dialog;
		if (!current) return;
		if (
			(current.kind === 'create-branch' || current.kind === 'create-tag') &&
			isUnsafeRefName(current.name)
		) {
			actionError = 'Name cannot be empty or HEAD.';
			return;
		}
		if (
			(current.kind === 'rename-branch' || current.kind === 'rename-tag') &&
			isUnsafeRefName(current.next)
		) {
			actionError = 'Name cannot be empty or HEAD.';
			return;
		}
		dialog = null;
		if (current.kind === 'create-branch') {
			await run(() =>
				createWorkspaceGitBranch(normalizedWorkspace, current.name.trim(), current.sha)
			);
		} else if (current.kind === 'create-tag') {
			await run(() =>
				createWorkspaceGitTag(normalizedWorkspace, current.name.trim(), current.sha)
			);
		} else if (current.kind === 'rename-branch') {
			await run(() =>
				renameWorkspaceGitBranch(normalizedWorkspace, current.name, current.next.trim())
			);
		} else if (current.kind === 'rename-tag') {
			await run(() =>
				renameWorkspaceGitTag(normalizedWorkspace, current.name, current.next.trim())
			);
		} else if (current.kind === 'delete-branch') {
			await run(() => deleteWorkspaceGitBranch(normalizedWorkspace, current.name));
		} else if (current.kind === 'delete-tag') {
			await run(() => deleteWorkspaceGitTag(normalizedWorkspace, current.name));
		}
	}

	async function run(action: () => Promise<unknown>) {
		mutating = true;
		actionError = null;
		try {
			await action();
			await load();
		} catch (err) {
			actionError = apiErrorMessage(err, 'Git action failed');
		} finally {
			mutating = false;
		}
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
		get graph() {
			return graph;
		},
		get uncommittedCount() {
			return uncommittedCount;
		},
		get commits() {
			return commits;
		},
		get headLabel() {
			return headLabel;
		},
		get menu() {
			return menu;
		},
		get dialog() {
			return dialog;
		},
		get selectedSha() {
			return selectedSha;
		},
		get detail() {
			return detail;
		},
		get detailLoading() {
			return detailLoading;
		},
		get detailError() {
			return detailError;
		},
		get workspaceAvailable() {
			return workspaceAvailable;
		},
		load,
		loadMore,
		selectCommit,
		closeDetail,
		openCommitMenu,
		openRefMenu,
		closeMenu,
		cancelDialog,
		choose,
		setDialogName,
		confirmDialog
	};
}
