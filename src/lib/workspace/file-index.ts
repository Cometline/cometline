import { listWorkspaceFiles } from '$lib/client/cometmind';

export interface FileIndexEntry {
	files: string[];
	loading: boolean;
	loaded: boolean;
	error: string | null;
	loadedAt: number;
}

const cache = new Map<string, FileIndexEntry>();
const inFlight = new Map<string, Promise<void>>();

export function getFileIndex(workspacePath: string): FileIndexEntry | null {
	return cache.get(workspacePath) ?? null;
}

export function isFileIndexReady(workspacePath: string): boolean {
	const entry = cache.get(workspacePath);
	return Boolean(entry?.loaded && !entry.loading);
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
			loadedAt: Date.now()
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
	if (entry) {
		entry.loading = true;
		entry.error = null;
	} else {
		cache.set(workspacePath, {
			files: [],
			loading: true,
			loaded: false,
			error: null,
			loadedAt: 0
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
		const files = await listWorkspaceFiles(workspacePath, '', 500);
		cache.set(workspacePath, {
			files,
			loading: false,
			loaded: true,
			error: null,
			loadedAt: Date.now()
		});
	} catch (err) {
		const message = err instanceof Error ? err.message : String(err);
		const current = cache.get(workspacePath);
		cache.set(workspacePath, {
			files: current?.files ?? [],
			loading: false,
			loaded: current?.loaded ?? false,
			error: message,
			loadedAt: current?.loadedAt ?? 0
		});
	}
}

export function filterFileIndex(files: string[], query: string): string[] {
	const q = query.trim().toLowerCase();
	if (!q) return files;
	return files.filter((path) => path.toLowerCase().includes(q));
}
