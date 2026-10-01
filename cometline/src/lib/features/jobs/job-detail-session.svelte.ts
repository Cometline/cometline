import { getSession, type JobResource } from '$lib/client/cometmind';
import { navigateToSession } from '$lib/actions/navigate-to-session';

export function createJobDetailSessionController(deps: {
	getJob: () => JobResource | null;
	onClose: () => void;
}) {
	let openingSession = $state(false);
	let openSessionError = $state('');

	async function handleOpenRunSession() {
		const job = deps.getJob();
		if (!job?.assigned_session_id) return;
		openingSession = true;
		openSessionError = '';
		try {
			const session = await getSession(job.assigned_session_id);
			navigateToSession(session);
			deps.onClose();
		} catch (err) {
			openSessionError = err instanceof Error ? err.message : 'Failed to open run session';
		} finally {
			openingSession = false;
		}
	}

	return {
		get openingSession() {
			return openingSession;
		},
		get openSessionError() {
			return openSessionError;
		},
		handleOpenRunSession
	};
}
