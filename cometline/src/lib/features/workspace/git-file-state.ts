/** Minimal git status fields used for stage / unstage affordances. */
export type GitFileStageState = {
	xy?: string;
	untracked: boolean;
	staged: boolean;
};

/**
 * True when the working tree (or untracked) side still has changes to stage.
 * Uses porcelain `xy`: index char is [0], worktree char is [1].
 */
export function hasUnstagedSide(file: GitFileStageState): boolean {
	if (file.untracked) return true;
	const xy = file.xy ?? '';
	if (xy.length < 2) return !file.staged;
	return xy[1] !== ' ' && xy[1] !== '?';
}

/** Stage is available when there is still a worktree / untracked side. */
export function canStageGitFile(file: GitFileStageState | null | undefined): boolean {
	return Boolean(file && hasUnstagedSide(file));
}

/** Unstage is available when the index has an entry for this path. */
export function canUnstageGitFile(file: GitFileStageState | null | undefined): boolean {
	return Boolean(file?.staged);
}
