import { clearFileIndex, normalizeWorkspacePath } from './file-index';

export type WorkspaceChange = {
	workspacePath: string;
	paths: string[];
	gitChanged: boolean;
};

type WorkspaceChangeState = {
	version: number;
	unknownFileVersion: number;
	fileVersions: Record<string, number>;
};

const changes = $state<Record<string, WorkspaceChangeState>>({});

function normalizeFilePath(filePath: string): string {
	return filePath.replace(/\\/g, '/').replace(/^\.\//, '').replace(/^\/+/, '');
}

function stateFor(workspacePath: string): WorkspaceChangeState | undefined {
	return changes[normalizeWorkspacePath(workspacePath)];
}

function recordChange(workspacePath: string, paths: string[], unknownFiles: boolean) {
	const workspace = normalizeWorkspacePath(workspacePath);
	if (!workspace || workspace === '/') return;
	const current = changes[workspace] ?? { version: 0, unknownFileVersion: 0, fileVersions: {} };
	const fileVersions = { ...current.fileVersions };
	for (const rawPath of paths) {
		const filePath = normalizeFilePath(rawPath);
		if (filePath) fileVersions[filePath] = (fileVersions[filePath] ?? 0) + 1;
	}
	changes[workspace] = {
		version: current.version + 1,
		unknownFileVersion: current.unknownFileVersion + (unknownFiles ? 1 : 0),
		fileVersions
	};
	if (paths.length > 0 || unknownFiles) clearFileIndex(workspace);
}

/** Applies a coalesced Electron filesystem watcher notification. */
export function applyWorkspaceChange(change: WorkspaceChange): void {
	const paths = change.paths.map(normalizeFilePath).filter(Boolean);
	recordChange(change.workspacePath, paths, paths.length === 0 && !change.gitChanged);
}

/** Refreshes stale state after this window regains focus. */
export function refreshWorkspace(workspacePath: string): void {
	recordChange(workspacePath, [], true);
}

export function workspaceChangeVersion(workspacePath: string): number {
	return stateFor(workspacePath)?.version ?? 0;
}

export function workspaceFileChangeVersion(workspacePath: string, filePath: string): number {
	const state = stateFor(workspacePath);
	if (!state) return 0;
	return state.unknownFileVersion + (state.fileVersions[normalizeFilePath(filePath)] ?? 0);
}
