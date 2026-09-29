export function createMenuHighlight(deps: {
	getQuery: () => string;
	getOpen: () => boolean;
	getCount: () => number;
}) {
	let picked = $derived.by(() => {
		deps.getQuery();
		deps.getOpen();
		return 0;
	});

	const index = $derived.by(() => {
		const count = deps.getCount();
		if (!deps.getOpen() || count <= 0) return 0;
		return Math.min(Math.max(0, picked), count - 1);
	});

	function set(next: number) {
		const count = deps.getCount();
		if (count <= 0) {
			picked = 0;
			return;
		}
		picked = Math.max(0, Math.min(next, count - 1));
	}

	function move(delta: number) {
		const count = deps.getCount();
		if (count <= 0) return;
		set((index + delta + count) % count);
	}

	return {
		get index() {
			return index;
		},
		set index(next: number) {
			set(next);
		},
		set,
		move
	};
}
