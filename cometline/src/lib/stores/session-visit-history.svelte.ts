const MAX_VISIT_HISTORY = 50;
export const LAST_VISITED_SESSION_STORAGE_KEY = 'cometline:last-active-session-id';

function browserStorage(): Storage | null {
	if (typeof window === 'undefined') return null;
	try {
		return window.localStorage;
	} catch {
		return null;
	}
}

function readPersistedSessionId(): string | null {
	try {
		const sessionId = browserStorage()?.getItem(LAST_VISITED_SESSION_STORAGE_KEY)?.trim();
		return sessionId || null;
	} catch {
		return null;
	}
}

function persistSessionId(sessionId: string) {
	try {
		browserStorage()?.setItem(LAST_VISITED_SESSION_STORAGE_KEY, sessionId);
	} catch {
		// Session navigation must still work when browser storage is unavailable.
	}
}

function clearPersistedSessionId() {
	try {
		browserStorage()?.removeItem(LAST_VISITED_SESSION_STORAGE_KEY);
	} catch {
		// A stale value is harmless when browser storage is unavailable.
	}
}

function createSessionVisitHistoryStore() {
	let stack = $state<string[]>([]);
	let index = $state(-1);

	function markActive(sessionId: string): string | null {
		const id = sessionId.trim();
		if (!id) return null;
		persistSessionId(id);
		return id;
	}

	function recordVisit(sessionId: string) {
		const id = markActive(sessionId);
		if (!id) return;
		if (stack[index] === id) return;

		const next = stack.slice(0, index + 1);
		next.push(id);
		while (next.length > MAX_VISIT_HISTORY) {
			next.shift();
		}
		stack = next;
		index = next.length - 1;
	}

	function goBack(exists: (sessionId: string) => boolean): string | null {
		while (index > 0) {
			index -= 1;
			const id = stack[index];
			if (exists(id)) return id;
			// Drop missing entry; index now sits on the next older candidate.
			stack = [...stack.slice(0, index), ...stack.slice(index + 1)];
		}
		return null;
	}

	function goForward(exists: (sessionId: string) => boolean): string | null {
		while (index >= 0 && index < stack.length - 1) {
			index += 1;
			const id = stack[index];
			if (exists(id)) return id;
			// Drop missing entry; keep index pointing at the slot that shifted in.
			stack = [...stack.slice(0, index), ...stack.slice(index + 1)];
			index -= 1;
		}
		return null;
	}

	/** Peek the current visit pointer (or older entries) without moving history. */
	function mostRecent(exists: (sessionId: string) => boolean): string | null {
		if (stack.length === 0) {
			const persisted = readPersistedSessionId();
			if (persisted && exists(persisted)) return persisted;
			if (persisted) clearPersistedSessionId();
		}
		for (let i = index; i >= 0; i -= 1) {
			const id = stack[i];
			if (exists(id)) return id;
		}
		return null;
	}

	function reset() {
		stack = [];
		index = -1;
	}

	return {
		get stack() {
			return stack;
		},
		get index() {
			return index;
		},
		get canGoBack() {
			return index > 0;
		},
		get canGoForward() {
			return index >= 0 && index < stack.length - 1;
		},
		markActive,
		recordVisit,
		goBack,
		goForward,
		mostRecent,
		reset
	};
}

export const sessionVisitHistory = createSessionVisitHistoryStore();
