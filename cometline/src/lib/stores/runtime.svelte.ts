const FAST_POLL_INTERVAL_MS = 1000;
const STEADY_POLL_INTERVAL_MS = 10_000;
const BASE_URL = 'http://127.0.0.1:7700';
/** Keep showing "Starting CometMind…" through cold-start failures before escalating to error. */
const CONNECTING_GRACE_ATTEMPTS = 30;

type Status = 'connecting' | 'ready' | 'error';

function createConnectionState() {
	let status = $state<Status>('connecting');
	let message = $state('');
	let timer: ReturnType<typeof setTimeout> | null = null;
	let failedAttempts = 0;
	let polling = false;
	let checkInFlight = false;

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
	}

	function pollInterval() {
		// Startup and outages stay on the fast cadence. A healthy sidecar does not
		// need a health request every second from every open window.
		return status === 'ready' ? STEADY_POLL_INTERVAL_MS : FAST_POLL_INTERVAL_MS;
	}

	function scheduleNext() {
		if (!polling || timer) return;
		timer = setTimeout(() => {
			timer = null;
			void runScheduledCheck();
		}, pollInterval());
	}

	async function runScheduledCheck() {
		if (!polling || checkInFlight) return;
		checkInFlight = true;
		try {
			await check();
		} finally {
			checkInFlight = false;
			scheduleNext();
		}
	}

	function startPolling() {
		if (polling) return;
		polling = true;
		void runScheduledCheck();
	}

	function stopPolling() {
		polling = false;
		if (timer) {
			clearTimeout(timer);
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
