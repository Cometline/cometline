import type { WebPanelTreeSource } from './web-panel-prefs';

export type PanelHistoryEntry =
	| { kind: 'browse'; source: WebPanelTreeSource }
	| { kind: 'file'; path: string }
	| { kind: 'git-diff'; path: string }
	| { kind: 'url'; url: string };

export type PanelHistoryState = {
	entries: PanelHistoryEntry[];
	index: number;
};

export function createPanelHistoryState(): PanelHistoryState {
	return { entries: [], index: -1 };
}

export function entriesEqual(a: PanelHistoryEntry, b: PanelHistoryEntry): boolean {
	if (a.kind !== b.kind) return false;
	if (a.kind === 'browse' && b.kind === 'browse') return a.source === b.source;
	if (a.kind === 'file' && b.kind === 'file') return a.path === b.path;
	if (a.kind === 'git-diff' && b.kind === 'git-diff') return a.path === b.path;
	if (a.kind === 'url' && b.kind === 'url') return a.url === b.url;
	return false;
}

export function currentEntry(state: PanelHistoryState): PanelHistoryEntry | null {
	if (state.index < 0 || state.index >= state.entries.length) return null;
	return state.entries[state.index] ?? null;
}

export function canGoBack(state: PanelHistoryState): boolean {
	return state.index > 0;
}

export function canGoForward(state: PanelHistoryState): boolean {
	return state.index >= 0 && state.index < state.entries.length - 1;
}

/**
 * Pushes an entry, truncating any forward stack. No-ops when equal to current.
 * When the stack is empty and the entry is not browse, seeds a browse underlay
 * so Back from the first file/URL returns to the file tree (using seedSource).
 */
export function pushEntry(
	state: PanelHistoryState,
	entry: PanelHistoryEntry,
	seedSource: WebPanelTreeSource = 'wiki'
): PanelHistoryState {
	const current = currentEntry(state);
	if (current && entriesEqual(current, entry)) {
		return state;
	}

	let base = state.entries.slice(0, state.index + 1);
	if (base.length === 0 && entry.kind !== 'browse') {
		base = [{ kind: 'browse', source: seedSource }];
	}

	return {
		entries: [...base, entry],
		index: base.length
	};
}

export function goBack(state: PanelHistoryState): PanelHistoryState {
	if (!canGoBack(state)) return state;
	return { ...state, index: state.index - 1 };
}

export function goForward(state: PanelHistoryState): PanelHistoryState {
	if (!canGoForward(state)) return state;
	return { ...state, index: state.index + 1 };
}
