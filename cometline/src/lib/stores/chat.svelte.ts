import { browser } from '$app/environment';
import { goto } from '$app/navigation';
import {
	abortSession,
	getSessionMessages,
	isSessionNotFoundError,
	listChildSessions,
	streamMessage
} from '$lib/client/cometmind';
import type { ChatItem, ImageAttachment, Session, StreamEvent, TranscriptItem } from '$lib/types';
import type { ChatTurnPayload } from '$lib/actions/start-chat';
import { reduceChatState } from '$lib/reducers/chat';
import {
	anyReasoningPending,
	getReasoningSegments,
	hasReasoning
} from '$lib/conversation/reasoning';
import { stripInlinedFileBlocks } from '$lib/messages/strip-inlined-files';
import { sessionStore } from '$lib/stores/session.svelte';
import { chatDebug, summarizeChatItems, summarizeStreamEvent } from '../debug/chat';
import { playResponseCompleteSound } from '$lib/sound/response-complete';

export type { ChatItem } from '$lib/types';

let nextLocalID = 0;

function localID(prefix: string) {
	nextLocalID += 1;
	return `${prefix}-${Date.now()}-${nextLocalID}`;
}

function mapDelegationStatus(
	status: string | undefined
): Extract<ChatItem, { type: 'subagent' }>['status'] {
	switch (status) {
		case 'completed':
			return 'completed';
		case 'cancelled':
			return 'cancelled';
		case 'failed':
			return 'failed';
		case 'running':
			return 'running';
		default:
			return 'pending';
	}
}

function subagentFromChild(
	child: Session,
	agentName = 'opencode'
): Extract<ChatItem, { type: 'subagent' }> {
	return {
		id: `subagent-${child.id}`,
		type: 'subagent',
		childSessionId: child.id,
		purpose: child.purpose ?? child.title ?? 'Delegated coding task',
		agentName,
		status: mapDelegationStatus(child.delegation_status),
		progress: [],
		summary: child.output_summary,
		pending: child.delegation_status === 'running'
	};
}

function mergeSubagents(items: ChatItem[], children: Session[]): ChatItem[] {
	if (children.length === 0) return items;
	const used = new Set<string>();
	const out: ChatItem[] = [];

	for (const item of items) {
		if (item.type === 'tool' && item.toolName === 'delegate_coding_task') {
			const match = children.find(
				(child) => !used.has(child.id) && item.output?.includes(child.id)
			);
			if (match) {
				used.add(match.id);
				out.push(subagentFromChild(match));
				continue;
			}
		}
		out.push(item);
	}

	for (const child of children) {
		if (!used.has(child.id)) {
			out.push(subagentFromChild(child));
		}
	}
	return out;
}

function itemsFromTranscript(transcriptItems: TranscriptItem[]): ChatItem[] {
	const out: ChatItem[] = [];
	let currentAssistant: Extract<ChatItem, { type: 'assistant' }> | null = null;

	function pushAssistant(index: number, text = '') {
		const assistant: Extract<ChatItem, { type: 'assistant' }> = {
			id: `history-${index}`,
			type: 'assistant',
			text
		};
		out.push(assistant);
		currentAssistant = assistant;
		return assistant;
	}

	function ensureAssistant(index: number) {
		return currentAssistant ?? pushAssistant(index, '');
	}

	function appendAssistantText(index: number, text: string) {
		if (!text) return;
		const assistant = ensureAssistant(index);
		assistant.text += text;
	}

	function appendReasoning(index: number, text: string) {
		if (!text) return;
		const assistant = ensureAssistant(index);
		const segments = [...getReasoningSegments(assistant.reasoning)];
		segments.push({ text, pending: false });
		assistant.reasoning = { segments };
	}

	for (let i = 0; i < transcriptItems.length; i++) {
		const item = transcriptItems[i];
		if (item.type === 'user' || item.type === 'system') {
			currentAssistant = null;
			out.push(itemFromTranscript(item, i));
			continue;
		}
		if (item.type === 'assistant') {
			appendAssistantText(i, item.text ?? '');
			continue;
		}
		if (item.type === 'reasoning') {
			appendReasoning(i, item.text ?? '');
			continue;
		}
		if (item.type === 'tool') {
			const toolItem = itemFromTranscript(item, i);
			if (toolItem.type === 'tool') {
				const host = out.findLast(
					(row): row is Extract<ChatItem, { type: 'assistant' }> =>
						row.type === 'assistant'
				);
				toolItem.afterSegment = host
					? Math.max(0, getReasoningSegments(host.reasoning).length - 1)
					: 0;
			}
			out.push(toolItem);
			continue;
		}
		out.push(itemFromTranscript(item, i));
	}
	return out;
}

function itemFromTranscript(item: TranscriptItem, index: number): ChatItem {
	if (item.type === 'user')
		return {
			id: `history-${index}`,
			type: 'user',
			text: stripInlinedFileBlocks(item.text ?? ''),
			images: item.images
		};
	if (item.type === 'assistant')
		return { id: `history-${index}`, type: 'assistant', text: item.text ?? '' };
	if (item.type === 'system')
		return { id: `history-${index}`, type: 'status', text: item.text ?? '' };
	if (item.type === 'reasoning')
		return {
			id: `history-${index}`,
			type: 'assistant',
			text: '',
			reasoning: { segments: [{ text: item.text ?? '', pending: false }] }
		};
	if (item.type === 'memory')
		return {
			id: `history-${index}`,
			type: 'memory',
			memories: (item.memories ?? []).map((mem) => ({
				id: mem.id,
				content: mem.content,
				kind: mem.kind,
				similarity: mem.similarity,
				effective_weight: mem.effective_weight
			}))
		};
	return {
		id: `history-${index}`,
		type: 'tool',
		toolName: item.tool_name ?? '',
		input: item.tool_input,
		output: item.tool_error ? undefined : item.tool_output,
		error: item.tool_error ? item.tool_output : undefined,
		pending: false
	};
}

type StreamCtx = {
	assistant: { current: Extract<ChatItem, { type: 'assistant' }> | null };
	reasoning: { current: { text: string; pending: boolean } | null };
};

interface SessionStream {
	run: number;
	abort: AbortController;
	pendingBatchEvents: StreamEvent[];
	batchFrame: number;
	ctx: StreamCtx;
}

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
	let streamingSessionIds = $state.raw<Set<string>>(new Set());

	const BATCHABLE_EVENTS = new Set(['text_delta', 'reasoning_delta', 'reasoning_start']);

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

	function writeSessionItems(targetSessionID: string, nextItems: ChatItem[]) {
		sessionCache.set(targetSessionID, nextItems);
		if (sessionID === targetSessionID) {
			items = nextItems;
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
		streamingSessionIds.add(targetSessionID);
		streamingSessionIds = new Set(streamingSessionIds);
	}

	function unmarkStreaming(targetSessionID: string) {
		streamHandles.delete(targetSessionID);
		if (streamingSessionIds.delete(targetSessionID)) {
			streamingSessionIds = new Set(streamingSessionIds);
		}
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
		streamingSessionIds = new Set();
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

	function addUser(text: string, images?: ImageAttachment[], reveal = true) {
		if (!sessionID) return;
		addUserToSession(sessionID, text, images, reveal);
	}

	function stageUser(text: string, images?: ImageAttachment[]) {
		addUser(text, images, false);
	}

	function revealStagedUser() {
		if (!sessionID) return;
		const current = getCachedItems(sessionID);
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
			sessionID,
			current.map((item, i) =>
				i === revealIndex && item.type === 'user' ? { ...item, reveal: true } : item
			)
		);
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

		if (!opts?.skipUser) addUserToSession(nextSessionID, text, images);
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
		let streamOutcome: 'success' | 'abort' | 'error' = 'success';
		try {
			for await (const event of streamMessage(
				nextSessionID,
				{
					text,
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
				if (event.type === 'done') break;
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
				applyEventToSession(nextSessionID, { type: 'done' }, ctx);
				unmarkStreaming(nextSessionID);
				chatDebug('store:send-finish', {
					sessionID: nextSessionID,
					run: handle.run,
					beforeDone,
					afterDone: summarizeChatItems(getCachedItems(nextSessionID)),
					assistantID: ctx.assistant.current?.id ?? null,
					reasoning: ctx.reasoning.current,
					error: sessionErrors.get(nextSessionID) ?? ''
				});
				if (streamOutcome === 'success' && !sessionErrors.get(nextSessionID)) {
					playResponseCompleteSound();
				}
			}
		}
	}

	async function cancel(targetSessionID?: string) {
		const id = targetSessionID ?? sessionID;
		if (!id) return;
		const handle = streamHandles.get(id);
		if (!handle) return;

		chatDebug('store:cancel-start', { sessionID: id });
		handle.abort.abort();
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
		detachActiveSession,
		bindSession,
		loadTranscript,
		stageUser,
		revealStagedUser,
		send,
		cancel,
		cancelSubagent
	};
}

export const chatStore = createChatStore();
