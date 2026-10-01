import {
	getSessionMessages,
	isSessionNotFoundError,
	listChildSessions
} from '$lib/client/cometmind';
import type { Session } from '$lib/types';
import { itemsFromTranscript, localID, mergeSubagents } from '$lib/stores/chat-transcript';
import type { ChatState } from './chat-state.svelte';
import type { SessionCache } from './session-cache.svelte';
import type { SessionBinding } from './session-binding.svelte';

export function createTranscriptLoader(
	state: ChatState,
	cache: SessionCache,
	binding: SessionBinding
) {
	async function loadTranscript(nextSessionID: string) {
		if (state.sessionID === nextSessionID && state.items.length > 0) return;
		if (cache.hasInFlightTurn(nextSessionID) && cache.cachedItemCount(nextSessionID) > 0)
			return;
		if (state.sessionID === nextSessionID && state.isLoading && state.loadPromise)
			return state.loadPromise;

		const run = ++state.loadRun;
		const switchingSession = state.sessionID !== nextSessionID;
		if (switchingSession) {
			if (state.sessionID) {
				state.sessionCache.set(state.sessionID, cache.getCachedItems(state.sessionID));
			}
			state.sessionID = nextSessionID;
			state.items = state.sessionCache.get(nextSessionID) ?? [];
			state.error = state.sessionErrors.get(nextSessionID) ?? '';
			state.contextBudget = state.sessionContextBudgets.get(nextSessionID) ?? null;
			state.hasMoreHistory = cache.getTranscriptPageState(nextSessionID).hasMore;
		} else {
			state.sessionID = nextSessionID;
		}
		state.isLoading = true;
		state.error = '';
		state.loadPromiseSession = nextSessionID;
		state.loadPromise = (async () => {
			try {
				const transcript = await getSessionMessages(nextSessionID);
				const children = await listChildSessions(nextSessionID).catch(() => ({
					sessions: [] as Session[]
				}));
				if (run !== state.loadRun && state.sessionID !== nextSessionID) return;
				if (
					cache.hasInFlightTurn(nextSessionID) &&
					cache.cachedItemCount(nextSessionID) > 0
				)
					return;
				if (state.sessionID === nextSessionID && state.items.length > 0) return;
				const loaded = mergeSubagents(
					itemsFromTranscript(transcript.items, { idPrefix: 'history' }),
					children.sessions
				);
				cache.writeSessionItems(nextSessionID, loaded);
				cache.applyTranscriptPageMeta(nextSessionID, transcript, { resetSeq: true });
				state.sessionErrors.delete(nextSessionID);
				if (state.sessionID === nextSessionID) state.error = '';
			} catch (err) {
				if (isSessionNotFoundError(err)) {
					binding.discardMissingSession(nextSessionID);
					return;
				}
				if (run !== state.loadRun && state.sessionID !== nextSessionID) return;
				if (
					cache.hasInFlightTurn(nextSessionID) &&
					cache.cachedItemCount(nextSessionID) > 0
				)
					return;
				if (state.sessionID === nextSessionID && state.items.length > 0) return;
				const message = err instanceof Error ? err.message : 'Failed to load transcript';
				state.sessionErrors.set(nextSessionID, message);
				cache.writeSessionItems(nextSessionID, [
					{ id: localID('error'), type: 'error', text: message }
				]);
				if (state.sessionID === nextSessionID) state.error = message;
			} finally {
				if (state.loadPromiseSession === nextSessionID) {
					if (state.sessionID === nextSessionID) {
						state.isLoading = false;
					}
					state.loadPromise = null;
					state.loadPromiseSession = null;
				}
			}
		})();
		return state.loadPromise;
	}

	async function refreshTranscript(nextSessionID: string) {
		if (!nextSessionID) return;
		if (cache.hasInFlightTurn(nextSessionID)) return;
		const run = ++state.loadRun;
		try {
			const transcript = await getSessionMessages(nextSessionID);
			const children = await listChildSessions(nextSessionID).catch(() => ({
				sessions: [] as Session[]
			}));
			if (run !== state.loadRun && state.sessionID !== nextSessionID) return;
			if (cache.hasInFlightTurn(nextSessionID)) return;
			const loaded = mergeSubagents(
				itemsFromTranscript(transcript.items, { idPrefix: 'history' }),
				children.sessions
			);
			cache.writeSessionItems(nextSessionID, loaded);
			cache.applyTranscriptPageMeta(nextSessionID, transcript, { resetSeq: true });
			state.sessionErrors.delete(nextSessionID);
			if (state.sessionID === nextSessionID) state.error = '';
		} catch (err) {
			if (isSessionNotFoundError(err)) {
				binding.discardMissingSession(nextSessionID);
				return;
			}
			if (run !== state.loadRun && state.sessionID !== nextSessionID) return;
			if (cache.hasInFlightTurn(nextSessionID)) return;
			const message = err instanceof Error ? err.message : 'Failed to refresh transcript';
			state.sessionErrors.set(nextSessionID, message);
			if (state.sessionID === nextSessionID) state.error = message;
		}
	}

	/** Prepend an older keyset page into the same chat store. Returns how many ChatItems were prepended. */
	async function loadOlderTranscript(
		targetSessionID: string = state.sessionID ?? ''
	): Promise<number> {
		if (!targetSessionID) return 0;
		const page = cache.getTranscriptPageState(targetSessionID);
		if (!page.hasMore || !page.nextBefore) return 0;
		if (state.isLoadingOlder && state.sessionID === targetSessionID) return 0;
		if (state.sessionID === targetSessionID) state.isLoadingOlder = true;
		const before = page.nextBefore;
		const seq = page.olderPageSeq + 1;
		try {
			const transcript = await getSessionMessages(targetSessionID, { before });
			if (state.sessionID !== targetSessionID && !state.sessionCache.has(targetSessionID))
				return 0;
			const older = itemsFromTranscript(transcript.items, { idPrefix: `older-${seq}` });
			if (older.length > 0) {
				const current = cache.getCachedItems(targetSessionID);
				cache.writeSessionItems(targetSessionID, [...older, ...current]);
			}
			cache.setTranscriptPageState(targetSessionID, {
				hasMore: Boolean(transcript.has_more),
				nextBefore: transcript.next_before ?? '',
				olderPageSeq: seq
			});
			return older.length;
		} catch (err) {
			if (isSessionNotFoundError(err)) {
				binding.discardMissingSession(targetSessionID);
				return 0;
			}
			const message = err instanceof Error ? err.message : 'Failed to load older messages';
			state.sessionErrors.set(targetSessionID, message);
			if (state.sessionID === targetSessionID) state.error = message;
			return 0;
		} finally {
			if (state.sessionID === targetSessionID) state.isLoadingOlder = false;
		}
	}

	return { loadTranscript, refreshTranscript, loadOlderTranscript };
}

export type TranscriptLoader = ReturnType<typeof createTranscriptLoader>;
