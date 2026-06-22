import type { ImageAttachment, Session } from '$lib/types';

export interface PendingMessage {
	sessionId: string;
	text: string;
	displayText?: string;
	images?: ImageAttachment[];
	filePaths?: string[];
}

function createSessionStore() {
	let sessions = $state<Session[]>([]);
	let current = $state<Session | null>(null);
	let pendingMessages = $state.raw(new Map<string, Omit<PendingMessage, 'sessionId'>>());

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

	function discardSession(id: string) {
		removeSession(id);
		if (pendingMessages.has(id)) {
			const next = new Map(pendingMessages);
			next.delete(id);
			pendingMessages = next;
		}
	}

	function queuePendingMessage(
		sessionId: string,
		text: string,
		images?: ImageAttachment[],
		filePaths?: string[],
		displayText?: string
	) {
		pendingMessages = new Map(pendingMessages).set(sessionId, {
			text,
			images,
			filePaths,
			displayText
		});
	}

	function hasPendingMessage(sessionId: string) {
		return pendingMessages.has(sessionId);
	}

	function takePendingMessage(sessionId: string): Omit<PendingMessage, 'sessionId'> | null {
		const message = pendingMessages.get(sessionId);
		if (!message) return null;
		const next = new Map(pendingMessages);
		next.delete(sessionId);
		pendingMessages = next;
		return message;
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
		discardSession,
		queuePendingMessage,
		hasPendingMessage,
		takePendingMessage
	};
}

export const sessionStore = createSessionStore();
