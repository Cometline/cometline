import type { ImageAttachment, Session } from '$lib/types';

export interface PendingMessage {
	sessionId: string;
	text: string;
	images?: ImageAttachment[];
	filePaths?: string[];
}

function createSessionStore() {
	let sessions = $state<Session[]>([]);
	let current = $state<Session | null>(null);
	let pendingMessage = $state<PendingMessage | null>(null);

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

	function queuePendingMessage(
		sessionId: string,
		text: string,
		images?: ImageAttachment[],
		filePaths?: string[]
	) {
		pendingMessage = { sessionId, text, images, filePaths };
	}

	function hasPendingMessage(sessionId: string) {
		return pendingMessage?.sessionId === sessionId;
	}

	function takePendingMessage(sessionId: string): Omit<PendingMessage, 'sessionId'> | null {
		if (pendingMessage?.sessionId !== sessionId) return null;
		const message = {
			text: pendingMessage.text,
			images: pendingMessage.images,
			filePaths: pendingMessage.filePaths
		};
		pendingMessage = null;
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
		queuePendingMessage,
		hasPendingMessage,
		takePendingMessage
	};
}

export const sessionStore = createSessionStore();
