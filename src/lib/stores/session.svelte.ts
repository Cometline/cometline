import type { Session } from '$lib/types';

function createSessionStore() {
	let sessions = $state<Session[]>([]);
	let current = $state<Session | null>(null);
	let pendingMessage = $state<{ sessionId: string; text: string } | null>(null);

	function selectSession(session: Session | null) {
		current = session;
	}

	function setSessions(list: Session[]) {
		sessions = list;
	}

	function appendSession(session: Session) {
		sessions = [session, ...sessions];
		current = session;
	}

	function updateSession(session: Session) {
		sessions = sessions.map((item) => (item.id === session.id ? session : item));
		if (current?.id === session.id) current = session;
	}

	function removeSession(id: string) {
		sessions = sessions.filter((item) => item.id !== id);
		if (current?.id === id) current = null;
	}

	function queuePendingMessage(sessionId: string, text: string) {
		pendingMessage = { sessionId, text };
	}

	function takePendingMessage(sessionId: string): string | null {
		if (pendingMessage?.sessionId !== sessionId) return null;
		const text = pendingMessage.text;
		pendingMessage = null;
		return text;
	}

	return {
		get sessions() {
			return sessions;
		},
		get current() {
			return current;
		},
		selectSession,
		setSessions,
		appendSession,
		updateSession,
		removeSession,
		queuePendingMessage,
		takePendingMessage
	};
}

export const sessionStore = createSessionStore();
