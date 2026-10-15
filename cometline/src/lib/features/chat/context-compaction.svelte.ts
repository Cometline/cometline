function createContextCompactionStore() {
	let ids = $state<string[]>([]);

	return {
		isCompacting(sessionId: string) {
			return sessionId !== '' && ids.includes(sessionId);
		},
		begin(sessionId: string) {
			if (!sessionId || ids.includes(sessionId)) return;
			ids = [...ids, sessionId];
		},
		end(sessionId: string) {
			ids = ids.filter((id) => id !== sessionId);
		}
	};
}

export const contextCompactionStore = createContextCompactionStore();
