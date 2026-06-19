const POLL_INTERVAL_MS = 1000;
const BASE_URL = 'http://127.0.0.1:7700';

type Status = 'connecting' | 'ready' | 'error';

function createConnectionState() {
	let status = $state<Status>('connecting');
	let message = $state('');
	let timer: ReturnType<typeof setInterval> | null = null;

	async function check() {
		try {
			const res = await fetch(`${BASE_URL}/api/v1/health`, {
				method: 'GET',
				cache: 'no-store'
			});
			if (res.ok) {
				status = 'ready';
				message = '';
			} else {
				status = 'error';
				message = `Health check returned ${res.status}`;
			}
		} catch (err) {
			status = 'error';
			message = err instanceof Error ? err.message : 'Cannot reach CometMind';
		}
	}

	function reconnect() {
		status = 'connecting';
		message = '';
		void check();
	}

	function startPolling() {
		check();
		if (timer) return;
		timer = setInterval(check, POLL_INTERVAL_MS);
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
