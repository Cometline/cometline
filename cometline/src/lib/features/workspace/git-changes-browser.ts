import type { WorkspaceGitStatus } from '#lib/client/cometmind.js';

export type GitFile = WorkspaceGitStatus['files'][number];
export type GitChangesSectionKind = 'staged' | 'changes';
export type DiscardConfirm = { kind: 'one'; path: string } | { kind: 'all' };

/** One row in the multi-file review, in list order. */
export type GitReviewFile = {
	path: string;
	section: GitChangesSectionKind;
};

/**
 * Files the review pane should open: current filter, staged block then changes.
 * A path that is both staged and unstaged appears twice, once per section.
 */
export function reviewFilesFor(stagedFiles: GitFile[], changeFiles: GitFile[]): GitReviewFile[] {
	return [
		...stagedFiles.map((file) => ({ path: file.path, section: 'staged' as const })),
		...changeFiles.map((file) => ({ path: file.path, section: 'changes' as const }))
	];
}

export function gitStatusBadge(file: GitFile): string {
	if (file.untracked) return 'U';
	switch (file.status) {
		case 'modified':
			return 'M';
		case 'added':
			return 'A';
		case 'deleted':
			return 'D';
		case 'renamed':
			return 'R';
		case 'conflict':
			return '!';
		default:
			return file.status.slice(0, 1).toUpperCase() || '?';
	}
}

export function gitFileName(path: string): string {
	return path.split(/[/\\]/).filter(Boolean).pop() || path;
}

export function gitFileDir(path: string): string {
	const parts = path.split(/[/\\]/).filter(Boolean);
	if (parts.length <= 1) return '';
	return parts.slice(0, -1).join('/');
}

export function discardConfirmDescription(
	confirm: DiscardConfirm | null,
	changeCount: number
): string {
	return confirm?.kind === 'one'
		? `Discard changes to “${confirm.path}”? Tracked files restore to HEAD. Untracked files are deleted. This cannot be undone.`
		: confirm?.kind === 'all'
			? `Discard all ${changeCount} unstaged change${changeCount === 1 ? '' : 's'}? Tracked files restore to HEAD. Untracked files are deleted. This cannot be undone.`
			: 'Tracked files restore to HEAD. Untracked files are deleted. This cannot be undone.';
}
