import { listWikiFiles } from '$lib/client/cometmind';

const INDEX_LIMIT = 500;
const TTL_MS = 30_000;

type WikiFileIndex = {
	files: string[];
	loadedAt: number;
	loading: Promise<string[]> | null;
};

let index: WikiFileIndex = {
	files: [],
	loadedAt: 0,
	loading: null
};

export function getCachedWikiFiles(): string[] {
	return index.files;
}

export async function refreshWikiFileIndex(force = false): Promise<string[]> {
	const fresh = Date.now() - index.loadedAt < TTL_MS;
	if (!force && fresh && index.files.length > 0) return index.files;
	if (!force && index.loading) return index.loading;

	const promise = listWikiFiles('', INDEX_LIMIT)
		.then((result) => {
			index = {
				files: result.files ?? [],
				loadedAt: Date.now(),
				loading: null
			};
			return index.files;
		})
		.catch(() => {
			index = { ...index, loading: null };
			return index.files;
		});

	index = { ...index, loading: promise };
	return promise;
}
