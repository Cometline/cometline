import { browser } from '$app/environment';
import { sessionStore } from '$lib/stores/session.svelte';
import { gotoHome } from '$lib/routes/session-route';
import { unreadSessionOutputStore } from '$lib/stores/unread-session-output.svelte';
import type { ChatState } from './chat-state.svelte';
import type { SessionCache } from './session-cache.svelte';

export function createSessionBinding(state: ChatState, cache: SessionCache) {
	function discardMissingSession(targetSessionID: string) {
		const handle = state.streamHandles.get(targetSessionID);
		if (handle) handle.abort.abort();
		cache.unmarkStreaming(targetSessionID);
		state.sessionCache.delete(targetSessionID);
		state.sessionErrors.delete(targetSessionID);
		cache.setRunError(targetSessionID, false);
		state.sessionContextBudgets.delete(targetSessionID);
		state.sessionTranscriptPages.delete(targetSessionID);
		sessionStore.discardSession(targetSessionID);
		if (state.sessionID === targetSessionID) {
			state.sessionID = null;
			state.items = [];
			state.error = '';
			state.contextBudget = null;
			state.isLoading = false;
			state.isLoadingOlder = false;
			state.hasMoreHistory = false;
		}
		if (browser) {
			void gotoHome();
		}
	}

	function clear() {
		cache.abortAllStreams();
		state.sessionCache.clear();
		state.sessionErrors.clear();
		state.failedRunSessionIds = new Set();
		state.sessionContextBudgets.clear();
		state.sessionTranscriptPages.clear();
		state.sessionID = null;
		state.items = [];
		state.isLoading = false;
		state.isLoadingOlder = false;
		state.hasMoreHistory = false;
		state.error = '';
		state.contextBudget = null;
		state.loadRun += 1;
		state.loadPromise = null;
		state.loadPromiseSession = null;
	}

	function resetTranscript(targetSessionID: string) {
		state.loadRun += 1;
		state.loadPromise = null;
		state.loadPromiseSession = null;
		state.sessionErrors.delete(targetSessionID);
		cache.setRunError(targetSessionID, false);
		state.sessionContextBudgets.delete(targetSessionID);
		state.sessionTranscriptPages.delete(targetSessionID);
		cache.writeSessionItems(targetSessionID, []);
		if (state.sessionID === targetSessionID) {
			state.error = '';
			state.isLoading = false;
			state.isLoadingOlder = false;
			state.hasMoreHistory = false;
			state.contextBudget = null;
		}
	}

	function detachActiveSession() {
		if (state.sessionID) {
			state.sessionCache.set(state.sessionID, cache.getCachedItems(state.sessionID));
		}
		state.loadRun += 1;
		state.loadPromise = null;
		state.loadPromiseSession = null;
		state.sessionID = null;
		state.items = [];
		state.isLoading = false;
		state.isLoadingOlder = false;
		state.hasMoreHistory = false;
		state.error = '';
		state.contextBudget = null;
	}

	function bindSession(nextSessionID: string) {
		if (state.sessionID === nextSessionID) {
			unreadSessionOutputStore.markRead(nextSessionID);
			return;
		}

		if (state.sessionID) {
			state.sessionCache.set(state.sessionID, cache.getCachedItems(state.sessionID));
		}

		state.loadRun += 1;
		state.loadPromise = null;
		state.loadPromiseSession = null;
		state.sessionID = nextSessionID;
		state.items = state.sessionCache.get(nextSessionID) ?? [];
		state.error = state.sessionErrors.get(nextSessionID) ?? '';
		state.contextBudget = state.sessionContextBudgets.get(nextSessionID) ?? null;
		state.hasMoreHistory = cache.getTranscriptPageState(nextSessionID).hasMore;
		state.isLoading = false;
		state.isLoadingOlder = false;
		unreadSessionOutputStore.markRead(nextSessionID);
	}

	return { discardMissingSession, clear, resetTranscript, detachActiveSession, bindSession };
}

export type SessionBinding = ReturnType<typeof createSessionBinding>;
