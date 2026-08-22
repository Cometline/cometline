import type { Session, StreamEvent } from '$lib/types';

export interface SessionRuntimeEventDeps {
	getActiveSessionId: () => string | null;
	setRunning: (sessionId: string, running: boolean) => void;
	refreshTranscript: (sessionId: string) => Promise<void>;
	resumeRun: (sessionId: string) => Promise<void>;
	refreshSession: (sessionId: string) => Promise<Session>;
	updateSession: (session: Session) => void;
}

export async function applySessionRuntimeEvent(
	event: StreamEvent,
	deps: SessionRuntimeEventDeps
): Promise<boolean> {
	if (event.type === 'run_started') {
		deps.setRunning(event.session_id, true);
		if (deps.getActiveSessionId() === event.session_id) {
			await deps.refreshTranscript(event.session_id);
			await deps.resumeRun(event.session_id);
		}
		return true;
	}
	if (event.type === 'run_finished') {
		deps.setRunning(event.session_id, false);
		if (deps.getActiveSessionId() === event.session_id) {
			await deps.refreshTranscript(event.session_id);
		}
		return true;
	}
	if (event.type !== 'session_cleared') return false;

	const [, latest] = await Promise.allSettled([
		deps.refreshTranscript(event.session_id),
		deps.refreshSession(event.session_id)
	]);
	if (latest.status === 'fulfilled') deps.updateSession(latest.value);
	return true;
}
