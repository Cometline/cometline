function createMiniShellStore() {
	let sidebarOpen = $state(false);
	let requestedNewSessionId = '';

	return {
		get sidebarOpen() {
			return sidebarOpen;
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
