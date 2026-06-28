import { browser } from '$app/environment';
import { goto } from '$app/navigation';
import {
	abortSession,
	getSessionMessages,
	isSessionNotFoundError,
	listChildSessions,
	streamMessage
} from '$lib/client/cometmind';
import type { ChatItem, ImageAttachment, Session, StreamEvent } from '$lib/types';
import type { ChatTurnPayload } from '$lib/actions/start-chat';
import { reduceChatState } from '$lib/reducers/chat';
import { anyReasoningPending, hasReasoning } from '$lib/conversation/reasoning';
import { sessionStore } from '$lib/stores/session.svelte';
import { chatDebug, summarizeChatItems, summarizeStreamEvent } from '../debug/chat';
import { playResponseCompleteSound } from '$lib/sound/response-complete';
import { publishWindowSync, subscribeWindowSync } from '$lib/window-sync';

import { itemsFromTranscript, localID, mergeSubagents } from '$lib/stores/chat-transcript';
import {
	BATCHABLE_STREAM_EVENTS,
	type SessionStream,
	type StreamCtx
} from '$lib/stores/chat-stream-types';

export type { ChatItem } from '$lib/types';

type AssistantItem = Extract<ChatItem, { type: 'assistant' }>;

function createChatStore() {
	let sessionID = $state<string | null>(null);
	let items = $state.raw<ChatItem[]>([]);
	let isLoading = $state(false);
	let error = $state('');
	let nextId = 0;
	let globalStreamRun = 0;
	let loadRun = 0;
	let loadPromise: Promise<void> | null = null;
	let loadPromiseSession: string | null = null;

	const sessionCache = new Map<string, ChatItem[]>();
	const sessionErrors = new Map<string, string>();
	const streamHandles = new Map<string, SessionStream>();
	const localStreamingSessionIds = new Set<string>();
	const remoteStreamingSessionIds = new Set<string>();
	let streamingSessionIds = $state.raw<Set<string>>(new Set());

	const BATCHABLE_EVENTS = BATCHABLE_STREAM_EVENTS;

	function isAbortError(err: unknown) {
		return err instanceof DOMException && err.name === 'AbortError';
	}

	function cachedItemCount(targetSessionID: string) {
		return sessionCache.get(targetSessionID)?.length ?? 0;
	}

	function getCachedItemCount(targetSessionID: string) {
		return cachedItemCount(targetSessionID);
	}

	function getCachedItems(targetSessionID: string) {
		return sessionCache.get(targetSessionID) ?? [];
	}

	function refreshStreamingState() {
		streamingSessionIds = new Set([...localStreamingSessionIds, ...remoteStreamingSessionIds]);
	}

	function writeSessionItems(
		targetSessionID: string,
		nextItems: ChatItem[],
		options: { broadcast?: boolean } = {}
	) {
		const { broadcast = true } = options;
		sessionCache.set(targetSessionID, nextItems);
		if (sessionID === targetSessionID) {
			items = nextItems;
		}
		if (broadcast) {
			publishWindowSync({ type: 'chat-items', sessionId: targetSessionID, items: nextItems });
		}
	}

	function discardMissingSession(targetSessionID: string) {
		const handle = streamHandles.get(targetSessionID);
		if (handle) handle.abort.abort();
		unmarkStreaming(targetSessionID);
		sessionCache.delete(targetSessionID);
		sessionErrors.delete(targetSessionID);
		sessionStore.discardSession(targetSessionID);
		if (sessionID === targetSessionID) {
			sessionID = null;
			items = [];
			error = '';
			isLoading = false;
		}
		if (browser) void goto('/');
	}

	function markStreaming(targetSessionID: string, handle: SessionStream) {
		streamHandles.set(targetSessionID, handle);
		localStreamingSessionIds.add(targetSessionID);
		refreshStreamingState();
		publishWindowSync({ type: 'chat-streaming', sessionId: targetSessionID, streaming: true });
	}

	function unmarkStreaming(targetSessionID: string) {
		streamHandles.delete(targetSessionID);
		if (localStreamingSessionIds.delete(targetSessionID)) {
			refreshStreamingState();
		}
		publishWindowSync({ type: 'chat-streaming', sessionId: targetSessionID, streaming: false });
	}

	function setRemoteStreamingState(targetSessionID: string, streaming: boolean) {
		if (streaming) {
			remoteStreamingSessionIds.add(targetSessionID);
		} else {
			remoteStreamingSessionIds.delete(targetSessionID);
		}
		refreshStreamingState();
	}

	function isStreamingFor(targetSessionID: string) {
		return streamingSessionIds.has(targetSessionID);
	}

	function hasInFlightTurn(targetSessionID: string) {
		if (isStreamingFor(targetSessionID)) return true;
		if (streamHandles.has(targetSessionID)) return true;
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
		for (const [, handle] of streamHandles) {
			handle.abort.abort();
		}
		streamHandles.clear();
		localStreamingSessionIds.clear();
		refreshStreamingState();
		globalStreamRun += 1;
	}

	function clear() {
		abortAllStreams();
		sessionCache.clear();
		sessionErrors.clear();
		sessionID = null;
		items = [];
		isLoading = false;
		error = '';
		loadRun += 1;
		loadPromise = null;
		loadPromiseSession = null;
	}

	function resetTranscript(targetSessionID: string) {
		loadRun += 1;
		loadPromise = null;
		loadPromiseSession = null;
		sessionErrors.delete(targetSessionID);
		writeSessionItems(targetSessionID, []);
		if (sessionID === targetSessionID) {
			error = '';
			isLoading = false;
		}
	}

	function detachActiveSession() {
		if (sessionID) {
			const handle = streamHandles.get(sessionID);
			if (handle) {
				flushBatchForSession(sessionID, handle.ctx, handle);
			}
			sessionCache.set(sessionID, getCachedItems(sessionID));
		}
		loadRun += 1;
		loadPromise = null;
		loadPromiseSession = null;
		sessionID = null;
		items = [];
		isLoading = false;
		error = '';
	}

	function reconcileStreamCtx(targetSessionID: string, ctx: StreamCtx) {
		const cached = getCachedItems(targetSessionID);
		if (ctx.assistant.current) {
			const synced = cached.find(
				(item): item is AssistantItem =>
					item.type === 'assistant' && item.id === ctx.assistant.current!.id
			);
			if (synced) {
				ctx.assistant.current = synced;
				return;
			}
			ctx.assistant.current = null;
		}
		const last = cached.at(-1);
		if (last?.type === 'assistant' && (last.pending === true || anyReasoningPending(last))) {
			ctx.assistant.current = last;
			return;
		}
		for (let i = cached.length - 1; i >= 0; i--) {
			const item = cached[i];
			if (item.type === 'assistant') {
				ctx.assistant.current = item;
				return;
			}
		}
	}

	function bindSession(nextSessionID: string) {
		if (sessionID === nextSessionID) return;

		if (sessionID) {
			const handle = streamHandles.get(sessionID);
			if (handle) {
				flushBatchForSession(sessionID, handle.ctx, handle);
			}
			sessionCache.set(sessionID, getCachedItems(sessionID));
		}

		loadRun += 1;
		loadPromise = null;
		loadPromiseSession = null;
		sessionID = nextSessionID;
		items = sessionCache.get(nextSessionID) ?? [];
		error = sessionErrors.get(nextSessionID) ?? '';
		isLoading = false;
	}

	async function loadTranscript(nextSessionID: string) {
		if (sessionID === nextSessionID && items.length > 0) return;
		if (hasInFlightTurn(nextSessionID) && cachedItemCount(nextSessionID) > 0) return;
		if (sessionID === nextSessionID && isLoading && loadPromise) return loadPromise;

		const run = ++loadRun;
		const switchingSession = sessionID !== nextSessionID;
		if (switchingSession) {
			if (sessionID) {
				const handle = streamHandles.get(sessionID);
				if (handle) {
					flushBatchForSession(sessionID, handle.ctx, handle);
				}
				sessionCache.set(sessionID, getCachedItems(sessionID));
			}
			sessionID = nextSessionID;
			items = sessionCache.get(nextSessionID) ?? [];
		} else {
			sessionID = nextSessionID;
		}
		isLoading = true;
		error = '';
		loadPromiseSession = nextSessionID;
		loadPromise = (async () => {
			try {
				const transcript = await getSessionMessages(nextSessionID);
				const children = await listChildSessions(nextSessionID).catch(() => ({
					sessions: [] as Session[]
				}));
				if (run !== loadRun && sessionID !== nextSessionID) return;
				if (hasInFlightTurn(nextSessionID) && cachedItemCount(nextSessionID) > 0) return;
				if (sessionID === nextSessionID && items.length > 0) return;
				const loaded = mergeSubagents(
					itemsFromTranscript(transcript.items),
					children.sessions
				);
				writeSessionItems(nextSessionID, loaded);
				sessionErrors.delete(nextSessionID);
				if (sessionID === nextSessionID) error = '';
				chatDebug('store:load-transcript', {
					sessionID: nextSessionID,
					rawItems: transcript.items,
					items: summarizeChatItems(getCachedItems(nextSessionID))
				});
			} catch (err) {
				if (isSessionNotFoundError(err)) {
					discardMissingSession(nextSessionID);
					return;
				}
				if (run !== loadRun && sessionID !== nextSessionID) return;
				if (hasInFlightTurn(nextSessionID) && cachedItemCount(nextSessionID) > 0) return;
				if (sessionID === nextSessionID && items.length > 0) return;
				const message = err instanceof Error ? err.message : 'Failed to load transcript';
				sessionErrors.set(nextSessionID, message);
				writeSessionItems(nextSessionID, [
					{ id: localID('error'), type: 'error', text: message }
				]);
				if (sessionID === nextSessionID) error = message;
			} finally {
				if (loadPromiseSession === nextSessionID) {
					if (sessionID === nextSessionID) {
						isLoading = false;
					}
					loadPromise = null;
					loadPromiseSession = null;
				}
			}
		})();
		return loadPromise;
	}

	function addUserToSession(
		targetSessionID: string,
		text: string,
		images?: ImageAttachment[],
		reveal = true
	) {
		const next = getCachedItems(targetSessionID).slice();
		next.push({ id: localID('user'), type: 'user', text, images, reveal });
		writeSessionItems(targetSessionID, next);
	}

	function stageUserForSession(
		targetSessionID: string,
		text: string,
		images?: ImageAttachment[]
	) {
		addUserToSession(targetSessionID, text, images, false);
	}

	function revealStagedUserForSession(targetSessionID: string) {
		const current = getCachedItems(targetSessionID);
		let revealIndex = -1;
		for (let i = current.length - 1; i >= 0; i--) {
			const item = current[i];
			if (item.type === 'user' && item.reveal === false) {
				revealIndex = i;
				break;
			}
		}
		if (revealIndex < 0) return;
		writeSessionItems(
			targetSessionID,
			current.map((item, i) =>
				i === revealIndex && item.type === 'user' ? { ...item, reveal: true } : item
			)
		);
	}

	function stageUser(text: string, images?: ImageAttachment[]) {
		if (!sessionID) return;
		stageUserForSession(sessionID, text, images);
	}

	function revealStagedUser() {
		if (!sessionID) return;
		revealStagedUserForSession(sessionID);
	}

	function applyEventToSession(targetSessionID: string, event: StreamEvent, ctx: StreamCtx) {
		if (isStreamingFor(targetSessionID)) {
			reconcileStreamCtx(targetSessionID, ctx);
		}
		const sessionItems = getCachedItems(targetSessionID);
		const sessionError =
			sessionErrors.get(targetSessionID) ?? (sessionID === targetSessionID ? error : '');
		const reduced = reduceChatState(
			{
				items: sessionItems,
				error: sessionError,
				assistant: ctx.assistant.current,
				reasoning: ctx.reasoning.current,
				nextId
			},
			event
		);
		nextId = reduced.nextId;
		ctx.assistant.current = reduced.assistant;
		ctx.reasoning.current = reduced.reasoning;
		if (reduced.error) {
			sessionErrors.set(targetSessionID, reduced.error);
		} else {
			sessionErrors.delete(targetSessionID);
		}
		if (sessionID === targetSessionID) {
			error = reduced.error;
		}
		writeSessionItems(targetSessionID, reduced.items);
	}

	function flushBatchForSession(targetSessionID: string, ctx: StreamCtx, handle: SessionStream) {
		if (handle.pendingBatchEvents.length === 0) return;
		const batch = handle.pendingBatchEvents;
		handle.pendingBatchEvents = [];
		for (const event of batch) {
			applyEventToSession(targetSessionID, event, ctx);
		}
	}

	function scheduleBatchForSession(
		targetSessionID: string,
		event: StreamEvent,
		ctx: StreamCtx,
		handle: SessionStream
	) {
		handle.pendingBatchEvents.push(event);
		if (handle.batchFrame) return;
		handle.batchFrame = requestAnimationFrame(() => {
			handle.batchFrame = 0;
			const current = streamHandles.get(targetSessionID);
			if (!current || current.run !== handle.run) return;
			flushBatchForSession(targetSessionID, ctx, handle);
		});
	}

	function applyStreamEventForSession(
		targetSessionID: string,
		event: StreamEvent,
		ctx: StreamCtx,
		handle: SessionStream
	) {
		if (BATCHABLE_EVENTS.has(event.type)) {
			scheduleBatchForSession(targetSessionID, event, ctx, handle);
			return;
		}
		if (handle.pendingBatchEvents.length > 0) {
			flushBatchForSession(targetSessionID, ctx, handle);
		}
		applyEventToSession(targetSessionID, event, ctx);
	}

	async function send(
		nextSessionID: string,
		payloadOrText: ChatTurnPayload | string,
		opts?: { skipUser?: boolean }
	) {
		const payload = typeof payloadOrText === 'string' ? { text: payloadOrText } : payloadOrText;
		const text = payload.text;
		const displayText = payload.displayText ?? text;
		const images = payload.images;
		if (isStreamingFor(nextSessionID)) {
			chatDebug('store:send-blocked', {
				sessionID: nextSessionID,
				reason: 'session-already-streaming',
				textLength: text.length
			});
			throw new Error('Session is already streaming');
		}

		const handle: SessionStream = {
			run: ++globalStreamRun,
			abort: new AbortController(),
			pendingBatchEvents: [],
			batchFrame: 0,
			ctx: {
				assistant: { current: null },
				reasoning: { current: null }
			}
		};

		if (sessionID === nextSessionID) {
			error = '';
			sessionErrors.delete(nextSessionID);
		}

		if (!opts?.skipUser) addUserToSession(nextSessionID, displayText, images);
		markStreaming(nextSessionID, handle);

		const ctx = handle.ctx;
		const preId = localID('assistant');
		const preAssistant: Extract<ChatItem, { type: 'assistant' }> = {
			id: preId,
			type: 'assistant',
			text: '',
			pending: true,
			pendingStartedAt: Date.now()
		};
		const preItems = getCachedItems(nextSessionID).slice();
		preItems.push(preAssistant);
		writeSessionItems(nextSessionID, preItems);
		ctx.assistant.current = preAssistant;
		let eventIndex = 0;
		let streamDone = false;
		let streamOutcome: 'success' | 'abort' | 'error' = 'success';
		try {
			for await (const event of streamMessage(
				nextSessionID,
				{
					text,
					display_text: payload.displayText,
					images: images?.map((image) => ({
						media_type: image.media_type,
						data: image.data
					})),
					file_paths: payload.filePaths
				},
				handle.abort.signal
			)) {
				const current = streamHandles.get(nextSessionID);
				if (!current || current.run !== handle.run) {
					handle.abort.abort();
					return;
				}
				eventIndex += 1;
				const before = summarizeChatItems(getCachedItems(nextSessionID));
				if (event.type === 'done') {
					if (!streamDone) {
						streamDone = true;
						flushBatchForSession(nextSessionID, ctx, handle);
						applyEventToSession(nextSessionID, event, ctx);
						unmarkStreaming(nextSessionID);
						if (streamOutcome === 'success' && !sessionErrors.get(nextSessionID)) {
							playResponseCompleteSound();
						}
					}
					chatDebug('store:stream-event', {
						sessionID: nextSessionID,
						run: handle.run,
						eventIndex,
						event: summarizeStreamEvent(event),
						before,
						after: summarizeChatItems(getCachedItems(nextSessionID)),
						assistantID: ctx.assistant.current?.id ?? null,
						reasoning: ctx.reasoning.current
					});
					continue;
				}
				applyStreamEventForSession(nextSessionID, event, ctx, handle);
				chatDebug('store:stream-event', {
					sessionID: nextSessionID,
					run: handle.run,
					eventIndex,
					event: summarizeStreamEvent(event),
					before,
					after: summarizeChatItems(getCachedItems(nextSessionID)),
					assistantID: ctx.assistant.current?.id ?? null,
					reasoning: ctx.reasoning.current
				});
			}
		} catch (err) {
			const current = streamHandles.get(nextSessionID);
			if (!current || current.run !== handle.run) {
				handle.abort.abort();
				return;
			}
			if (isAbortError(err)) {
				streamOutcome = 'abort';
				chatDebug('store:send-aborted', { sessionID: nextSessionID, run: handle.run });
				return;
			}
			if (isSessionNotFoundError(err)) {
				streamOutcome = 'error';
				discardMissingSession(nextSessionID);
				return;
			}
			streamOutcome = 'error';
			applyStreamEventForSession(
				nextSessionID,
				{
					type: 'error',
					message: err instanceof Error ? err.message : 'Failed to send message'
				},
				ctx,
				handle
			);
		} finally {
			if (streamHandles.get(nextSessionID) === handle) {
				flushBatchForSession(nextSessionID, ctx, handle);
				const beforeDone = summarizeChatItems(getCachedItems(nextSessionID));
				if (!streamDone) {
					applyEventToSession(nextSessionID, { type: 'done' }, ctx);
					unmarkStreaming(nextSessionID);
					if (streamOutcome === 'success' && !sessionErrors.get(nextSessionID)) {
						playResponseCompleteSound();
					}
				}
				chatDebug('store:send-finish', {
					sessionID: nextSessionID,
					run: handle.run,
					beforeDone,
					afterDone: summarizeChatItems(getCachedItems(nextSessionID)),
					assistantID: ctx.assistant.current?.id ?? null,
					reasoning: ctx.reasoning.current,
					error: sessionErrors.get(nextSessionID) ?? ''
				});
			}
		}
	}

	async function cancel(targetSessionID?: string) {
		const id = targetSessionID ?? sessionID;
		if (!id) return;
		const handle = streamHandles.get(id);
		if (!handle && !isStreamingFor(id)) return;

		chatDebug('store:cancel-start', { sessionID: id });
		handle?.abort.abort();
		try {
			await abortSession(id);
		} catch (err) {
			chatDebug('store:cancel-abort-failed', {
				sessionID: id,
				error: err instanceof Error ? err.message : String(err)
			});
		}
	}

	function patchSubagentCard(
		targetSessionID: string,
		childSessionId: string,
		patch: Partial<Extract<ChatItem, { type: 'subagent' }>>
	) {
		const next = getCachedItems(targetSessionID).map((item) =>
			item.type === 'subagent' && item.childSessionId === childSessionId
				? { ...item, ...patch }
				: item
		);
		writeSessionItems(targetSessionID, next);
	}

	async function cancelSubagent(childSessionId: string) {
		if (!sessionID) return;
		try {
			await abortSession(childSessionId);
			patchSubagentCard(sessionID, childSessionId, {
				status: 'cancelled',
				pending: false
			});
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to cancel subagent';
		}
	}

	function appendLocalUserMessage(targetSessionID: string, text: string) {
		addUserToSession(targetSessionID, text);
	}

	if (browser) {
		subscribeWindowSync((message) => {
			if (message.type === 'chat-items') {
				writeSessionItems(message.sessionId, message.items, { broadcast: false });
				return;
			}
			if (message.type === 'chat-streaming') {
				setRemoteStreamingState(message.sessionId, message.streaming);
			}
		});
	}

	return {
		get sessionID() {
			return sessionID;
		},
		get items() {
			return items;
		},
		get isLoading() {
			return isLoading;
		},
		get isStreaming() {
			return streamingSessionIds.size > 0;
		},
		get error() {
			return error;
		},
		isStreamingFor,
		hasInFlightTurn,
		isAwaitingFirstAssistant,
		getCachedItemCount,
		clear,
		resetTranscript,
		detachActiveSession,
		bindSession,
		loadTranscript,
		stageUserForSession,
		revealStagedUserForSession,
		stageUser,
		revealStagedUser,
		send,
		cancel,
		cancelSubagent,
		appendLocalUserMessage
	};
}

export const chatStore = createChatStore();
