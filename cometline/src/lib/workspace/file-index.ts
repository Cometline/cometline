import { listWorkspaceFiles } from '$lib/client/cometmind';

export interface FileIndexEntry {
	files: string[];
	loading: boolean;
	loaded: boolean;
	error: string | null;
	loadedAt: number;
	// True when the workspace has more matching files than the cached page, so
	// type-to-filter must fall back to a server-side query to find them all.
	truncated: boolean;
}

/** Page size for the warm cache used by the @-mention picker. */
const INDEX_LIMIT = 500;
/** Page size for an on-demand server-side search in a truncated workspace. */
const SEARCH_LIMIT = 50;

const cache = new Map<string, FileIndexEntry>();
const inFlight = new Map<string, Promise<void>>();

/** How long a loaded index is considered fresh before a background refresh. */
export const FILE_INDEX_TTL_MS = 30_000;

export function getFileIndex(workspacePath: string): FileIndexEntry | null {
	return cache.get(workspacePath) ?? null;
}

export function isFileIndexReady(workspacePath: string): boolean {
	const entry = cache.get(workspacePath);
	return Boolean(entry?.loaded && !entry.loading);
}

/** True when the index is loaded and within its TTL (no refresh needed). */
export function isFileIndexFresh(workspacePath: string, ttlMs = FILE_INDEX_TTL_MS): boolean {
	const entry = cache.get(workspacePath);
	if (!entry?.loaded || entry.loading) return false;
	return Date.now() - entry.loadedAt < ttlMs;
}

export function clearFileIndex(workspacePath: string): void {
	cache.delete(workspacePath);
	inFlight.delete(workspacePath);
}

export function clearAllFileIndexes(): void {
	cache.clear();
	inFlight.clear();
}

export async function refreshFileIndex(workspacePath: string): Promise<FileIndexEntry> {
	if (!workspacePath) {
		const entry: FileIndexEntry = {
			files: [],
			loading: false,
			loaded: true,
			error: null,
			loadedAt: Date.now(),
			truncated: false
		};
		cache.set(workspacePath, entry);
		return entry;
	}

	const existing = inFlight.get(workspacePath);
	if (existing) {
		await existing;
		return cache.get(workspacePath)!;
	}

	const entry = cache.get(workspacePath);
	if (entry?.loaded) {
		// Already have a usable list — refresh in the background without
		// flipping into a loading state, so the picker keeps showing the
		// current files while fresh ones load in.
		entry.error = null;
	} else if (entry) {
		entry.loading = true;
		entry.error = null;
	} else {
		cache.set(workspacePath, {
			files: [],
			loading: true,
			loaded: false,
			error: null,
			loadedAt: 0,
			truncated: false
		});
	}

	const promise = load(workspacePath);
	inFlight.set(workspacePath, promise);
	try {
		await promise;
	} finally {
		inFlight.delete(workspacePath);
	}
	return cache.get(workspacePath)!;
}

async function load(workspacePath: string): Promise<void> {
	try {
		const { files, truncated } = await listWorkspaceFiles(workspacePath, '', INDEX_LIMIT);
		cache.set(workspacePath, {
			files,
			loading: false,
			loaded: true,
			error: null,
			loadedAt: Date.now(),
			truncated
		});
	} catch (err) {
		const message = err instanceof Error ? err.message : String(err);
		const current = cache.get(workspacePath);
		cache.set(workspacePath, {
			files: current?.files ?? [],
			loading: false,
			loaded: current?.loaded ?? false,
			error: message,
			loadedAt: current?.loadedAt ?? 0,
			truncated: current?.truncated ?? false
		});
	}
}

/**
 * Whether the cached index for this workspace is incomplete, meaning
 * type-to-filter should query the backend to find files outside the cached page.
 */
export function isFileIndexTruncated(workspacePath: string): boolean {
	return Boolean(cache.get(workspacePath)?.truncated);
}

/**
 * Server-side filename search for a workspace, used when the cached index is
 * truncated so the user can still find files beyond the cached page.
 */
export async function searchWorkspaceFiles(
	workspacePath: string,
	query: string
): Promise<string[]> {
	if (!workspacePath || !query.trim()) return [];
	const { files } = await listWorkspaceFiles(workspacePath, query.trim(), SEARCH_LIMIT);
	return files;
}

export function filterFileIndex(files: string[], query: string): string[] {
	const q = query.trim().toLowerCase();
	if (!q) return files;
	return files.filter((path) => path.toLowerCase().includes(q));
}
