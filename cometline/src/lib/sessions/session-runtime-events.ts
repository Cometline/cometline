import type { Session, StreamEvent } from '$lib/types';

export interface SessionRuntimeEventDeps {
	setRunning: (sessionId: string, running: boolean) => void;
	refreshTranscript: (sessionId: string) => Promise<void>;
	refreshSession: (sessionId: string) => Promise<Session>;
	updateSession: (session: Session) => void;
}

export async function applySessionRuntimeEvent(
	event: StreamEvent,
	deps: SessionRuntimeEventDeps
): Promise<boolean> {
	if (event.type === 'run_started' || event.type === 'run_finished') {
		deps.setRunning(event.session_id, event.type === 'run_started');
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
