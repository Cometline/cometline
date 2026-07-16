import { listWikiFiles } from '$lib/client/cometmind';
import { toWikiUiPath } from '$lib/wiki/paths';

const DEFAULT_LIMIT = 8;

export async function loadWikiFileOptions(query: string, limit = DEFAULT_LIMIT): Promise<string[]> {
	const trimmed = query.trim();
	if (!trimmed) return [];

	const { files } = await listWikiFiles(trimmed, limit);
	return files.map((path) => toWikiUiPath(path));
}
