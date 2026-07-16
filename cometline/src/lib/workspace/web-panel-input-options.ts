import {
	filterFileIndex,
	getFileIndex,
	isFileIndexFresh,
	normalizeWorkspacePath,
	refreshFileIndex,
	searchWorkspaceFiles
} from '$lib/workspace/file-index';

const DEFAULT_FILE_OPTION_LIMIT = 8;

function basename(path: string): string {
	const slash = Math.max(path.lastIndexOf('/'), path.lastIndexOf('\\'));
	return slash >= 0 ? path.slice(slash + 1) : path;
}

function stem(name: string): string {
	const dot = name.lastIndexOf('.');
	return dot > 0 ? name.slice(0, dot) : name;
}

function rankFile(path: string, query: string): number {
	const p = path.toLowerCase();
	const rawBasename = basename(path);
	const rawStem = stem(rawBasename);
	const b = rawBasename.toLowerCase();
	const s = rawStem.toLowerCase();
	const q = query.toLowerCase();
	if (path === query) return 0;
	if (p === q) return 1;
	if (rawBasename === query) return 2;
	if (b === q) return 3;
	if (rawStem === query) return 4;
	if (s === q) return 5;
	if (p.startsWith(q)) return 6;
	if (b.startsWith(q)) return 7;
	return 8;
}

export function rankWorkspaceFileMatches(files: string[], query: string): string[] {
	const trimmed = query.trim();
	if (!trimmed) return [];
	return [...files].sort((a, b) => {
		const rank = rankFile(a, trimmed) - rankFile(b, trimmed);
		if (rank !== 0) return rank;
		return a.localeCompare(b);
	});
}

export async function loadWebPanelFileOptions(
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
