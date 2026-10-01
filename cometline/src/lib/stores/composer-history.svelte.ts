import {
	buildRecallList,
	coerceHistoryEntry,
	COMPOSER_HISTORY_MAX_ENTRIES,
	listUserMessageTexts,
	trimHistoryEntries,
	type ComposerHistoryEntry,
	type PendingUnsentDraft
} from '$lib/components/composer/composer-history';
import type { ImageAttachment } from '$lib/types';

const PENDING_STORAGE_KEY = 'cometline.composer-history-pending';
const LOCAL_HISTORY_KEY = 'cometline.composer-history';

type PendingMap = Record<string, { text: string; at: number }>;

function storageAvailable(): boolean {
	return typeof localStorage !== 'undefined';
}

function readPendingMap(): PendingMap {
	if (!storageAvailable()) return {};
	try {
		const raw = localStorage.getItem(PENDING_STORAGE_KEY);
		if (!raw) return {};
		const parsed: unknown = JSON.parse(raw);
		if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) return {};
		return parsed as PendingMap;
	} catch {
		return {};
	}
}

function writePendingMap(map: PendingMap) {
	if (!storageAvailable()) return;
	localStorage.setItem(PENDING_STORAGE_KEY, JSON.stringify(map));
}

function readLocalHistoryFallback(): ComposerHistoryEntry[] {
	if (!storageAvailable()) return [];
	try {
		const raw = localStorage.getItem(LOCAL_HISTORY_KEY);
		if (!raw) return [];
		const parsed: unknown = JSON.parse(raw);
		if (!Array.isArray(parsed)) return [];
		const entries: ComposerHistoryEntry[] = [];
		for (const item of parsed) {
			const entry = coerceHistoryEntry(item);
			if (entry) entries.push(entry);
		}
		return entries;
	} catch {
		return [];
	}
}

function writeLocalHistoryFallback(entries: ComposerHistoryEntry[]) {
	if (!storageAvailable()) return;
	localStorage.setItem(LOCAL_HISTORY_KEY, JSON.stringify(entries));
}

function createComposerHistoryStore() {
	let entries = $state.raw<ComposerHistoryEntry[]>([]);
	let loaded = $state(false);
	let loadPromise: Promise<void> | null = null;
	/** In-memory images for pending unsent drafts (not persisted — may be large). */
	const pendingImages = new Map<string, ImageAttachment[]>();

	async function ensureLoaded() {
		if (loaded) return;
		if (loadPromise) {
			await loadPromise;
			return;
		}
		loadPromise = (async () => {
			const api = typeof window !== 'undefined' ? window.electronAPI : undefined;
			if (api?.loadComposerHistory) {
				try {
					const loadedEntries = await api.loadComposerHistory();
					entries = Array.isArray(loadedEntries)
						? loadedEntries
								.map((item) => coerceHistoryEntry(item))
								.filter((item): item is ComposerHistoryEntry => item != null)
						: [];
				} catch {
					entries = readLocalHistoryFallback();
				}
			} else {
				entries = readLocalHistoryFallback();
			}
			loaded = true;
		})();
		try {
			await loadPromise;
		} finally {
			loadPromise = null;
		}
	}

	function sessionKey(sessionId: string): string {
		return sessionId.trim() || '__home__';
	}

	function stashUnsent(sessionId: string, draft: PendingUnsentDraft) {
		const text = draft.text.trim();
		if (!text && !(draft.images?.length ?? 0)) {
			clearPending(sessionId);
			return;
		}
		const key = sessionKey(sessionId);
		if (text) {
			const map = readPendingMap();
			map[key] = { text: draft.text, at: Date.now() };
			writePendingMap(map);
		} else {
			const map = readPendingMap();
			delete map[key];
			writePendingMap(map);
		}
		if (draft.images?.length) {
			pendingImages.set(key, draft.images);
		} else {
			pendingImages.delete(key);
		}
	}

	function getPending(sessionId: string): PendingUnsentDraft | null {
		const key = sessionKey(sessionId);
		const stored = readPendingMap()[key];
		const images = pendingImages.get(key);
		if (!stored?.text.trim() && !(images?.length ?? 0)) return null;
		return {
			text: stored?.text ?? '',
			images: images?.length ? images : undefined
		};
	}

	function clearPending(sessionId: string) {
		const key = sessionKey(sessionId);
		const map = readPendingMap();
		if (map[key]) {
			delete map[key];
			writePendingMap(map);
		}
		pendingImages.delete(key);
	}

	async function append(entry: Omit<ComposerHistoryEntry, 'timestamp'> & { timestamp?: number }) {
		const display = entry.display.trim();
		if (!display) return;
		const full: ComposerHistoryEntry = {
			display: entry.display,
			timestamp: entry.timestamp ?? Date.now(),
			workspacePath: entry.workspacePath,
			sessionId: entry.sessionId
		};
		await ensureLoaded();
		const api = typeof window !== 'undefined' ? window.electronAPI : undefined;
		if (api?.appendComposerHistory) {
			try {
				const result = await api.appendComposerHistory(full);
				if (result?.ok && Array.isArray(result.entries)) {
					entries = result.entries
						.map((item) => coerceHistoryEntry(item))
						.filter((item): item is ComposerHistoryEntry => item != null);
					return;
				}
			} catch {
				/* fall through to local */
			}
		}
		entries = trimHistoryEntries([...entries, full], COMPOSER_HISTORY_MAX_ENTRIES);
		writeLocalHistoryFallback(entries);
	}

	async function recallTexts(options: {
		sessionId: string;
		workspacePath: string;
		transcriptUserTexts?: string[];
	}): Promise<string[]> {
		await ensureLoaded();
		const pending = getPending(options.sessionId);
		return buildRecallList({
			pendingText: pending?.text ?? null,
			historyEntries: entries,
			workspacePath: options.workspacePath,
			transcriptUserTexts: options.transcriptUserTexts
		});
	}

	return {
		get entries() {
			return entries;
		},
		get loaded() {
			return loaded;
		},
		ensureLoaded,
		stashUnsent,
		getPending,
		clearPending,
		append,
		recallTexts,
		listUserMessageTexts
	};
}

export const composerHistoryStore = createComposerHistoryStore();
