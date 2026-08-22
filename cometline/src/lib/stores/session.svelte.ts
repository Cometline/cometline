import { browser } from '$app/environment';
import type { AgentMode, ImageAttachment, Session } from '$lib/types';
import type { WebContext } from '$lib/actions/start-chat';
import { publishWindowSync, subscribeWindowSync } from '$lib/window-sync';
import { unreadSessionOutputStore } from '$lib/stores/unread-session-output.svelte';
import { applySessionMetadata, type SessionMetadataPatch } from '$lib/sessions/session-metadata';

export interface PendingMessage {
	sessionId: string;
	text: string;
	displayText?: string;
	images?: ImageAttachment[];
	filePaths?: string[];
	webContexts?: WebContext[];
	agentMode?: AgentMode;
}

function createSessionStore() {
	let sessions = $state<Session[]>([]);
	let loaded = $state(false);
	let current = $state<Session | null>(null);
	let pendingMessages = $state.raw(new Map<string, Omit<PendingMessage, 'sessionId'>>());

	function writeSession(
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

	function upsertSession(
		session: Session,
		options: { selectCurrent?: boolean; prepend?: boolean; broadcast?: boolean } = {}
	) {
		writeSession(session, options);
	}

	function selectSession(session: Session | null) {
		current = session;
	}

	function setSessions(list: Session[]) {
		sessions = list;
		loaded = true;
		unreadSessionOutputStore.prune(list.map((session) => session.id));
		if (current && !list.some((session) => session.id === current?.id)) {
			current = null;
		} else if (current) {
			current = list.find((session) => session.id === current?.id) ?? current;
		}
	}

	function patchSessionMetadata(sessionId: string, patch: SessionMetadataPatch) {
		const existing = sessions.find((item) => item.id === sessionId);
		if (!existing) return;
		writeSession(applySessionMetadata(existing, patch), { prepend: true });
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
		unreadSessionOutputStore.remove(id, broadcast);
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
		displayText?: string,
		webContexts?: WebContext[],
		agentMode?: AgentMode
	) {
		pendingMessages = new Map(pendingMessages).set(sessionId, {
			text,
			images,
			filePaths,
			displayText,
			webContexts,
			agentMode
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
		get loaded() {
			return loaded;
		},
		get current() {
			return current;
		},
		selectSession,
		setSessions,
		upsertSession,
		appendSession,
		updateSession,
		patchSessionMetadata,
		removeSession,
		discardSession,
		queuePendingMessage,
		hasPendingMessage,
		takePendingMessage
	};
}

export const sessionStore = createSessionStore();
