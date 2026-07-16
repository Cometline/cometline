import { loadWikiFileOptions } from '$lib/wiki/file-options';
import {
	loadWebPanelFileOptions,
	rankWorkspaceFileMatches
} from '$lib/workspace/web-panel-input-options';

const DEFAULT_LIMIT = 8;

export async function loadPanelFileOptions(
	workspacePath: string,
	query: string,
	limit = DEFAULT_LIMIT
): Promise<string[]> {
	const trimmed = query.trim();
	if (!trimmed) return [];

	const [workspaceFiles, wikiFiles] = await Promise.all([
		workspacePath && workspacePath !== '/'
			? loadWebPanelFileOptions(workspacePath, trimmed, limit)
			: Promise.resolve([] as string[]),
		loadWikiFileOptions(trimmed, limit)
	]);

	const merged = [...wikiFiles, ...workspaceFiles];
	const seen = new Set<string>();
	const unique: string[] = [];
	for (const path of merged) {
		if (seen.has(path)) continue;
		seen.add(path);
		unique.push(path);
	}
	return rankWorkspaceFileMatches(unique, trimmed).slice(0, limit);
}
