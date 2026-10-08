import { browser } from '$app/env';
import type { ChatItem } from '#lib/types.js';
import { subscribeWindowSync } from '#lib/window-sync.js';
import { unreadSessionOutputStore } from '#lib/stores/unread-session-output.svelte.js';
import { createChatState } from '#lib/stores/chat/chat-state.svelte.js';
import { createSessionCache } from '#lib/stores/chat/session-cache.svelte.js';
import { createSessionBinding } from '#lib/stores/chat/session-binding.svelte.js';
import { createTranscriptLoader } from '#lib/stores/chat/transcript-loader.svelte.js';
import { createTurnEvents } from '#lib/stores/chat/turn-events.svelte.js';
import { createStreamRunner } from '#lib/stores/chat/stream-runner.svelte.js';

export type { ChatItem } from '#lib/types.js';

export function revealRemoteUserItems(items: ChatItem[]): ChatItem[] {
	return items.map((item) =>
		item.type === 'user' && item.reveal === false ? { ...item, reveal: true } : item
	);
}

function createChatStore() {
	const state = createChatState();
	const cache = createSessionCache(state);
	const binding = createSessionBinding(state, cache);
	const loader = createTranscriptLoader(state, cache, binding);
	const turns = createTurnEvents(state, cache);
	const runner = createStreamRunner(state, cache, binding, loader, turns);

	if (browser) {
		subscribeWindowSync((message) => {
			if (message.type === 'chat-items') {
				if (state.streamHandles.has(message.sessionId)) return;
				// Flight visibility belongs to the sending renderer. Other windows have
				// no matching particle, so render their copy immediately.
				cache.writeSessionItems(message.sessionId, revealRemoteUserItems(message.items), {
					broadcast: false
				});
				return;
			}
			if (message.type === 'chat-streaming') {
				cache.setRemoteStreamingState(message.sessionId, message.streaming);
				return;
			}
			if (message.type === 'chat-run-error') {
				cache.setRunError(message.sessionId, message.failed, false);
				return;
			}
			if (
				message.type === 'session-output-unread' &&
				message.unread &&
				state.sessionID === message.sessionId
			) {
				// Another window generated output while this window is already showing
				// the session, so keep the shared marker cleared.
				unreadSessionOutputStore.markRead(message.sessionId);
			}
		});
	}

	return {
		get sessionID() {
			return state.sessionID;
		},
		get items() {
			return state.items;
		},
		get isLoading() {
			return state.isLoading;
		},
		get isLoadingOlder() {
			return state.isLoadingOlder;
		},
		get hasMoreHistory() {
			return state.hasMoreHistory;
		},
		get isStreaming() {
			return state.streamingSessionIds.size > 0;
		},
		get error() {
			return state.error;
		},
		get contextBudget() {
			return state.contextBudget;
		},
		isStreamingFor: cache.isStreamingFor,
		hasLocalStream: cache.hasLocalStream,
		consumeLocalRunSettled: cache.consumeLocalRunSettled,
		hasRunError: cache.hasRunError,
		hasInFlightTurn: cache.hasInFlightTurn,
		isAwaitingFirstAssistant: cache.isAwaitingFirstAssistant,
		getCachedItemCount: cache.getCachedItemCount,
		hasCachedConversationTurns: cache.hasCachedConversationTurns,
		clear: binding.clear,
		resetTranscript: binding.resetTranscript,
		detachActiveSession: binding.detachActiveSession,
		bindSession: binding.bindSession,
		loadTranscript: loader.loadTranscript,
		loadOlderTranscript: loader.loadOlderTranscript,
		refreshTranscript: loader.refreshTranscript,
		resumeRun: runner.resumeRun,
		stageUserForSession: turns.stageUserForSession,
		revealStagedUserForSession: turns.revealStagedUserForSession,
		send: runner.send,
		cancel: runner.cancel,
		cancelSubagent: runner.cancelSubagent
	};
}

export const chatStore = createChatStore();
