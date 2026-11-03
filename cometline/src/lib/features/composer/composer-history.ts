/** Pure helpers for Claude Code–style composer ↑/↓ history recall. */

export const COMPOSER_HISTORY_MAX_ENTRIES = 2000;

export type ComposerHistoryEntry = {
	/** What the composer showed, including `[Pasted ~N lines]` tokens. */
	display: string;
	/**
	 * Full composer text when it differs from `display`. Paste chips keep their
	 * body here so history recall can rebuild them. Omitted for plain prompts.
	 */
	text?: string;
	/** Paste chip bodies, in the order their `[Pasted …]` tokens appear in `display`. */
	pastes?: string[];
	timestamp: number;
	workspacePath: string;
	sessionId: string;
};

export type PendingUnsentDraft = {
	text: string;
	images?: import('#lib/types.js').ImageAttachment[];
};

export function normalizeWorkspacePath(path: string): string {
	return path.trim().replace(/\/+$/, '') || '/';
}

export function parseHistoryJsonl(raw: string): ComposerHistoryEntry[] {
	const entries: ComposerHistoryEntry[] = [];
	for (const line of raw.split('\n')) {
		const trimmed = line.trim();
		if (!trimmed) continue;
		try {
			const parsed: unknown = JSON.parse(trimmed);
			const entry = coerceHistoryEntry(parsed);
			if (entry) entries.push(entry);
		} catch {
			/* skip corrupt lines */
		}
	}
	return entries;
}

export function coerceHistoryEntry(value: unknown): ComposerHistoryEntry | null {
	if (!value || typeof value !== 'object' || Array.isArray(value)) return null;
	const record = value as Record<string, unknown>;
	const display = typeof record.display === 'string' ? record.display : '';
	if (!display.trim()) return null;
	const text = typeof record.text === 'string' && record.text.trim() ? record.text : undefined;
	const pastes = Array.isArray(record.pastes)
		? record.pastes.filter((item): item is string => typeof item === 'string' && item.length > 0)
		: undefined;
	const workspacePath =
		typeof record.workspacePath === 'string'
			? record.workspacePath
			: typeof record.project === 'string'
				? record.project
				: '';
	const sessionId = typeof record.sessionId === 'string' ? record.sessionId : '';
	const timestamp =
		typeof record.timestamp === 'number' && Number.isFinite(record.timestamp)
			? record.timestamp
			: Date.now();
	return {
		display,
		text,
		pastes: pastes?.length ? pastes : undefined,
		timestamp,
		workspacePath,
		sessionId
	};
}

export function serializeHistoryEntry(entry: ComposerHistoryEntry): string {
	return JSON.stringify({
		display: entry.display,
		...(entry.text && entry.text !== entry.display ? { text: entry.text } : {}),
		...(entry.pastes?.length ? { pastes: entry.pastes } : {}),
		timestamp: entry.timestamp,
		workspacePath: entry.workspacePath,
		sessionId: entry.sessionId
	});
}

/** Keep the newest `max` entries (array is oldest→newest). */
export function trimHistoryEntries(
	entries: ComposerHistoryEntry[],
	max = COMPOSER_HISTORY_MAX_ENTRIES
): ComposerHistoryEntry[] {
	if (entries.length <= max) return entries;
	return entries.slice(entries.length - max);
}

/**
 * Build newest-first recall list for ↑ navigation.
 * Order: pendingUnsent → workspace history file → transcript user texts (fill gaps).
 */
export type ComposerRecall = {
	/** Label form restored into the editor, including paste tokens. */
	display: string;
	/** Expanded composer text when a paste body was stored. Absent for plain prompts. */
	text?: string;
	pastes?: string[];
};

export function buildRecallList(options: {
	pendingText?: string | null;
	historyEntries: ComposerHistoryEntry[];
	workspacePath: string;
	transcriptUserTexts?: string[];
}): ComposerRecall[] {
	const workspace = normalizeWorkspacePath(options.workspacePath);
	const seen = new Set<string>();
	const out: ComposerRecall[] = [];

	const push = (display: string, text?: string, pastes?: string[]) => {
		const trimmed = display.trim();
		if (!trimmed || seen.has(trimmed)) return;
		seen.add(trimmed);
		const body = text?.trim() && text !== display ? text : undefined;
		const chips = pastes?.filter((item) => item.length > 0);
		out.push({
			display,
			...(body ? { text: body } : {}),
			...(chips?.length ? { pastes: chips } : {})
		});
	};

	const pending = options.pendingText?.trim();
	if (pending) push(options.pendingText!);

	// historyEntries stored oldest→newest; walk newest-first
	for (let i = options.historyEntries.length - 1; i >= 0; i--) {
		const entry = options.historyEntries[i];
		if (normalizeWorkspacePath(entry.workspacePath) !== workspace) continue;
		push(entry.display, entry.text, entry.pastes);
	}

	const transcript = options.transcriptUserTexts ?? [];
	for (const text of transcript) {
		push(text);
	}

	return out;
}

/** Newest-first user message texts from chat items. */
export function listUserMessageTexts(items: Array<{ type: string; text?: string }>): string[] {
	const out: string[] = [];
	for (let i = items.length - 1; i >= 0; i--) {
		const item = items[i];
		if (item.type !== 'user') continue;
		const text = typeof item.text === 'string' ? item.text.trim() : '';
		if (text) out.push(item.text!);
	}
	return out;
}

export type HistoryStepResult = {
	/** null means exit history mode and restore live draft */
	index: number | null;
};

/**
 * Step through a newest-first recall list.
 * `current === null` means not yet in history mode (live draft).
 */
export function stepHistoryIndex(
	current: number | null,
	direction: 'up' | 'down',
	length: number
): HistoryStepResult {
	if (length <= 0) return { index: null };

	if (direction === 'up') {
		if (current === null) return { index: 0 };
		return { index: Math.min(current + 1, length - 1) };
	}

	// down
	if (current === null) return { index: null };
	if (current <= 0) return { index: null };
	return { index: current - 1 };
}
