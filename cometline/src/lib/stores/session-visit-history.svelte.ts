const MAX_VISIT_HISTORY = 50;

function createSessionVisitHistoryStore() {
	let stack = $state<string[]>([]);
	let index = $state(-1);

	function recordVisit(sessionId: string) {
		const id = sessionId.trim();
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
		recordVisit,
		goBack,
		goForward,
		mostRecent,
		reset
	};
}

export const sessionVisitHistory = createSessionVisitHistoryStore();
