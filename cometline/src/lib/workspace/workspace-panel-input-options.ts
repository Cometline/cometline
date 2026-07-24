import {
	filterFileIndex,
	getFileIndex,
	isFileIndexFresh,
	normalizeWorkspacePath,
	refreshFileIndex,
	searchWorkspaceFiles
} from '$lib/workspace/file-index';
import { rankFilePaths } from '$lib/workspace/file-search';

const DEFAULT_FILE_OPTION_LIMIT = 8;

export function rankWorkspaceFileMatches(files: string[], query: string): string[] {
	const trimmed = query.trim();
	if (!trimmed) return [];
	return rankFilePaths(files, trimmed);
}

export async function loadWorkspacePanelFileOptions(
	workspacePath: string,
	query: string,
	limit = DEFAULT_FILE_OPTION_LIMIT
): Promise<string[]> {
	workspacePath = normalizeWorkspacePath(workspacePath);
	const trimmed = query.trim();
	if (!trimmed || !workspacePath || workspacePath === '/') return [];

	let index = getFileIndex(workspacePath);
	if (!isFileIndexFresh(workspacePath)) {
		index = await refreshFileIndex(workspacePath);
	}

	const matches = index?.truncated
		? await searchWorkspaceFiles(workspacePath, trimmed)
		: filterFileIndex(index?.files ?? [], trimmed);

	return rankWorkspaceFileMatches(matches, trimmed).slice(0, limit);
}
