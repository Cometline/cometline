function createJobsIndicatorStore() {
	let ongoingCount = $state(0);
	let loaded = $state(false);

	function setOngoingCount(next: number) {
		ongoingCount = Math.max(0, next);
		loaded = true;
	}

	return {
		get ongoingCount() {
			return ongoingCount;
		},
		get loaded() {
			return loaded;
		},
		get hasOngoing() {
			return ongoingCount > 0;
		},
		setOngoingCount
	};
}

export const jobsIndicatorStore = createJobsIndicatorStore();
