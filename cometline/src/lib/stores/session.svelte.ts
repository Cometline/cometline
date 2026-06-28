import { browser } from '$app/environment';
import type { ImageAttachment, Session } from '$lib/types';
import { publishWindowSync, subscribeWindowSync } from '$lib/window-sync';

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

	function upsertSession(
		session: Session,
		options: { selectCurrent?: boolean; prepend?: boolean; broadcast?: boolean } = {}
	) {
		const { selectCurrent = false, prepend = false, broadcast = true } = options;
		const existingIndex = sessions.findIndex((item) => item.id === session.id);
		if (existingIndex === -1) {
			sessions = prepend ? [session, ...sessions] : [...sessions, session];
		} else if (prepend) {
			sessions = [session, ...sessions.filter((item) => item.id !== session.id)];
		} else {
			sessions = sessions.map((item) => (item.id === session.id ? session : item));
		}
		if (selectCurrent || current?.id === session.id) current = session;
		if (broadcast) {
			publishWindowSync({ type: 'session-upsert', session });
		}
	}

	function selectSession(session: Session | null) {
		current = session;
	}

	function setSessions(list: Session[]) {
		sessions = list;
	}

	function appendSession(session: Session) {
		upsertSession(session, { selectCurrent: true, prepend: true });
	}

	function updateSession(session: Session) {
		upsertSession(session, { prepend: true });
	}

	function removeSession(id: string, options: { broadcast?: boolean } = {}) {
		const { broadcast = true } = options;
		sessions = sessions.filter((item) => item.id !== id);
		if (current?.id === id) current = null;
		if (broadcast) {
			publishWindowSync({ type: 'session-remove', sessionId: id });
		}
	}

	function discardSession(id: string, options: { broadcast?: boolean } = {}) {
		removeSession(id, options);
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

	if (browser) {
		subscribeWindowSync((message) => {
			if (message.type === 'session-upsert') {
				upsertSession(message.session, { prepend: true, broadcast: false });
				return;
			}
			if (message.type === 'session-remove') {
				removeSession(message.sessionId, { broadcast: false });
			}
		});
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
		upsertSession,
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
