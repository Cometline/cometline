import { browser } from '$app/environment';
import { publishWindowSync, subscribeWindowSync } from '$lib/window-sync';

const STORAGE_KEY = 'cometline.unread-session-output.v1';

function storageAvailable() {
	return typeof localStorage !== 'undefined';
}

function readUnreadSessionIds(): Set<string> {
	if (!storageAvailable()) return new Set();
	try {
		const raw = localStorage.getItem(STORAGE_KEY);
		if (!raw) return new Set();
		const parsed: unknown = JSON.parse(raw);
		if (!Array.isArray(parsed)) return new Set();
		return new Set(parsed.filter((id): id is string => typeof id === 'string' && id.trim() !== ''));
	} catch {
		return new Set();
	}
}

function writeUnreadSessionIds(ids: Set<string>) {
	if (!storageAvailable()) return;
	if (ids.size === 0) {
		localStorage.removeItem(STORAGE_KEY);
		return;
	}
	localStorage.setItem(STORAGE_KEY, JSON.stringify([...ids]));
}

function createUnreadSessionOutputStore() {
	let unreadSessionIds = $state.raw(readUnreadSessionIds());

	function setUnread(sessionId: string, unread: boolean, broadcast = true) {
		const id = sessionId.trim();
		if (!id) return;
		const next = new Set(unreadSessionIds);
		if (unread) {
			next.add(id);
		} else {
			next.delete(id);
		}
		if (next.size !== unreadSessionIds.size || next.has(id) !== unreadSessionIds.has(id)) {
			unreadSessionIds = next;
			writeUnreadSessionIds(next);
		}
		if (broadcast) {
			publishWindowSync({ type: 'session-output-unread', sessionId: id, unread });
		}
	}

	function isUnread(sessionId: string) {
		return unreadSessionIds.has(sessionId);
	}

	function markUnread(sessionId: string) {
		setUnread(sessionId, true);
	}

	function markRead(sessionId: string) {
		setUnread(sessionId, false);
	}

	function remove(sessionId: string, broadcast = true) {
		setUnread(sessionId, false, broadcast);
	}

	function prune(sessionIds: Iterable<string>) {
		const valid = new Set(sessionIds);
		for (const id of unreadSessionIds) {
			if (!valid.has(id)) remove(id);
		}
	}

	if (browser) {
		subscribeWindowSync((message) => {
			if (message.type === 'session-output-unread') {
				setUnread(message.sessionId, message.unread, false);
			}
		});
	}

	return {
		get unreadSessionIds() {
			return unreadSessionIds;
		},
		isUnread,
		markUnread,
		markRead,
		remove,
		prune
	};
}

export const unreadSessionOutputStore = createUnreadSessionOutputStore();
