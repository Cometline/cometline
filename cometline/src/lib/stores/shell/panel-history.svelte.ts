import {
	canGoBack as historyCanGoBack,
	canGoForward as historyCanGoForward,
	createPanelHistoryState,
	currentEntry,
	entriesEqual,
	goBack as historyGoBack,
	goForward as historyGoForward,
	pushEntry,
	type PanelHistoryEntry,
	type PanelHistoryState
} from '#lib/features/workspace/panel-history.js';
import type { WorkspacePanelTreeSource } from '#lib/features/workspace/workspace-panel-prefs.js';
import type { SurfaceContentKey } from '#lib/features/workspace/workspace-panel-state.js';

function pushSurfaceHistory(
	state: PanelHistoryState,
	entry: PanelHistoryEntry,
	surface: SurfaceContentKey
): PanelHistoryState {
	if (surface === 'web-search') {
		const current = currentEntry(state);
		if (current && entriesEqual(current, entry)) return state;
		let base = state.entries.slice(0, state.index + 1);
		if (base.length === 0 && entry.kind === 'url' && entry.url) {
			base = [{ kind: 'url', url: '' }];
		}
		return { entries: [...base, entry], index: base.length };
	}
	const seed: WorkspacePanelTreeSource =
		surface === 'workspace' || surface === 'changes' ? surface : 'wiki';
	return pushEntry(state, entry, seed);
}

/** Per-session, per-surface Back/Forward stacks for the workspace panel. */
export function createPanelHistoryStore() {
	let panelHistoryBySessionSurface = $state<
		Record<string, Partial<Record<SurfaceContentKey, PanelHistoryState>>>
	>({});
	/** Suppresses history recording while applying back/forward navigation. */
	let applyingPanelHistory = false;

	function historyFor(sessionId: string, surface: SurfaceContentKey): PanelHistoryState {
		return panelHistoryBySessionSurface[sessionId]?.[surface] ?? createPanelHistoryState();
	}

	function setHistoryFor(
		sessionId: string,
		surface: SurfaceContentKey,
		state: PanelHistoryState
	) {
		panelHistoryBySessionSurface = {
			...panelHistoryBySessionSurface,
			[sessionId]: {
				...panelHistoryBySessionSurface[sessionId],
				[surface]: state
			}
		};
	}

	function step(
		sessionId: string,
		surface: SurfaceContentKey,
		canStep: (state: PanelHistoryState) => boolean,
		move: (state: PanelHistoryState) => PanelHistoryState
	): PanelHistoryEntry | null {
		const prev = historyFor(sessionId, surface);
		if (!canStep(prev)) return null;
		const next = move(prev);
		setHistoryFor(sessionId, surface, next);
		return currentEntry(next);
	}

	return {
		record(sessionId: string, surface: SurfaceContentKey, entry: PanelHistoryEntry) {
			if (applyingPanelHistory) return;
			const next = pushSurfaceHistory(historyFor(sessionId, surface), entry, surface);
			setHistoryFor(sessionId, surface, next);
		},
		canGoBack(sessionId: string, surface: SurfaceContentKey) {
			return historyCanGoBack(historyFor(sessionId, surface));
		},
		canGoForward(sessionId: string, surface: SurfaceContentKey) {
			return historyCanGoForward(historyFor(sessionId, surface));
		},
		/** Moves the stack back one entry; returns the entry to apply, or null when none. */
		stepBack(sessionId: string, surface: SurfaceContentKey) {
			return step(sessionId, surface, historyCanGoBack, historyGoBack);
		},
		/** Moves the stack forward one entry; returns the entry to apply, or null when none. */
		stepForward(sessionId: string, surface: SurfaceContentKey) {
			return step(sessionId, surface, historyCanGoForward, historyGoForward);
		},
		/** Runs `apply` with history recording suppressed. */
		whileApplying(apply: () => void) {
			applyingPanelHistory = true;
			try {
				apply();
			} finally {
				applyingPanelHistory = false;
			}
		},
		clearForSession(sessionId: string) {
			if (!(sessionId in panelHistoryBySessionSurface)) return;
			const next = { ...panelHistoryBySessionSurface };
			delete next[sessionId];
			panelHistoryBySessionSurface = next;
		}
	};
}

export type PanelHistoryStore = ReturnType<typeof createPanelHistoryStore>;
