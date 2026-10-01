import type { ChatItem } from '$lib/types';
import { anyReasoningPending, hasReasoning } from '$lib/features/chat/reasoning';
import { publishWindowSync } from '$lib/window-sync';
import type { SessionStream } from '$lib/stores/chat-stream-types';
import type { ChatState, TranscriptPageState } from './chat-state.svelte';

const CHAT_ITEMS_BROADCAST_MS = 64;

export function createSessionCache(state: ChatState) {
	function cachedItemCount(targetSessionID: string) {
		return state.sessionCache.get(targetSessionID)?.length ?? 0;
	}

	function getCachedItemCount(targetSessionID: string) {
		return cachedItemCount(targetSessionID);
	}

	/** User/assistant turns only — fork system notes map to `status` and must not
	 *  count as a real conversation for first-turn / composer dock. */
	function hasCachedConversationTurns(targetSessionID: string) {
		return getCachedItems(targetSessionID).some(
			(item) => item.type === 'user' || item.type === 'assistant'
		);
	}

	function getCachedItems(targetSessionID: string) {
		return state.sessionCache.get(targetSessionID) ?? [];
	}

	function refreshStreamingState() {
		state.streamingSessionIds = new Set([
			...state.localStreamingSessionIds,
			...state.remoteStreamingSessionIds
		]);
	}

	function setRunError(targetSessionID: string, failed: boolean, broadcast = true) {
		const next = new Set(state.failedRunSessionIds);
		if (failed) {
			next.add(targetSessionID);
		} else {
			next.delete(targetSessionID);
		}
		if (
			next.size === state.failedRunSessionIds.size &&
			next.has(targetSessionID) === state.failedRunSessionIds.has(targetSessionID)
		) {
			return;
		}
		state.failedRunSessionIds = next;
		if (broadcast) {
			publishWindowSync({ type: 'chat-run-error', sessionId: targetSessionID, failed });
		}
	}

	function flushChatItemsBroadcast(targetSessionID: string) {
		const timer = state.chatItemsBroadcastTimers.get(targetSessionID);
		if (timer) {
			clearTimeout(timer);
			state.chatItemsBroadcastTimers.delete(targetSessionID);
		}
		const nextItems = state.pendingChatItemsBroadcast.get(targetSessionID);
		if (!nextItems) return;
		state.pendingChatItemsBroadcast.delete(targetSessionID);
		publishWindowSync({ type: 'chat-items', sessionId: targetSessionID, items: nextItems });
	}

	function scheduleChatItemsBroadcast(targetSessionID: string, nextItems: ChatItem[]) {
		state.pendingChatItemsBroadcast.set(targetSessionID, nextItems);
		if (state.chatItemsBroadcastTimers.has(targetSessionID)) return;
		state.chatItemsBroadcastTimers.set(
			targetSessionID,
			setTimeout(() => {
				state.chatItemsBroadcastTimers.delete(targetSessionID);
				flushChatItemsBroadcast(targetSessionID);
			}, CHAT_ITEMS_BROADCAST_MS)
		);
	}

	function writeSessionItems(
		targetSessionID: string,
		nextItems: ChatItem[],
		options: { broadcast?: boolean } = {}
	) {
		const { broadcast = true } = options;
		state.sessionCache.set(targetSessionID, nextItems);
		if (state.sessionID === targetSessionID) {
			state.items = nextItems;
		}
		if (broadcast) scheduleChatItemsBroadcast(targetSessionID, nextItems);
	}

	function markStreaming(targetSessionID: string, handle: SessionStream) {
		state.streamHandles.set(targetSessionID, handle);
		state.localStreamingSessionIds.add(targetSessionID);
		refreshStreamingState();
		publishWindowSync({ type: 'chat-streaming', sessionId: targetSessionID, streaming: true });
	}

	function unmarkStreaming(targetSessionID: string) {
		state.streamHandles.delete(targetSessionID);
		if (state.localStreamingSessionIds.delete(targetSessionID)) {
			refreshStreamingState();
		}
		flushChatItemsBroadcast(targetSessionID);
		publishWindowSync({ type: 'chat-streaming', sessionId: targetSessionID, streaming: false });
	}

	function hasLocalStream(targetSessionID: string) {
		return state.streamHandles.has(targetSessionID);
	}

	function setRemoteStreamingState(targetSessionID: string, streaming: boolean) {
		if (streaming) {
			state.remoteStreamingSessionIds.add(targetSessionID);
		} else {
			state.remoteStreamingSessionIds.delete(targetSessionID);
		}
		refreshStreamingState();
	}

	function isStreamingFor(targetSessionID: string) {
		return state.streamingSessionIds.has(targetSessionID);
	}

	function hasRunError(targetSessionID: string) {
		return state.failedRunSessionIds.has(targetSessionID);
	}

	function hasInFlightTurn(targetSessionID: string) {
		if (isStreamingFor(targetSessionID)) return true;
		if (state.streamHandles.has(targetSessionID)) return true;
		return getCachedItems(targetSessionID).some(
			(item) =>
				item.type === 'assistant' && (item.pending === true || anyReasoningPending(item))
		);
	}

	function isAwaitingFirstAssistant(targetSessionID: string) {
		if (!hasInFlightTurn(targetSessionID) && !isStreamingFor(targetSessionID)) return false;
		const cached = getCachedItems(targetSessionID);
		const hasUser = cached.some((item) => item.type === 'user');
		const pendingAssistant = cached.some(
			(item) => item.type === 'assistant' && item.pending === true
		);
		const hasCompletedAssistant = cached.some(
			(item) =>
				item.type === 'assistant' &&
				item.pending !== true &&
				(item.text.length > 0 || hasReasoning(item))
		);
		return hasUser && pendingAssistant && !hasCompletedAssistant;
	}

	function abortAllStreams() {
		for (const [, handle] of state.streamHandles) {
			handle.abort.abort();
		}
		state.streamHandles.clear();
		state.localStreamingSessionIds.clear();
		refreshStreamingState();
		state.globalStreamRun += 1;
	}

	function getTranscriptPageState(targetSessionID: string): TranscriptPageState {
		return (
			state.sessionTranscriptPages.get(targetSessionID) ?? {
				hasMore: false,
				nextBefore: '',
				olderPageSeq: 0
			}
		);
	}

	function setTranscriptPageState(
		targetSessionID: string,
		next: TranscriptPageState,
		syncActive = true
	) {
		state.sessionTranscriptPages.set(targetSessionID, next);
		if (syncActive && state.sessionID === targetSessionID) {
			state.hasMoreHistory = next.hasMore;
		}
	}

	function applyTranscriptPageMeta(
		targetSessionID: string,
		transcript: { has_more?: boolean; next_before?: string },
		opts: { resetSeq?: boolean } = {}
	) {
		const prev = getTranscriptPageState(targetSessionID);
		setTranscriptPageState(targetSessionID, {
			hasMore: Boolean(transcript.has_more),
			nextBefore: transcript.next_before ?? '',
			olderPageSeq: opts.resetSeq ? 0 : prev.olderPageSeq
		});
	}

	return {
		cachedItemCount,
		getCachedItemCount,
		hasCachedConversationTurns,
		getCachedItems,
		setRunError,
		writeSessionItems,
		markStreaming,
		unmarkStreaming,
		hasLocalStream,
		setRemoteStreamingState,
		isStreamingFor,
		hasRunError,
		hasInFlightTurn,
		isAwaitingFirstAssistant,
		abortAllStreams,
		getTranscriptPageState,
		setTranscriptPageState,
		applyTranscriptPageMeta
	};
}

export type SessionCache = ReturnType<typeof createSessionCache>;
