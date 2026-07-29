const POLL_INTERVAL_MS = 1000;
const BASE_URL = 'http://127.0.0.1:7700';
/** Keep showing "Starting CometMind…" through cold-start failures before escalating to error. */
const CONNECTING_GRACE_ATTEMPTS = 30;

type Status = 'connecting' | 'ready' | 'error';

function createConnectionState() {
	let status = $state<Status>('connecting');
	let message = $state('');
	let timer: ReturnType<typeof setInterval> | null = null;
	let failedAttempts = 0;

	function applyFailure(nextMessage: string) {
		failedAttempts += 1;
		message = nextMessage;

		// Once healthy, a failed poll is a real outage — surface it immediately.
		if (status === 'ready') {
			status = 'error';
			return;
		}

		// Already in the error UI: keep the latest message while background polls continue.
		if (status === 'error') {
			return;
		}

		// Startup / reconnect grace: stay on the comet loading state until we exhaust retries.
		if (failedAttempts >= CONNECTING_GRACE_ATTEMPTS) {
			status = 'error';
		}
	}

	async function check() {
		try {
			const res = await fetch(`${BASE_URL}/api/v1/health`, {
				method: 'GET',
				cache: 'no-store'
			});
			if (res.ok) {
				status = 'ready';
				message = '';
				failedAttempts = 0;
			} else {
				applyFailure(`Health check returned ${res.status}`);
			}
		} catch (err) {
			applyFailure(err instanceof Error ? err.message : 'Cannot reach CometMind');
		}
	}

	function reconnect() {
		status = 'connecting';
		message = '';
		failedAttempts = 0;
		void pollUntilReady();
	}

	async function pollUntilReady(maxAttempts = CONNECTING_GRACE_ATTEMPTS) {
		for (let attempt = 0; attempt < maxAttempts; attempt++) {
			await check();
			if (status === 'ready') return;
			// check() may escalate to error after the grace budget; stop early.
			if (status === 'error') return;
			await new Promise((resolve) => setTimeout(resolve, POLL_INTERVAL_MS));
		}
		// Defensive: if every attempt failed without escalating (shouldn't happen), surface error.
		if (status !== 'ready') {
			status = 'error';
			if (!message) message = 'Cannot reach CometMind';
		}
	}

	function startPolling() {
		void check();
		if (timer) return;
		timer = setInterval(() => {
			void check();
		}, POLL_INTERVAL_MS);
	}

	function stopPolling() {
		if (timer) {
			clearInterval(timer);
			timer = null;
		}
	}

	return {
		get status() {
			return status;
		},
		get message() {
			return message;
		},
		startPolling,
		stopPolling,
		check,
		reconnect
	};
}

export const connectionState = createConnectionState();
