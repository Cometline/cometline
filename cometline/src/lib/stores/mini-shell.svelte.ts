const MINI_OPENING_MIN_VISIBLE_MS = 320;

function createMiniShellStore() {
	let sidebarOpen = $state(false);
	let opening = $state(false);
	let openingRun = 0;
	let openingStartedAt: number | null = null;
	let requestedNewSessionId = '';

	function startOpening() {
		openingRun += 1;
		openingStartedAt = performance.now();
		opening = true;
		return openingRun;
	}

	return {
		get sidebarOpen() {
			return sidebarOpen;
		},
		get opening() {
			return opening;
		},
		get openingRun() {
			return openingRun;
		},
		prepareOpening() {
			opening = true;
			openingStartedAt = null;
		},
		startOpening,
		ensureOpening() {
			if (opening && openingStartedAt !== null) return openingRun;
			return startOpening();
		},
		async finishOpening(run: number) {
			if (!run || run !== openingRun || openingStartedAt === null) return;
			const remaining = Math.max(
				0,
				MINI_OPENING_MIN_VISIBLE_MS - (performance.now() - openingStartedAt)
			);
			if (remaining > 0) {
				await new Promise((resolve) => setTimeout(resolve, remaining));
			}
			if (run === openingRun) opening = false;
		},
		resetOpening() {
			openingRun += 1;
			openingStartedAt = null;
			opening = false;
		},
		toggleSidebar() {
			sidebarOpen = !sidebarOpen;
		},
		openSidebar() {
			sidebarOpen = true;
		},
		closeSidebar() {
			sidebarOpen = false;
		},
		requestNewSession(sessionId: string) {
			requestedNewSessionId = sessionId;
		},
		consumeNewSessionRequest(sessionId: string) {
			const requested = requestedNewSessionId === sessionId;
			if (requested) requestedNewSessionId = '';
			return requested;
		},
		clearNewSessionRequest(sessionId?: string) {
			if (!sessionId || requestedNewSessionId === sessionId) requestedNewSessionId = '';
		}
	};
}

export const miniShellStore = createMiniShellStore();
