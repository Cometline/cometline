import { navigateToSession } from '$lib/actions/navigate-to-session';
import { sessionVisitHistory } from '$lib/stores/session-visit-history.svelte';
import { sessionStore } from '$lib/stores/session.svelte';

function sessionExists(sessionId: string): boolean {
	return sessionStore.sessions.some((session) => session.id === sessionId);
}

/** Move through recently visited sessions (not sidebar order). */
export function navigateSessionHistory(direction: 'back' | 'forward') {
	const targetId =
		direction === 'back'
			? sessionVisitHistory.goBack(sessionExists)
			: sessionVisitHistory.goForward(sessionExists);
	if (!targetId) return;

	const session = sessionStore.sessions.find((item) => item.id === targetId);
	if (!session) return;

	navigateToSession(session, { fromHistory: true, commitSidebarOrder: false });
}

/** Jump to the most recently visited session from any route. */
export function navigateToRecentSession() {
	const targetId = sessionVisitHistory.mostRecent(sessionExists);
	if (!targetId) return;

	const session = sessionStore.sessions.find((item) => item.id === targetId);
	if (!session) return;

	navigateToSession(session);
}
