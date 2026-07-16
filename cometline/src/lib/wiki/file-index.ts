import { listWikiFiles } from '$lib/client/cometmind';

export interface WikiFileIndexEntry {
	files: string[];
	loading: boolean;
	loaded: boolean;
	error: string | null;
	loadedAt: number;
	truncated: boolean;
}

const INDEX_LIMIT = 500;
const SEARCH_LIMIT = 50;

export const WIKI_FILE_INDEX_TTL_MS = 30_000;

let entry: WikiFileIndexEntry = {
	files: [],
	loading: false,
	loaded: false,
	error: null,
	loadedAt: 0,
	truncated: false
};
let inFlight: Promise<void> | null = null;

export function getWikiFileIndex(): WikiFileIndexEntry {
	return entry;
}

export function isWikiFileIndexReady(): boolean {
	return Boolean(entry.loaded && !entry.loading);
}

export function isWikiFileIndexFresh(ttlMs = WIKI_FILE_INDEX_TTL_MS): boolean {
	if (!entry.loaded || entry.loading) return false;
	return Date.now() - entry.loadedAt < ttlMs;
}

export async function refreshWikiFileIndex(): Promise<WikiFileIndexEntry> {
	if (inFlight) {
		await inFlight;
		return entry;
	}

	if (entry.loaded) {
		entry.error = null;
	} else {
		entry = { ...entry, loading: true, error: null };
	}

	inFlight = load();
	try {
		await inFlight;
	} finally {
		inFlight = null;
	}
	return entry;
}

async function load(): Promise<void> {
	try {
		const { files, truncated } = await listWikiFiles('', INDEX_LIMIT);
		entry = {
			files,
			loading: false,
			loaded: true,
			error: null,
			loadedAt: Date.now(),
			truncated
		};
	} catch (err) {
		const message = err instanceof Error ? err.message : String(err);
		entry = {
			files: entry.files,
			loading: false,
			loaded: entry.loaded,
			error: message,
			loadedAt: entry.loadedAt,
			truncated: entry.truncated
		};
	}
}

export function isWikiFileIndexTruncated(): boolean {
	return Boolean(entry.truncated);
}

export async function searchWikiFiles(query: string): Promise<string[]> {
	if (!query.trim()) return [];
	const { files } = await listWikiFiles(query.trim(), SEARCH_LIMIT);
	return files;
}

export function filterWikiFileIndex(files: string[], query: string): string[] {
	const q = query.trim().toLowerCase();
	if (!q) return files;
	return files.filter((path) => path.toLowerCase().includes(q));
}
